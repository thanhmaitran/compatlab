package mocking_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/thanhmaitran/compatlab/internal/domain"
	"github.com/thanhmaitran/compatlab/internal/mocking"
	"github.com/thanhmaitran/compatlab/internal/specification"
	"github.com/thanhmaitran/compatlab/internal/validation"
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

func do(h http.Handler, method, target, body string, headers map[string]string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, target, strings.NewReader(body))
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func TestExamplesSatisfyTheirSchemas(t *testing.T) {
	for _, file := range []string{"valid/simple-api.yaml", "valid/payment-api.yaml", "compatible/users-v1.yaml", "breaking/users-v2-breaking.yaml"} {
		api := load(t, file)
		for name, s := range api.Schemas {
			if problems := validation.Value(mocking.Example(s), s); len(problems) > 0 {
				t.Errorf("%s schema %s: example violates its schema: %v", file, name, problems)
			}
		}
	}
}

func TestRoutingAndMethods(t *testing.T) {
	h := mocking.NewHandler(load(t, "compatible/users-v1.yaml"), nil)
	if rec := do(h, "GET", "/nope", "", nil); rec.Code != 404 {
		t.Errorf("unknown route: %d", rec.Code)
	}
	rec := do(h, "DELETE", "/users", "", nil)
	if rec.Code != 405 || !strings.Contains(rec.Header().Get("Allow"), "GET") {
		t.Errorf("wrong method: %d allow=%q", rec.Code, rec.Header().Get("Allow"))
	}
	rec = do(h, "GET", "/users/u-1", "", nil)
	var body map[string]any
	if rec.Code != 200 || json.Unmarshal(rec.Body.Bytes(), &body) != nil || body["id"] == nil {
		t.Errorf("GET /users/u-1: %d %s", rec.Code, rec.Body.String())
	}
	if rec := do(h, "DELETE", "/users/u-1", "", nil); rec.Code != 204 {
		t.Errorf("DELETE /users/u-1: %d", rec.Code)
	}
}

func TestRequestValidation(t *testing.T) {
	h := mocking.NewHandler(load(t, "compatible/users-v1.yaml"), nil)
	tests := []struct {
		name, method, target, body string
		want                       int
	}{
		{"valid body", "POST", "/users", `{"email":"a@b.co"}`, 201},
		{"missing required field", "POST", "/users", `{"name":"x"}`, 400},
		{"invalid JSON", "POST", "/users", `{`, 400},
		{"bad enum", "POST", "/users", `{"email":"a@b.co","role":"root"}`, 400},
		{"empty body", "POST", "/users", ``, 400},
		{"valid query", "GET", "/users?page=2", "", 200},
		{"non-numeric query", "GET", "/users?page=abc", "", 400},
		{"below minimum", "GET", "/users?page=0", "", 400},
	}
	for _, tc := range tests {
		if rec := do(h, tc.method, tc.target, tc.body, nil); rec.Code != tc.want {
			t.Errorf("%s: status %d, want %d (%s)", tc.name, rec.Code, tc.want, rec.Body.String())
		}
	}
}

func TestAuthenticationIsEnforcedWhereSecured(t *testing.T) {
	h := mocking.NewHandler(load(t, "breaking/users-v2-breaking.yaml"), nil)
	if rec := do(h, "GET", "/users/u-1", "", nil); rec.Code != 401 {
		t.Errorf("no credentials: %d", rec.Code)
	}
	if rec := do(h, "GET", "/users/u-1", "", map[string]string{"Authorization": "Bearer t"}); rec.Code != 200 {
		t.Errorf("with credentials: %d", rec.Code)
	}
	if rec := do(h, "DELETE", "/users/u-1", "", nil); rec.Code != 405 {
		t.Errorf("removed method must be 405, got %d", rec.Code)
	}
}

func TestStatusSelectionAndScenarios(t *testing.T) {
	scenarios := []domain.MockScenario{{Name: "down", Method: "GET", Path: "/users/{id}", Status: 503, Body: map[string]any{"error": "down"}}}
	h := mocking.NewHandler(load(t, "compatible/users-v1.yaml"), scenarios)
	if rec := do(h, "GET", "/users/u-1", "", map[string]string{"X-Mock-Status": "404"}); rec.Code != 404 {
		t.Errorf("documented status: %d", rec.Code)
	}
	if rec := do(h, "GET", "/users/u-1", "", map[string]string{"X-Mock-Status": "418"}); rec.Code != 400 {
		t.Errorf("undocumented status must be rejected, got %d", rec.Code)
	}
	if rec := do(h, "GET", "/users/u-1", "", map[string]string{"X-Mock-Scenario": "down"}); rec.Code != 503 {
		t.Errorf("scenario: %d", rec.Code)
	}
	if rec := do(h, "GET", "/users/u-1", "", map[string]string{"X-Mock-Scenario": "nope"}); rec.Code != 404 {
		t.Errorf("unknown scenario: %d", rec.Code)
	}
}

func TestResponsesAreDeterministic(t *testing.T) {
	h := mocking.NewHandler(load(t, "compatible/users-v1.yaml"), nil)
	first := do(h, "GET", "/users/u-1", "", nil).Body.String()
	for i := 0; i < 5; i++ {
		if got := do(h, "GET", "/users/u-1", "", nil).Body.String(); got != first {
			t.Fatal("mock responses must be deterministic")
		}
	}
}

func TestStartLocal(t *testing.T) {
	base, stop, err := mocking.StartLocal(mocking.NewHandler(load(t, "valid/simple-api.yaml"), nil))
	if err != nil {
		t.Fatal(err)
	}
	defer stop()
	resp, err := http.Get(base + "/health")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Errorf("status %d", resp.StatusCode)
	}
}
