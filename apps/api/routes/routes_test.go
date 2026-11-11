package routes_test

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/thanhmaitran/compatlab/apps/api/routes"
	"github.com/thanhmaitran/compatlab/internal/auth"
	"github.com/thanhmaitran/compatlab/internal/domain"
	"github.com/thanhmaitran/compatlab/internal/persistence"
	"github.com/thanhmaitran/compatlab/internal/service"
	"github.com/thanhmaitran/compatlab/internal/specification"
)

const root = "../../../"

type env struct {
	t      *testing.T
	router http.Handler
	store  *persistence.Memory
}

func newEnv(t *testing.T, cfg routes.Config) *env {
	t.Helper()
	store := persistence.NewMemory()
	tokens, err := auth.NewTokenIssuer("0123456789abcdef0123456789abcdef", time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	return &env{t: t, router: routes.New(cfg, service.New(store, tokens, nil)), store: store}
}

func (e *env) call(token, method, path string, body any) (int, any) {
	e.t.Helper()
	var rdr io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			e.t.Fatal(err)
		}
		rdr = bytes.NewReader(data)
	}
	req := httptest.NewRequest(method, path, rdr)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	rec := httptest.NewRecorder()
	e.router.ServeHTTP(rec, req)
	var out any
	if rec.Body.Len() > 0 {
		_ = json.Unmarshal(rec.Body.Bytes(), &out)
	}
	return rec.Code, out
}

// get walks a decoded JSON value by map keys (string) and array indexes (int).
func get(v any, path ...any) any {
	for _, p := range path {
		switch k := p.(type) {
		case string:
			m, ok := v.(map[string]any)
			if !ok {
				return nil
			}
			v = m[k]
		case int:
			a, ok := v.([]any)
			if !ok || k >= len(a) {
				return nil
			}
			v = a[k]
		}
	}
	return v
}

func read(t *testing.T, rel string) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(root, rel))
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

func (e *env) expect(want int, token, method, path string, body any) any {
	e.t.Helper()
	code, out := e.call(token, method, path, body)
	if code != want {
		e.t.Fatalf("%s %s: status %d, want %d (%v)", method, path, code, want, out)
	}
	return out
}

func (e *env) register(org, email string) string {
	e.t.Helper()
	out := e.expect(201, "", "POST", "/api/v1/auth/register", map[string]any{"organization": org, "email": email, "password": "correct-horse-1"})
	return get(out, "token").(string)
}

type seeded struct{ project, v1, v2, analysis string }

func (e *env) seed(token string) seeded {
	e.t.Helper()
	proj := e.expect(201, token, "POST", "/api/v1/projects", map[string]any{"name": "Users API", "description": "demo"})
	pid := get(proj, "id").(string)
	upload := func(label, file string) string {
		out := e.expect(201, token, "POST", "/api/v1/projects/"+pid+"/specifications",
			map[string]any{"version_label": label, "filename": filepath.Base(file), "content": read(e.t, file)})
		return get(out, "version", "id").(string)
	}
	v1 := upload("v1.0.0", "testdata/compatible/users-v1.yaml")
	v2 := upload("v2.0.0", "testdata/breaking/users-v2-breaking.yaml")
	an := e.expect(201, token, "POST", "/api/v1/projects/"+pid+"/analyze", map[string]any{"base_version_id": v1, "target_version_id": v2})
	return seeded{project: pid, v1: v1, v2: v2, analysis: get(an, "id").(string)}
}

func TestHealthAndSecurityHeaders(t *testing.T) {
	e := newEnv(t, routes.Config{})
	e.expect(200, "", "GET", "/health", nil)
	e.expect(200, "", "GET", "/ready", nil)
	rec := httptest.NewRecorder()
	e.router.ServeHTTP(rec, httptest.NewRequest("GET", "/health", nil))
	if rec.Header().Get("X-Content-Type-Options") != "nosniff" || rec.Header().Get("Cache-Control") != "no-store" {
		t.Errorf("missing security headers: %v", rec.Header())
	}
}

