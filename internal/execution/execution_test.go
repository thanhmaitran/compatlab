package execution

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/thanhmaitran/compatlab/internal/domain"
	"github.com/thanhmaitran/compatlab/internal/generation"
	"github.com/thanhmaitran/compatlab/internal/mocking"
	"github.com/thanhmaitran/compatlab/internal/security"
	"github.com/thanhmaitran/compatlab/internal/specification"
)

func load(t *testing.T, rel string) *domain.NormalizedAPI {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("..", "..", "testdata", rel))
	if err != nil {
		t.Fatal(err)
	}
	api, err := specification.NewParser().Parse(data)
	if err != nil {
		t.Fatal(err)
	}
	return api
}

func hostOf(t *testing.T, raw string) string {
	t.Helper()
	u, err := url.Parse(raw)
	if err != nil {
		t.Fatal(err)
	}
	return u.Host
}

// The generated suite, the mock server and the runner must agree with each
// other: running a suite generated from a spec against a mock of the same
// spec must pass completely.
func TestGeneratedSuitesPassAgainstTheirOwnMock(t *testing.T) {
	files := []string{"valid/simple-api.yaml", "valid/payment-api.yaml", "compatible/users-v1.yaml", "breaking/users-v2-breaking.yaml"}
	for _, file := range files {
		api := load(t, file)
		suite, err := generation.Generator{}.Generate(api, domain.GenerationConfig{})
		if err != nil {
			t.Fatal(err)
		}
		run, err := RunOnMock(context.Background(), api, suite)
		if err != nil {
			t.Fatalf("%s: %v", file, err)
		}
		if len(run.Results) != len(suite.Cases) || len(run.Results) == 0 {
			t.Fatalf("%s: %d results for %d cases", file, len(run.Results), len(suite.Cases))
		}
		names := map[string]string{}
		for _, c := range suite.Cases {
			names[c.ID] = c.Name
		}
		for _, res := range run.Results {
			if res.Status != domain.TestStatusPassed {
				t.Errorf("%s: %s -> %s: %s", file, names[res.CaseID], res.Status, res.Message)
			}
		}
		if run.PassRate() != 100 {
			t.Errorf("%s: pass rate %.1f", file, run.PassRate())
		}
	}
}

func TestTargetOutsideAllowlistIsRejected(t *testing.T) {
	api := load(t, "valid/simple-api.yaml")
	suite, _ := generation.Generator{}.Generate(api, domain.GenerationConfig{})
	_, err := Runner{}.Run(context.Background(), api, suite, "http://127.0.0.1:9")
	if !errors.Is(err, security.ErrTargetNotAllowed) {
		t.Errorf("expected ErrTargetNotAllowed, got %v", err)
	}
}

func TestRequestLimitIsEnforced(t *testing.T) {
	api := load(t, "compatible/users-v1.yaml")
	suite, _ := generation.Generator{}.Generate(api, domain.GenerationConfig{})
	runner := Runner{Config: Config{MaxRequests: 2, Policy: security.TargetPolicy{AllowedHosts: []string{"127.0.0.1"}}}}
	if _, err := runner.Run(context.Background(), api, suite, "http://127.0.0.1:9"); !errors.Is(err, domain.ErrInvalid) {
		t.Errorf("expected ErrInvalid for an oversized suite, got %v", err)
	}
}

func TestDifferentialExecution(t *testing.T) {
	api := load(t, "compatible/users-v1.yaml")
	suite, _ := generation.Generator{}.Generate(api, domain.GenerationConfig{})

	start := func(file string) (string, func()) {
		base, stop, err := mocking.StartLocal(mocking.NewHandler(load(t, file), nil))
		if err != nil {
			t.Fatal(err)
		}
		return base, stop
	}
	a, stopA := start("compatible/users-v1.yaml")
	defer stopA()
	same, stopSame := start("compatible/users-v1.yaml")
	defer stopSame()
	changed, stopChanged := start("breaking/users-v2-breaking.yaml")
	defer stopChanged()

	runner := Runner{Config: Config{Policy: security.TargetPolicy{AllowedHosts: []string{hostOf(t, a), hostOf(t, same), hostOf(t, changed)}}}}
	diffs, err := runner.RunDifferential(context.Background(), api, suite, a, same)
	if err != nil {
		t.Fatal(err)
	}
	for _, d := range diffs {
		if !d.Same {
			t.Errorf("identical providers must not differ: %+v", d)
		}
	}
	diffs, err = runner.RunDifferential(context.Background(), api, suite, a, changed)
	if err != nil {
		t.Fatal(err)
	}
	different := 0
	for _, d := range diffs {
		if !d.Same {
			different++
		}
	}
	if different == 0 {
		t.Error("the breaking provider must differ from the base provider on at least one case")
	}
}

