package contracts

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/thanhmaitran/compatlab/internal/compatibility"
	"github.com/thanhmaitran/compatlab/internal/domain"
	"github.com/thanhmaitran/compatlab/internal/specification"
)

func loadAPI(t *testing.T, rel string) *domain.NormalizedAPI {
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

func loadContract(t *testing.T, name string) *domain.ConsumerContract {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("..", "..", "tests", "fixtures", "contracts", name))
	if err != nil {
		t.Fatal(err)
	}
	c, err := Parse(data)
	if err != nil {
		t.Fatal(err)
	}
	return c
}

func TestParseRejectsInvalidContracts(t *testing.T) {
	bad := []string{
		`not json`,
		`{"contract_version":"1.0","consumer":"a","provider":"b","interactions":[],"extra":1}`,
		`{"contract_version":"1.0","consumer":"a","provider":"b","interactions":[]}`,
		`{"consumer":"a","provider":"b","interactions":[{"request":{"method":"GET","path":"/x"},"response":{"status":200}}]}`,
	}
	for _, doc := range bad {
		if _, err := Parse([]byte(doc)); !errors.Is(err, domain.ErrInvalid) {
			t.Errorf("%q: expected ErrInvalid, got %v", doc, err)
		}
	}
}

func TestMatchTemplate(t *testing.T) {
	if vars, ok := matchTemplate("/users/{id}", "/users/u-1"); !ok || vars != 1 {
		t.Errorf("template must match with one variable, got %d %v", vars, ok)
	}
	if _, ok := matchTemplate("/users/{id}", "/users"); ok {
		t.Error("different segment counts must not match")
	}
	if _, ok := matchTemplate("/users/search", "/users/other"); ok {
		t.Error("static segments must match exactly")
	}
	if _, ok := matchTemplate("/users/{id}", "/users/u-1?expand=1"); !ok {
		t.Error("query strings must be ignored")
	}
}

func TestFindEndpointPrefersStaticPaths(t *testing.T) {
	api := loadAPI(t, "compatible/users-v2.yaml")
	ep := FindEndpoint(api, "GET", "/users/search")
	if ep == nil || ep.Path != "/users/search" {
		t.Errorf("expected /users/search, got %+v", ep)
	}
	ep = FindEndpoint(api, "get", "/users/u-1")
	if ep == nil || ep.Path != "/users/{userId}" {
		t.Errorf("expected /users/{userId}, got %+v", ep)
	}
}

func TestVerifyAgainstBaseVersionPasses(t *testing.T) {
	base := loadAPI(t, "compatible/users-v1.yaml")
	for _, name := range []string{"mobile-app.json", "analytics.json"} {
		res, err := NewVerifier().Verify(loadContract(t, name), base)
		if err != nil {
			t.Fatal(err)
		}
		if !res.Passed() {
			t.Errorf("%s should pass against v1: %+v", name, res.Interactions)
		}
	}
}

func TestVerifyDetectsOnlyTheContractsThatBreak(t *testing.T) {
	broken := loadAPI(t, "breaking/users-v2-breaking.yaml")
	mobile, _ := NewVerifier().Verify(loadContract(t, "mobile-app.json"), broken)
	if mobile.Passed() {
		t.Error("mobile-app depends on id as string and on email; it must fail")
	}
	analytics, _ := NewVerifier().Verify(loadContract(t, "analytics.json"), broken)
	if !analytics.Passed() {
		t.Errorf("analytics only reads name; it must still pass: %+v", analytics.Interactions)
	}
}

func TestVerifyFailureReasons(t *testing.T) {
	api := loadAPI(t, "compatible/users-v1.yaml")
	mk := func(method, path string, status int, body any) *domain.ConsumerContract {
		return &domain.ConsumerContract{ContractVersion: "1.0", Consumer: "c", Provider: "p",
			Interactions: []domain.ContractInteraction{{
				Description: "x",
				Request:     domain.ContractRequest{Method: method, Path: path},
				Response:    domain.ContractResponse{Status: status, Body: body},
			}}}
	}
	tests := []struct {
		name string
		c    *domain.ConsumerContract
		pass bool
	}{
		{"unknown endpoint", mk("GET", "/nope", 200, nil), false},
		{"unknown method", mk("PATCH", "/users/1", 200, nil), false},
		{"undocumented status", mk("GET", "/users/1", 418, nil), false},
		{"documented status without body", mk("GET", "/users/1", 404, nil), true},
		{"unknown field", mk("GET", "/users/1", 200, map[string]any{"ghost": "x"}), false},
		{"wrong type", mk("GET", "/users/1", 200, map[string]any{"id": 5.0}), false},
		{"correct body", mk("GET", "/users/1", 200, map[string]any{"id": "u", "email": "e"}), true},
	}
	for _, tc := range tests {
		res, err := NewVerifier().Verify(tc.c, api)
		if err != nil {
			t.Fatal(err)
		}
		if res.Passed() != tc.pass {
			t.Errorf("%s: passed=%v, want %v (%+v)", tc.name, res.Passed(), tc.pass, res.Interactions)
		}
	}
	if _, err := NewVerifier().Verify(nil, api); err == nil {
		t.Error("nil contract must be rejected")
	}
}

func TestImpactAttributesChangesToTheRightConsumers(t *testing.T) {
	analysis, err := compatibility.NewEngine().Compare(
		loadAPI(t, "compatible/users-v1.yaml"), loadAPI(t, "breaking/users-v2-breaking.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	impacts := Impact(analysis, []*domain.ConsumerContract{
		loadContract(t, "mobile-app.json"), loadContract(t, "analytics.json")})
	rulesByConsumer := map[string]map[string]string{}
	for _, im := range impacts {
		if rulesByConsumer[im.Consumer] == nil {
			rulesByConsumer[im.Consumer] = map[string]string{}
		}
		rulesByConsumer[im.Consumer][im.RuleID] = im.Level
	}
	mobile, analytics := rulesByConsumer["mobile-app"], rulesByConsumer["analytics"]
	for _, id := range []string{"RES-001", "RES-004", "SEC-001"} {
		if mobile[id] != domain.ImpactAffected {
			t.Errorf("mobile-app should be AFFECTED by %s, got %q", id, mobile[id])
		}
	}
	if analytics["SEC-001"] != domain.ImpactAffected {
		t.Error("analytics calls the same endpoint, so authentication affects it too")
	}
	if _, hit := analytics["RES-001"]; hit {
		t.Error("analytics does not read email, so RES-001 must not be attributed")
	}
	if _, hit := analytics["RES-004"]; hit {
		t.Error("analytics does not read id, so RES-004 must not be attributed")
	}
}

func TestSplitResponseLocation(t *testing.T) {
	tests := []struct {
		loc, code, field string
		body             bool
	}{
		{"response.200.body.email", "200", "email", true},
		{"response.200.body", "200", "", true},
		{"response.200.body[].name", "200", "name", true},
		{"response.404", "404", "", false},
		{"endpoint", "", "", false},
	}
	for _, tc := range tests {
		code, field, body := splitResponseLocation(tc.loc)
		if code != tc.code || field != tc.field || body != tc.body {
			t.Errorf("%q: got (%q,%q,%v), want (%q,%q,%v)", tc.loc, code, field, body, tc.code, tc.field, tc.body)
		}
	}
}