func TestFullWorkflow(t *testing.T) {
	e := newEnv(t, routes.Config{})
	tok := e.register("Acme", "admin@acme.io")
	s := e.seed(tok)

	an := e.expect(200, tok, "GET", "/api/v1/analyses/"+s.analysis, nil)
	if n, _ := get(an, "backward", "breaking").(float64); n == 0 {
		t.Errorf("expected breaking changes: %v", get(an, "backward"))
	}
	if score, _ := get(an, "scores", "backward").(float64); score > 69 {
		t.Errorf("severe breaking changes must cap the score at 69, got %v", score)
	}
	if changes, _ := get(an, "analysis", "changes").([]any); len(changes) == 0 {
		t.Error("analysis must include its changes")
	}
	list := e.expect(200, tok, "GET", "/api/v1/analyses/"+s.analysis+"/changes?classification=BREAKING&direction=BACKWARD", nil).([]any)
	if len(list) == 0 {
		t.Fatal("expected backward breaking changes")
	}
	for _, c := range list {
		if get(c, "classification") != "BREAKING" || get(c, "direction") != "BACKWARD" {
			t.Errorf("filter not applied: %v", c)
		}
	}
	e.expect(400, tok, "GET", "/api/v1/analyses/"+s.analysis+"/changes?classification=SCARY", nil)

	var contractIDs []string
	for _, name := range []string{"mobile-app.json", "analytics.json"} {
		var contract map[string]any
		if err := json.Unmarshal([]byte(read(t, "tests/fixtures/contracts/"+name)), &contract); err != nil {
			t.Fatal(err)
		}
		out := e.expect(201, tok, "POST", "/api/v1/contracts", map[string]any{"project_id": s.project, "contract": contract})
		contractIDs = append(contractIDs, get(out, "id").(string))
	}
	verifyMobile := e.expect(200, tok, "POST", "/api/v1/contracts/"+contractIDs[0]+"/verify", map[string]any{"version_id": s.v2})
	if get(verifyMobile, "passed") != false {
		t.Error("mobile-app must fail against the breaking version")
	}
	verifyAnalytics := e.expect(200, tok, "POST", "/api/v1/contracts/"+contractIDs[1]+"/verify", map[string]any{"version_id": s.v2})
	if get(verifyAnalytics, "passed") != true {
		t.Errorf("analytics only reads name and must pass: %v", verifyAnalytics)
	}

	suite := e.expect(201, tok, "POST", "/api/v1/test-suites/generate",
		map[string]any{"version_id": s.v1, "analysis_id": s.analysis, "max_cases_per_endpoint": 20})
	if cases, _ := get(suite, "cases").(float64); cases == 0 {
		t.Error("suite must contain cases")
	}
	suiteID := get(suite, "id").(string)
	e.expect(200, tok, "GET", "/api/v1/test-suites/"+suiteID, nil)

	run := e.expect(201, tok, "POST", "/api/v1/test-runs", map[string]any{"suite_id": suiteID, "version_id": s.v1})
	if rate, _ := get(run, "pass_rate").(float64); rate != 100 {
		t.Errorf("a suite generated from v1 must pass against a mock of v1, got %v%%: %v", rate, get(run, "failed"))
	}
	runID := get(run, "id").(string)
	e.expect(200, tok, "GET", "/api/v1/test-runs/"+runID, nil)

	pol := e.expect(201, tok, "POST", "/api/v1/policies", map[string]any{
		"project_id": s.project,
		"policy":     map[string]any{"name": "block-breaking", "fail_on": []string{"BREAKING"}, "require_contract_tests": true, "minimum_pass_rate": 95},
	})
	gate := e.expect(201, tok, "POST", "/api/v1/gate/evaluate", map[string]any{
		"project_id": s.project, "analysis_id": s.analysis, "policy_id": get(pol, "id"),
		"contract_ids": contractIDs, "test_run_id": runID,
	})
	if get(gate, "report", "evaluation", "decision") != "BLOCK" {
		t.Errorf("breaking change must block the release: %v", get(gate, "report", "evaluation"))
	}
	if impacts, _ := get(gate, "report", "consumer_impacts").([]any); len(impacts) == 0 {
		t.Error("the gate report must include consumer impacts")
	}

	counts := map[string]int{"versions": 2, "analyses": 1, "contracts": 2, "policies": 1, "test-runs": 1, "gates": 1}
	for path, want := range counts {
		got := e.expect(200, tok, "GET", "/api/v1/projects/"+s.project+"/"+path, nil).([]any)
		if len(got) != want {
			t.Errorf("%s: %d items, want %d", path, len(got), want)
		}
	}
	stats := e.expect(200, tok, "GET", "/api/v1/projects/"+s.project+"/analytics", nil)
	if get(stats, "total_analyses") != float64(1) || get(stats, "gate_blocks") != float64(1) {
		t.Errorf("analytics wrong: %v", stats)
	}
	if len(get(stats, "pass_rate_trend").([]any)) != 1 || len(get(stats, "top_changed_endpoints").([]any)) == 0 {
		t.Errorf("analytics trends wrong: %v", stats)
	}
	if len(e.store.AuditEvents()) < 8 {
		t.Errorf("mutations must be audited, got %d events", len(e.store.AuditEvents()))
	}
}

