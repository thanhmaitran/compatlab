package commands

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const (
	root       = "../../../"
	v1         = root + "testdata/compatible/users-v1.yaml"
	v2         = root + "testdata/compatible/users-v2.yaml"
	v2Breaking = root + "testdata/breaking/users-v2-breaking.yaml"
	policy     = root + "configs/release-policy.yaml"
	mobile     = root + "tests/fixtures/contracts/mobile-app.json"
	analytics  = root + "tests/fixtures/contracts/analytics.json"
)

func run(args ...string) (int, string, string) {
	var out, errb bytes.Buffer
	code := Run(args, &out, &errb)
	return code, out.String(), errb.String()
}

func TestNoArgsAndUnknownCommandAreInvalid(t *testing.T) {
	if code, _, _ := run(); code != ExitInvalid {
		t.Errorf("no args: exit %d, want %d", code, ExitInvalid)
	}
	if code, _, _ := run("frobnicate"); code != ExitInvalid {
		t.Errorf("unknown command: exit %d, want %d", code, ExitInvalid)
	}
}

func TestVersionAndHelp(t *testing.T) {
	if code, out, _ := run("version"); code != ExitOK || !strings.Contains(out, "compatlab") {
		t.Errorf("version: %d %q", code, out)
	}
	if code, out, _ := run("help"); code != ExitOK || !strings.Contains(out, "Exit codes") {
		t.Errorf("help: %d %q", code, out)
	}
}

func TestSpecValidate(t *testing.T) {
	code, out, _ := run("spec", "validate", v1)
	if code != ExitOK || !strings.Contains(out, "OK: Users API 1.0.0") || !strings.Contains(out, "4 endpoints") {
		t.Errorf("exit %d output %q", code, out)
	}
	if code, _, _ := run("spec", "validate", root+"testdata/invalid/malformed-yaml.yaml"); code != ExitInvalid {
		t.Errorf("malformed spec: exit %d, want %d", code, ExitInvalid)
	}
	if code, _, _ := run("spec", "validate", root+"testdata/invalid/missing-openapi-version.yaml"); code != ExitInvalid {
		t.Errorf("invalid spec: exit %d, want %d", code, ExitInvalid)
	}
	if code, _, _ := run("spec", "validate", "does-not-exist.yaml"); code != ExitInvalid {
		t.Errorf("missing file: exit %d, want %d", code, ExitInvalid)
	}
	if code, _, _ := run("spec", "validate"); code != ExitInvalid {
		t.Errorf("missing argument: exit %d, want %d", code, ExitInvalid)
	}
}

func TestSpecInspect(t *testing.T) {
	code, out, _ := run("spec", "inspect", v1)
	if code != ExitOK || !strings.Contains(out, "GET") || !strings.Contains(out, "/users/{id}") || !strings.Contains(out, "User") {
		t.Errorf("exit %d output %q", code, out)
	}
}

func TestDiffTextAndJSON(t *testing.T) {
	code, out, _ := run("diff", "--base", v1, "--target", v2Breaking)
	if code != ExitOK || !strings.Contains(out, "METHOD_REMOVED") {
		t.Errorf("diff text: exit %d output %q", code, out)
	}
	code, out, _ = run("diff", "--base", v1, "--target", v2, "--format", "json")
	var changes []map[string]any
	if code != ExitOK || json.Unmarshal([]byte(out), &changes) != nil || len(changes) == 0 {
		t.Errorf("diff json: exit %d output %q", code, out)
	}
	if code, _, _ := run("diff", "--base", v1); code != ExitInvalid {
		t.Errorf("missing --target: exit %d, want %d", code, ExitInvalid)
	}
}

func TestAnalyzeFormats(t *testing.T) {
	code, out, _ := run("analyze", "--base", v1, "--target", v2Breaking)
	if code != ExitOK || !strings.Contains(out, "COMPATLAB RELEASE CHECK") {
		t.Errorf("text: exit %d", code)
	}
	code, out, _ = run("analyze", "--base", v1, "--target", v2Breaking, "--format", "json")
	var report map[string]any
	if code != ExitOK || json.Unmarshal([]byte(out), &report) != nil || report["analysis"] == nil {
		t.Errorf("json: exit %d", code)
	}
	if code, out, _ = run("analyze", "--base", v1, "--target", v2Breaking, "--format", "html"); code != ExitOK || !strings.Contains(out, "<!doctype html>") {
		t.Errorf("html: exit %d", code)
	}
	if code, _, _ = run("analyze", "--base", v1, "--target", v2, "--format", "yaml"); code != ExitInvalid {
		t.Errorf("unknown format: exit %d, want %d", code, ExitInvalid)
	}
}

func TestAnalyzeWritesOutputFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "report.json")
	code, out, _ := run("analyze", "--base", v1, "--target", v2, "--format", "json", "--output", path)
	if code != ExitOK || out != "" {
		t.Fatalf("exit %d, stdout %q", code, out)
	}
	data, err := os.ReadFile(path)
	if err != nil || !json.Valid(data) {
		t.Errorf("output file invalid: %v", err)
	}
}

func TestGatePassesForCompatibleChange(t *testing.T) {
	code, out, errOut := run("gate", "--base", v1, "--target", v2, "--policy", policy)
	if code != ExitOK {
		t.Fatalf("exit %d, want 0\n%s\n%s", code, out, errOut)
	}
	if !strings.Contains(out, "Release decision: PASSED") {
		t.Errorf("unexpected output:\n%s", out)
	}
}

