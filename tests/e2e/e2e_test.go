// Package e2e drives a real HTTP server through the complete workflow:
// create project, upload two versions, analyze, verify contracts, generate and
// run tests on a mock, evaluate a policy and export the report.
package e2e_test

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
	"github.com/thanhmaitran/compatlab/internal/reporting"
	"github.com/thanhmaitran/compatlab/internal/service"
)

const root = "../../"

type client struct {
	t     *testing.T
	base  string
	token string
}

func (c *client) do(method, path string, body any, out any) int {
	c.t.Helper()
	var rdr io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			c.t.Fatal(err)
		}
		rdr = bytes.NewReader(data)
	}
	req, err := http.NewRequest(method, c.base+path, rdr)
	if err != nil {
		c.t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/json")
	if c.token != "" {
		req.Header.Set("Authorization", "Bearer "+c.token)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		c.t.Fatal(err)
	}
	defer resp.Body.Close()
	data, _ := io.ReadAll(resp.Body)
	if out != nil && len(data) > 0 {
		if err := json.Unmarshal(data, out); err != nil {
			c.t.Fatalf("%s %s: bad JSON %q: %v", method, path, data, err)
		}
	}
	return resp.StatusCode
}

func (c *client) must(want int, method, path string, body, out any) {
	c.t.Helper()
	if got := c.do(method, path, body, out); got != want {
		c.t.Fatalf("%s %s: status %d, want %d", method, path, got, want)
	}
}

func file(t *testing.T, rel string) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(root, rel))
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

type id struct {
	ID string `json:"id"`
}

func TestCompleteUserWorkflow(t *testing.T) {
	tokens, err := auth.NewTokenIssuer("0123456789abcdef0123456789abcdef", time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	srv := httptest.NewServer(routes.New(routes.Config{}, service.New(persistence.NewMemory(), tokens, nil)))
	defer srv.Close()
	c := &client{t: t, base: srv.URL}

	// 0. account
	var reg struct{ Token string }
	c.must(201, "POST", "/api/v1/auth/register", map[string]any{"organization": "Acme", "email": "admin@acme.io", "password": "correct-horse-1"}, &reg)
	c.token = reg.Token

	// 1. project, 2-3. upload API v1 and v2
	var project id
	c.must(201, "POST", "/api/v1/projects", map[string]any{"name": "Users API"}, &project)
	var up struct {
		Version id `json:"version"`
	}
	c.must(201, "POST", "/api/v1/projects/"+project.ID+"/specifications",
		map[string]any{"version_label": "v1", "filename": "users-v1.yaml", "content": file(t, "testdata/compatible/users-v1.yaml")}, &up)
	v1 := up.Version.ID
	c.must(201, "POST", "/api/v1/projects/"+project.ID+"/specifications",
		map[string]any{"version_label": "v2", "filename": "users-v2.yaml", "content": file(t, "testdata/breaking/users-v2-breaking.yaml")}, &up)
	v2 := up.Version.ID

	// 4-5. analysis detects breaking changes
	var analysis struct {
		ID       string
		Backward domain.AnalysisSummary
	}
	c.must(201, "POST", "/api/v1/projects/"+project.ID+"/analyze", map[string]any{"base_version_id": v1, "target_version_id": v2}, &analysis)
	if analysis.Backward.Breaking == 0 {
		t.Fatal("breaking changes must be detected")
	}

	// 6. consumer contracts
	var contractIDs []string
	for _, name := range []string{"mobile-app.json", "analytics.json"} {
		var contract map[string]any
		if err := json.Unmarshal([]byte(file(t, "tests/fixtures/contracts/"+name)), &contract); err != nil {
			t.Fatal(err)
		}
		var created id
		c.must(201, "POST", "/api/v1/contracts", map[string]any{"project_id": project.ID, "contract": contract}, &created)
		contractIDs = append(contractIDs, created.ID)
	}

	// 7-8. generate tests and run them on a mock of v1
	var suite id
	c.must(201, "POST", "/api/v1/test-suites/generate", map[string]any{"version_id": v1, "analysis_id": analysis.ID, "max_cases_per_endpoint": 20}, &suite)
	var run struct {
		ID       string
		PassRate float64 `json:"pass_rate"`
	}
	c.must(201, "POST", "/api/v1/test-runs", map[string]any{"suite_id": suite.ID, "version_id": v1}, &run)
	if run.PassRate != 100 {
		t.Errorf("pass rate %.1f, want 100", run.PassRate)
	}

	// 9. evaluate the release policy
	var policy id
	c.must(201, "POST", "/api/v1/policies", map[string]any{"project_id": project.ID, "policy": map[string]any{
		"name": "default", "fail_on": []string{"BREAKING"}, "require_contract_tests": true, "minimum_pass_rate": 95}}, &policy)
	var gate struct {
		Report domain.CompatibilityReport `json:"report"`
	}
	c.must(201, "POST", "/api/v1/gate/evaluate", map[string]any{
		"project_id": project.ID, "analysis_id": analysis.ID, "policy_id": policy.ID, "contract_ids": contractIDs, "test_run_id": run.ID}, &gate)
	if gate.Report.Evaluation == nil || gate.Report.Evaluation.Decision != domain.DecisionBlock {
		t.Fatalf("the release must be blocked: %+v", gate.Report.Evaluation)
	}

	// 10. export the report
	text := reporting.Text(&gate.Report)
	for _, want := range []string{"Release decision: BLOCKED", "mobile-app", "analytics", "BREAKING (backward)"} {
		if !strings.Contains(text, want) {
			t.Errorf("exported report is missing %q:\n%s", want, text)
		}
	}
	if js, err := reporting.JSON(&gate.Report); err != nil || !json.Valid(js) {
		t.Errorf("JSON export failed: %v", err)
	}
}