func TestAuthentication(t *testing.T) {
	e := newEnv(t, routes.Config{})
	e.register("Acme", "admin@acme.io")
	e.expect(401, "", "GET", "/api/v1/projects", nil)
	e.expect(401, "garbage", "GET", "/api/v1/projects", nil)
	e.expect(409, "", "POST", "/api/v1/auth/register", map[string]any{"organization": "Other", "email": "ADMIN@acme.io", "password": "correct-horse-1"})
	e.expect(400, "", "POST", "/api/v1/auth/register", map[string]any{"organization": "Other", "email": "x@acme.io", "password": "short"})
	e.expect(400, "", "POST", "/api/v1/auth/register", map[string]any{"organization": "Other", "email": "not-an-email", "password": "correct-horse-1"})
	e.expect(401, "", "POST", "/api/v1/auth/login", map[string]any{"email": "admin@acme.io", "password": "wrong-password"})
	e.expect(401, "", "POST", "/api/v1/auth/login", map[string]any{"email": "nobody@acme.io", "password": "correct-horse-1"})
	out := e.expect(200, "", "POST", "/api/v1/auth/login", map[string]any{"email": "admin@acme.io", "password": "correct-horse-1"})
	if tok, _ := get(out, "token").(string); tok == "" {
		t.Error("login must return a token")
	}
}

func TestTenantIsolation(t *testing.T) {
	e := newEnv(t, routes.Config{})
	a := e.register("Org A", "a@example.com")
	b := e.register("Org B", "b@example.com")
	s := e.seed(a)

	if list := e.expect(200, b, "GET", "/api/v1/projects", nil).([]any); len(list) != 0 {
		t.Errorf("org B must not see org A's projects: %v", list)
	}
	base := "/api/v1/projects/" + s.project
	e.expect(404, b, "GET", base, nil)
	e.expect(404, b, "GET", base+"/versions", nil)
	e.expect(404, b, "GET", base+"/analytics", nil)
	e.expect(404, b, "POST", base+"/specifications", map[string]any{"version_label": "x", "content": read(t, "testdata/valid/simple-api.yaml")})
	e.expect(404, b, "POST", base+"/analyze", map[string]any{"base_version_id": s.v1, "target_version_id": s.v2})
	e.expect(404, b, "GET", "/api/v1/analyses/"+s.analysis, nil)
	e.expect(404, b, "POST", "/api/v1/test-suites/generate", map[string]any{"version_id": s.v1})
	e.expect(404, b, "POST", "/api/v1/contracts", map[string]any{"project_id": s.project, "contract": map[string]any{
		"contract_version": "1.0", "consumer": "c", "provider": "p",
		"interactions": []any{map[string]any{"request": map[string]any{"method": "GET", "path": "/x"}, "response": map[string]any{"status": 200}}}}})
}