func TestResponseSizeLimit(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write(make([]byte, 4096))
	}))
	defer srv.Close()
	cfg := Config{MaxResponseBytes: 100, Policy: security.TargetPolicy{AllowedHosts: []string{hostOf(t, srv.URL)}}}
	_, err := Do(context.Background(), NewClient(cfg), cfg, "GET", srv.URL, nil, nil)
	if !errors.Is(err, ErrLimit) {
		t.Errorf("expected ErrLimit, got %v", err)
	}
}

func TestRequestTimeout(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(500 * time.Millisecond)
	}))
	defer srv.Close()
	cfg := Config{Timeout: 50 * time.Millisecond, Policy: security.TargetPolicy{AllowedHosts: []string{hostOf(t, srv.URL)}}}
	start := time.Now()
	if _, err := Do(context.Background(), NewClient(cfg), cfg, "GET", srv.URL, nil, nil); err == nil {
		t.Error("expected a timeout error")
	}
	if time.Since(start) > 3*time.Second {
		t.Error("the request must not hang")
	}
}

func TestRedirectsToBlockedTargetsAreRefused(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "http://169.254.169.254/latest/meta-data/", http.StatusFound)
	}))
	defer srv.Close()
	cfg := Config{Policy: security.TargetPolicy{AllowedHosts: []string{hostOf(t, srv.URL)}}}
	if _, err := Do(context.Background(), NewClient(cfg), cfg, "GET", srv.URL, nil, nil); err == nil {
		t.Error("a redirect to a link-local address must be refused")
	}
}

func TestBuildRequestAppliesOverrides(t *testing.T) {
	ep := &domain.APIEndpoint{
		Path: "/users/{id}", Method: "POST",
		Parameters: []domain.Parameter{
			{Name: "id", Location: domain.LocationPath, Required: true, Schema: &domain.SchemaDefinition{Type: domain.KindString}},
			{Name: "page", Location: domain.LocationQuery, Required: true, Schema: &domain.SchemaDefinition{Type: domain.KindInteger}},
		},
		RequestBody: &domain.RequestBody{Schema: &domain.SchemaDefinition{
			Type: domain.KindObject, Required: []string{"name"},
			Properties: map[string]*domain.SchemaDefinition{"name": {Type: domain.KindString}},
		}},
		Security: []domain.SecurityRequirement{{Name: "bearer"}},
	}
	base := buildRequest(ep, domain.TestCase{})
	if base.query.Get("page") != "0" || base.headers["Authorization"] == "" || string(base.body) != `{"name":"string"}` {
		t.Errorf("unexpected baseline request: %+v body=%s", base, base.body)
	}
	omit := buildRequest(ep, domain.TestCase{Target: "query.page"})
	if _, has := omit.query["page"]; has {
		t.Error("a nil value must omit the parameter")
	}
	noAuth := buildRequest(ep, domain.TestCase{Category: domain.CategoryAuthentication, Variant: "missing"})
	if _, has := noAuth.headers["Authorization"]; has {
		t.Error("the authentication case must omit credentials")
	}
	override := buildRequest(ep, domain.TestCase{Target: "body.name", Value: 12345})
	if string(override.body) != `{"name":12345}` {
		t.Errorf("body override wrong: %s", override.body)
	}
	removed := buildRequest(ep, domain.TestCase{Target: "body.name"})
	if string(removed.body) != `{}` {
		t.Errorf("body omission wrong: %s", removed.body)
	}
}