func TestGateBlocksBreakingChange(t *testing.T) {
	code, out, _ := run("gate", "--base", v1, "--target", v2Breaking, "--policy", policy,
		"--contract", mobile, "--contract", analytics)
	if code != ExitRejected {
		t.Fatalf("exit %d, want %d\n%s", code, ExitRejected, out)
	}
	for _, want := range []string{"Release decision: BLOCKED", "mobile-app", "analytics", "violate the policy"} {
		if !strings.Contains(out, want) {
			t.Errorf("output missing %q:\n%s", want, out)
		}
	}
}

func TestGateRequiresPolicyAndValidFiles(t *testing.T) {
	if code, _, _ := run("gate", "--base", v1, "--target", v2); code != ExitInvalid {
		t.Errorf("missing policy: exit %d, want %d", code, ExitInvalid)
	}
	if code, _, _ := run("gate", "--base", v1, "--target", v2, "--policy", "missing.yaml"); code != ExitInvalid {
		t.Errorf("missing policy file: exit %d, want %d", code, ExitInvalid)
	}
}

func TestContractVerifyExitCodes(t *testing.T) {
	if code, out, _ := run("contract", "verify", "--spec", v1, "--contract", mobile, "--contract", analytics); code != ExitOK {
		t.Errorf("against v1: exit %d\n%s", code, out)
	}
	code, out, _ := run("contract", "verify", "--spec", v2Breaking, "--contract", mobile, "--contract", analytics)
	if code != ExitRejected || !strings.Contains(out, "FAIL") {
		t.Errorf("against breaking v2: exit %d\n%s", code, out)
	}
	if code, _, _ := run("contract", "verify", "--spec", v1); code != ExitInvalid {
		t.Errorf("no contracts: exit %d, want %d", code, ExitInvalid)
	}
}

func TestTestGenerate(t *testing.T) {
	code, out, errOut := run("test", "generate", "--spec", v1, "--max-per-endpoint", "5", "--seed", "3")
	if code != ExitOK {
		t.Fatalf("exit %d: %s", code, errOut)
	}
	var suite struct {
		GeneratorVersion string           `json:"generator_version"`
		Cases            []map[string]any `json:"cases"`
	}
	if err := json.Unmarshal([]byte(out), &suite); err != nil || suite.GeneratorVersion == "" || len(suite.Cases) == 0 {
		t.Fatalf("invalid suite output: %v\n%s", err, out)
	}
	if len(suite.Cases) > 4*5 {
		t.Errorf("limit not applied: %d cases for 4 endpoints", len(suite.Cases))
	}
	if code, _, _ := run("test", "generate"); code != ExitInvalid {
		t.Errorf("missing --spec: exit %d, want %d", code, ExitInvalid)
	}
	if code, _, _ := run("test"); code != ExitInvalid {
		t.Errorf("missing subcommand: exit %d, want %d", code, ExitInvalid)
	}
}

func TestTestRunOnMockPasses(t *testing.T) {
	code, out, errOut := run("test", "run", "--spec", v1)
	if code != ExitOK || !strings.Contains(out, "0 failed, 0 errored") {
		t.Fatalf("exit %d\n%s\n%s", code, out, errOut)
	}
}

func TestTestRunRejectsTargetsOutsideTheAllowlist(t *testing.T) {
	if code, _, _ := run("test", "run", "--spec", v1, "--target", "http://127.0.0.1:9"); code != ExitInvalid {
		t.Errorf("exit %d, want %d", code, ExitInvalid)
	}
}

func TestTestRunDetectsFailingTarget(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTeapot)
	}))
	defer srv.Close()
	u, _ := url.Parse(srv.URL)
	code, out, _ := run("test", "run", "--spec", v1, "--target", srv.URL, "--allow-host", u.Host)
	if code != ExitRejected || !strings.Contains(out, "FAILED") {
		t.Errorf("exit %d\n%s", code, out)
	}
}

func TestMockRefusesNonLoopbackAddresses(t *testing.T) {
	if code, _, _ := run("mock", "--spec", v1, "--addr", "0.0.0.0:9"); code != ExitInvalid {
		t.Errorf("exit %d, want %d", code, ExitInvalid)
	}
	if code, _, _ := run("mock"); code != ExitInvalid {
		t.Errorf("missing --spec: exit %d, want %d", code, ExitInvalid)
	}
}

func TestWorkflowRun(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	defer srv.Close()
	u, _ := url.Parse(srv.URL)
	dir := t.TempDir()
	good := filepath.Join(dir, "good.json")
	bad := filepath.Join(dir, "bad.json")
	step := `{"name":"ping","method":"GET","path":"/ping","assertions":[{"target":"body.ok","operator":"eq","expected":"%s"}]}`
	_ = os.WriteFile(good, []byte(`{"name":"w","steps":[`+strings.Replace(step, "%s", "true", 1)+`]}`), 0o644)
	_ = os.WriteFile(bad, []byte(`{"name":"w","steps":[`+strings.Replace(step, "%s", "false", 1)+`]}`), 0o644)
	if code, out, _ := run("workflow", "run", "--file", good, "--target", srv.URL, "--allow-host", u.Host); code != ExitOK {
		t.Errorf("good workflow: exit %d\n%s", code, out)
	}
	if code, _, _ := run("workflow", "run", "--file", bad, "--target", srv.URL, "--allow-host", u.Host); code != ExitRejected {
		t.Errorf("failing workflow: exit %d, want %d", code, ExitRejected)
	}
	if code, _, _ := run("workflow", "run", "--file", good, "--target", srv.URL); code != ExitInvalid {
		t.Errorf("target without allowlist: exit %d, want %d", code, ExitInvalid)
	}
}