func TestRoles(t *testing.T) {
	e := newEnv(t, routes.Config{})
	admin := e.register("Acme", "admin@acme.io")
	e.expect(201, admin, "POST", "/api/v1/users", map[string]any{"email": "dev@acme.io", "password": "correct-horse-1"})
	e.expect(409, admin, "POST", "/api/v1/users", map[string]any{"email": "dev@acme.io", "password": "correct-horse-1"})
	e.expect(400, admin, "POST", "/api/v1/users", map[string]any{"email": "x@acme.io", "password": "correct-horse-1", "role": "root"})
	member := get(e.expect(200, "", "POST", "/api/v1/auth/login", map[string]any{"email": "dev@acme.io", "password": "correct-horse-1"}), "token").(string)

	pid := get(e.expect(201, member, "POST", "/api/v1/projects", map[string]any{"name": "Shared"}), "id").(string)
	e.expect(200, admin, "GET", "/api/v1/projects/"+pid, nil) // same organization
	e.expect(403, member, "POST", "/api/v1/policies", map[string]any{"project_id": pid, "policy": map[string]any{"name": "p", "fail_on": []string{"BREAKING"}}})
	e.expect(403, member, "POST", "/api/v1/users", map[string]any{"email": "y@acme.io", "password": "correct-horse-1"})
	e.expect(201, admin, "POST", "/api/v1/policies", map[string]any{"project_id": pid, "policy": map[string]any{"name": "p", "fail_on": []string{"BREAKING"}}})
	e.expect(400, admin, "POST", "/api/v1/policies", map[string]any{"project_id": pid, "policy": map[string]any{"name": "p", "fail_on": []string{"SCARY"}}})
}

func TestSpecificationValidation(t *testing.T) {
	e := newEnv(t, routes.Config{})
	tok := e.register("Acme", "admin@acme.io")
	pid := get(e.expect(201, tok, "POST", "/api/v1/projects", map[string]any{"name": "API"}), "id").(string)
	path := "/api/v1/projects/" + pid + "/specifications"
	upload := func(label, content string) map[string]any {
		return map[string]any{"version_label": label, "content": content}
	}
	e.expect(400, tok, "POST", path, upload("bad", "openapi: 3.0.3\ninfo: [unclosed\n"))
	e.expect(400, tok, "POST", path, upload("bad2", `{"swagger":"2.0","info":{"title":"x","version":"1"}}`))
	e.expect(400, tok, "POST", path, upload("ext", `{"openapi":"3.0.3","info":{"title":"x","version":"1"},"components":{"schemas":{"A":{"$ref":"https://evil.example/s.json"}}}}`))
	e.expect(400, tok, "POST", path, map[string]any{"version_label": "empty"})
	good := read(t, "testdata/valid/simple-api.yaml")
	out := e.expect(201, tok, "POST", path, upload("v1", good))
	if get(out, "endpoints") != float64(3) || get(out, "schemas") != float64(2) {
		t.Errorf("unexpected summary: %v", out)
	}
	e.expect(409, tok, "POST", path, upload("v1", good))
}

func TestRequestLimits(t *testing.T) {
	small := newEnv(t, routes.Config{MaxBodyBytes: 512})
	tok := small.register("Acme", "admin@acme.io")
	small.expect(413, tok, "POST", "/api/v1/projects", map[string]any{"name": "Big", "description": strings.Repeat("x", 2000)})

	limited := newEnv(t, routes.Config{RateLimit: 3})
	token := limited.register("Acme", "admin@acme.io") // request 1 of 3
	limited.expect(200, token, "GET", "/api/v1/projects", nil)
	limited.expect(200, token, "GET", "/api/v1/projects", nil)
	limited.expect(429, token, "GET", "/api/v1/projects", nil)
	limited.expect(200, "", "GET", "/health", nil) // health checks are not rate limited

	auths := newEnv(t, routes.Config{AuthRateLimit: 2})
	auths.register("Acme", "admin@acme.io")
	creds := map[string]any{"email": "admin@acme.io", "password": "wrong-password"}
	auths.expect(401, "", "POST", "/api/v1/auth/login", creds)
	auths.expect(429, "", "POST", "/api/v1/auth/login", creds)
}

