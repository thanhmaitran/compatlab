package integration_test

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/thanhmaitran/compatlab/internal/auth"
	"github.com/thanhmaitran/compatlab/internal/compatibility"
	"github.com/thanhmaitran/compatlab/internal/domain"
	"github.com/thanhmaitran/compatlab/internal/mocking"
	"github.com/thanhmaitran/compatlab/internal/persistence"
	"github.com/thanhmaitran/compatlab/internal/reporting"
	"github.com/thanhmaitran/compatlab/internal/service"
	"github.com/thanhmaitran/compatlab/internal/specification"
)

const root = "../../"

func read(t *testing.T, rel string) []byte {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(root, rel))
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func parse(t *testing.T, rel string) *domain.NormalizedAPI {
	t.Helper()
	api, err := specification.NewParser().Parse(read(t, rel))
	if err != nil {
		t.Fatalf("%s: %v", rel, err)
	}
	return api
}

// OpenAPI -> parser -> normalizer -> diff -> classifier -> report.
func TestPipelineFromSpecificationToReports(t *testing.T) {
	pairs := [][2]string{
		{"testdata/compatible/users-v1.yaml", "testdata/compatible/users-v2.yaml"},
		{"testdata/compatible/users-v1.yaml", "testdata/breaking/users-v2-breaking.yaml"},
		{"testdata/valid/payment-api.yaml", "testdata/valid/payment-api.yaml"},
	}
	for _, p := range pairs {
		analysis, err := compatibility.NewEngine().Compare(parse(t, p[0]), parse(t, p[1]))
		if err != nil {
			t.Fatalf("%v: %v", p, err)
		}
		report := &domain.CompatibilityReport{Analysis: analysis}
		text := reporting.Text(report)
		js, err := reporting.JSON(report)
		if err != nil || !json.Valid(js) {
			t.Fatalf("%v: invalid JSON report: %v", p, err)
		}
		html, err := reporting.HTML(report)
		if err != nil || !strings.Contains(string(html), "<table>") {
			t.Fatalf("%v: invalid HTML report: %v", p, err)
		}
		if !strings.Contains(text, "COMPATLAB RELEASE CHECK") {
			t.Errorf("%v: text report incomplete", p)
		}
	}
}

// The backend must return exactly what the engine computes: persisting and
// reloading an analysis may not lose or change anything.
func TestBackendMatchesTheEngine(t *testing.T) {
	ctx := context.Background()
	tokens, err := auth.NewTokenIssuer("0123456789abcdef0123456789abcdef", time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	svc := service.New(persistence.NewMemory(), tokens, nil)
	token, err := svc.Register(ctx, "Acme", "admin@acme.io", "correct-horse-1")
	if err != nil {
		t.Fatal(err)
	}
	claims, err := tokens.Verify(token)
	if err != nil {
		t.Fatal(err)
	}
	actor := service.Actor{UserID: claims.Sub, OrgID: claims.Org, Role: claims.Role}
	proj, err := svc.CreateProject(ctx, actor, "API", "")
	if err != nil {
		t.Fatal(err)
	}
	up := func(label, rel string) string {
		r, err := svc.UploadSpecification(ctx, actor, proj.ID, label, filepath.Base(rel), read(t, rel))
		if err != nil {
			t.Fatal(err)
		}
		return r.Version.ID
	}
	v1 := up("v1", "testdata/compatible/users-v1.yaml")
	v2 := up("v2", "testdata/breaking/users-v2-breaking.yaml")
	view, err := svc.Analyze(ctx, actor, proj.ID, v1, v2)
	if err != nil {
		t.Fatal(err)
	}
	stored, err := svc.GetAnalysis(ctx, actor, view.ID)
	if err != nil {
		t.Fatal(err)
	}
	direct, err := compatibility.NewEngine().Compare(parse(t, "testdata/compatible/users-v1.yaml"), parse(t, "testdata/breaking/users-v2-breaking.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	want, _ := json.Marshal(struct {
		Changes []domain.ClassifiedChange
		Scores  *domain.CompatibilityScores
	}{direct.Changes, direct.Scores})
	got, _ := json.Marshal(struct {
		Changes []domain.ClassifiedChange
		Scores  *domain.CompatibilityScores
	}{stored.Analysis.Changes, stored.Analysis.Scores})
	if string(want) != string(got) {
		t.Error("the stored analysis differs from the engine result")
	}
}

func TestEngineIsSafeForConcurrentUse(t *testing.T) {
	a, b := read(t, "testdata/compatible/users-v1.yaml"), read(t, "testdata/breaking/users-v2-breaking.yaml")
	run := func() (string, error) {
		p := specification.NewParser()
		base, err := p.Parse(a)
		if err != nil {
			return "", err
		}
		target, err := p.Parse(b)
		if err != nil {
			return "", err
		}
		an, err := compatibility.NewEngine().Compare(base, target)
		if err != nil {
			return "", err
		}
		out, err := json.Marshal(an)
		return string(out), err
	}
	want, err := run()
	if err != nil {
		t.Fatal(err)
	}
	var wg sync.WaitGroup
	errs := make(chan string, 64)
	for i := 0; i < 32; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			got, err := run()
			if err != nil || got != want {
				errs <- "concurrent analysis differs or failed"
			}
		}()
	}
	wg.Wait()
	close(errs)
	for e := range errs {
		t.Error(e)
	}
}

func TestMockHandlerIsSafeForConcurrentUse(t *testing.T) {
	h := mocking.NewHandler(parse(t, "testdata/compatible/users-v1.yaml"), nil)
	var wg sync.WaitGroup
	failures := make(chan string, 1000)
	for g := 0; g < 40; g++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < 20; i++ {
				rec := newRecorder()
				h.ServeHTTP(rec, newRequest("GET", "/users/u-1"))
				if rec.status() != 200 {
					failures <- "unexpected status"
				}
			}
		}()
	}
	wg.Wait()
	close(failures)
	for f := range failures {
		t.Error(f)
	}
}

func TestMemoryStoreIsSafeForConcurrentUse(t *testing.T) {
	store := persistence.NewMemory()
	ctx := context.Background()
	if err := store.CreateOrgAndUser(ctx, &persistence.Organization{ID: "o", Name: "Org"}, &persistence.User{ID: "u", OrgID: "o", Email: "a@b.co", Role: "admin"}); err != nil {
		t.Fatal(err)
	}
	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			name := string(rune('A'+i%26)) + string(rune('a'+i/26))
			if err := store.CreateProject(ctx, &persistence.Project{ID: name, OrgID: "o", Name: name}); err != nil {
				t.Errorf("CreateProject(%s): %v", name, err)
			}
			if _, err := store.ListProjects(ctx, "o"); err != nil {
				t.Errorf("ListProjects: %v", err)
			}
		}(i)
	}
	wg.Wait()
	if list, _ := store.ListProjects(ctx, "o"); len(list) != 50 {
		t.Errorf("got %d projects, want 50", len(list))
	}
}