func TestTestRunsCannotTargetUnlistedHosts(t *testing.T) {
	e := newEnv(t, routes.Config{})
	tok := e.register("Acme", "admin@acme.io")
	s := e.seed(tok)
	suite := e.expect(201, tok, "POST", "/api/v1/test-suites/generate", map[string]any{"version_id": s.v1, "max_cases_per_endpoint": 5})
	e.expect(400, tok, "POST", "/api/v1/test-runs", map[string]any{"suite_id": get(suite, "id"), "version_id": s.v1, "target": "http://127.0.0.1:9"})
	e.expect(400, tok, "POST", "/api/v1/test-runs", map[string]any{"suite_id": get(suite, "id"), "version_id": s.v1, "target": "http://169.254.169.254/"})
}

func TestMalformedBodiesAreRejected(t *testing.T) {
	e := newEnv(t, routes.Config{})
	tok := e.register("Acme", "admin@acme.io")
	req := httptest.NewRequest("POST", "/api/v1/projects", strings.NewReader("{not json"))
	req.Header.Set("Authorization", "Bearer "+tok)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	e.router.ServeHTTP(rec, req)
	if rec.Code != 400 {
		t.Errorf("malformed JSON: status %d", rec.Code)
	}
	e.expect(400, tok, "POST", "/api/v1/projects", map[string]any{"name": ""})
	e.expect(404, tok, "GET", "/api/v1/analyses/does-not-exist", nil)
}

func TestSpecificationExplorer(t *testing.T) {
	e := newEnv(t, routes.Config{})
	tok := e.register("Acme", "admin@acme.io")
	other := e.register("Other", "other@acme.io")
	s := e.seed(tok)
	spec := e.expect(200, tok, "GET", "/api/v1/versions/"+s.v1+"/specification", nil)
	if eps, _ := get(spec, "endpoints").([]any); len(eps) != 4 {
		t.Errorf("expected 4 endpoints, got %v", get(spec, "endpoints"))
	}
	found := false
	for _, sc := range get(spec, "schemas").([]any) {
		if get(sc, "name") == "User" && len(get(sc, "properties").([]any)) == 4 {
			found = true
		}
	}
	if !found {
		t.Errorf("the User schema must be listed with its properties: %v", get(spec, "schemas"))
	}
	e.expect(404, other, "GET", "/api/v1/versions/"+s.v1+"/specification", nil)
}

// The published API description must match the routes the server registers.
func TestOpenAPIDescriptionMatchesRegisteredRoutes(t *testing.T) {
	api, err := specification.NewParser().Parse([]byte(read(t, "api/openapi/compatlab.yaml")))
	if err != nil {
		t.Fatal(err)
	}
	documented := map[string]bool{}
	for _, ep := range api.Endpoints {
		documented[ep.Key()] = true
	}
	tokens, _ := auth.NewTokenIssuer("0123456789abcdef0123456789abcdef", time.Hour)
	engine := routes.New(routes.Config{}, service.New(persistence.NewMemory(), tokens, nil))
	registered := map[string]bool{}
	for _, r := range engine.Routes() {
		path := r.Path
		for _, seg := range strings.Split(path, "/") {
			if strings.HasPrefix(seg, ":") {
				path = strings.Replace(path, seg, "{"+seg[1:]+"}", 1)
			}
		}
		key := strings.ToUpper(r.Method) + " " + domain.CanonicalPath(path)
		registered[key] = true
		if !documented[key] {
			t.Errorf("route %s is not described in api/openapi/compatlab.yaml", key)
		}
	}
	for key := range documented {
		if !registered[key] {
			t.Errorf("%s is described in the OpenAPI file but not registered", key)
		}
	}
}
