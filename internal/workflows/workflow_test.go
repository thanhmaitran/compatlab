package workflows

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/thanhmaitran/compatlab/internal/domain"
	"github.com/thanhmaitran/compatlab/internal/execution"
	"github.com/thanhmaitran/compatlab/internal/security"
)

func newServer(t *testing.T) (*httptest.Server, Runner) {
	t.Helper()
	var mu sync.Mutex
	orders := map[string]map[string]any{}
	flaky := 0
	reply := func(w http.ResponseWriter, status int, v any) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		_ = json.NewEncoder(w).Encode(v)
	}
	mux := http.NewServeMux()
	mux.HandleFunc("POST /orders", func(w http.ResponseWriter, r *http.Request) {
		var in map[string]any
		_ = json.NewDecoder(r.Body).Decode(&in)
		mu.Lock()
		defer mu.Unlock()
		orders["o-1"] = map[string]any{"id": "o-1", "status": "created", "sku": in["sku"]}
		reply(w, 201, orders["o-1"])
	})
	mux.HandleFunc("GET /orders/{id}", func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		defer mu.Unlock()
		if o, ok := orders[r.PathValue("id")]; ok {
			reply(w, 200, o)
			return
		}
		reply(w, 404, map[string]any{"error": "not found"})
	})
	mux.HandleFunc("PATCH /orders/{id}", func(w http.ResponseWriter, r *http.Request) {
		var in map[string]any
		_ = json.NewDecoder(r.Body).Decode(&in)
		mu.Lock()
		defer mu.Unlock()
		o, ok := orders[r.PathValue("id")]
		if !ok {
			reply(w, 404, map[string]any{"error": "not found"})
			return
		}
		o["status"] = in["status"]
		reply(w, 200, o)
	})
	mux.HandleFunc("DELETE /orders/{id}", func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		defer mu.Unlock()
		delete(orders, r.PathValue("id"))
		w.WriteHeader(204)
	})
	mux.HandleFunc("GET /flaky", func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		flaky++
		n := flaky
		mu.Unlock()
		if n < 3 {
			reply(w, 503, map[string]any{"error": "try again"})
			return
		}
		reply(w, 200, map[string]any{"ok": true})
	})
	mux.HandleFunc("GET /slow", func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(400 * time.Millisecond)
		reply(w, 200, map[string]any{})
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	u, _ := url.Parse(srv.URL)
	return srv, Runner{Config: execution.Config{Policy: security.TargetPolicy{AllowedHosts: []string{u.Host}}}}
}

func eq(target, expected string) domain.Assertion {
	return domain.Assertion{Target: target, Operator: "eq", Expected: expected}
}

func lifecycle() *domain.Workflow {
	return &domain.Workflow{
		Name: "order lifecycle",
		Steps: []domain.WorkflowStep{
			{Name: "create", Method: "POST", Path: "/orders", Body: map[string]any{"sku": "abc"},
				Extract: map[string]string{"order_id": "id"}, Assertions: []domain.Assertion{eq("status", "201"), eq("body.status", "created")}},
			{Name: "get", Method: "GET", Path: "/orders/{{order_id}}", Assertions: []domain.Assertion{eq("body.sku", "abc")}},
			{Name: "pay", Method: "PATCH", Path: "/orders/{{order_id}}", Body: map[string]any{"status": "paid"}, Assertions: []domain.Assertion{eq("body.status", "paid")}},
			{Name: "verify", Method: "GET", Path: "/orders/{{order_id}}", Assertions: []domain.Assertion{eq("body.status", "paid"), eq("body.id", "{{order_id}}")}},
			{Name: "cancel", Method: "DELETE", Path: "/orders/{{order_id}}", Assertions: []domain.Assertion{eq("status", "204")}},
		},
	}
}

func TestWorkflowPassesAndPassesDataBetweenSteps(t *testing.T) {
	srv, runner := newServer(t)
	res, err := runner.Run(context.Background(), lifecycle(), srv.URL)
	if err != nil {
		t.Fatal(err)
	}
	for _, s := range res.Steps {
		if s.Status != domain.TestStatusPassed {
			t.Errorf("step %s: %s %s", s.Name, s.Status, s.Message)
		}
	}
	if !res.Passed || len(res.Steps) != 5 {
		t.Errorf("unexpected result: %+v", res)
	}
}

func TestFailureSkipsRemainingStepsButRunsCleanup(t *testing.T) {
	srv, runner := newServer(t)
	wf := lifecycle()
	wf.Steps[1].Assertions = []domain.Assertion{eq("body.sku", "WRONG")}
	wf.Cleanup = []domain.WorkflowStep{{Name: "remove", Method: "DELETE", Path: "/orders/{{order_id}}"}}
	res, err := runner.Run(context.Background(), wf, srv.URL)
	if err != nil {
		t.Fatal(err)
	}
	if res.Passed || res.Steps[1].Status != domain.TestStatusFailed {
		t.Errorf("step 2 should fail: %+v", res.Steps[1])
	}
	for _, s := range res.Steps[2:] {
		if s.Status != domain.TestStatusSkipped {
			t.Errorf("step %s should be skipped, got %s", s.Name, s.Status)
		}
	}
	if len(res.Cleanup) != 1 || res.Cleanup[0].Status != domain.TestStatusPassed {
		t.Errorf("cleanup must run after a failure: %+v", res.Cleanup)
	}
}

func TestOnFailureContinueRunsLaterSteps(t *testing.T) {
	srv, runner := newServer(t)
	wf := lifecycle()
	wf.OnFailure = "continue"
	wf.Steps[1].Assertions = []domain.Assertion{eq("body.sku", "WRONG")}
	res, _ := runner.Run(context.Background(), wf, srv.URL)
	if res.Passed || res.Steps[2].Status != domain.TestStatusPassed {
		t.Errorf("later steps must still run with on_failure=continue: %+v", res.Steps)
	}
}

func TestRetryPolicy(t *testing.T) {
	srv, runner := newServer(t)
	ok := &domain.Workflow{Name: "retry", Steps: []domain.WorkflowStep{
		{Name: "flaky", Method: "GET", Path: "/flaky", Retry: domain.RetryPolicy{MaxAttempts: 3, Backoff: time.Millisecond}},
	}}
	res, _ := runner.Run(context.Background(), ok, srv.URL)
	if !res.Passed || res.Steps[0].Attempts != 3 {
		t.Errorf("expected success on the third attempt: %+v", res.Steps[0])
	}
	srv2, runner2 := newServer(t)
	bad := &domain.Workflow{Name: "retry", Steps: []domain.WorkflowStep{
		{Name: "flaky", Method: "GET", Path: "/flaky", Retry: domain.RetryPolicy{MaxAttempts: 2, Backoff: time.Millisecond}},
	}}
	res, _ = runner2.Run(context.Background(), bad, srv2.URL)
	if res.Passed || res.Steps[0].Attempts != 2 {
		t.Errorf("expected failure after two attempts: %+v", res.Steps[0])
	}
}

func TestTimeoutDoesNotHang(t *testing.T) {
	srv, runner := newServer(t)
	wf := &domain.Workflow{Name: "slow", Steps: []domain.WorkflowStep{
		{Name: "slow", Method: "GET", Path: "/slow", Timeout: 50 * time.Millisecond},
	}}
	start := time.Now()
	res, _ := runner.Run(context.Background(), wf, srv.URL)
	if res.Passed || time.Since(start) > 3*time.Second {
		t.Errorf("a slow step must fail quickly: %+v after %v", res.Steps[0], time.Since(start))
	}
}

func TestUndefinedVariableFailsTheStep(t *testing.T) {
	srv, runner := newServer(t)
	wf := &domain.Workflow{Name: "x", Steps: []domain.WorkflowStep{{Name: "get", Method: "GET", Path: "/orders/{{nope}}"}}}
	res, _ := runner.Run(context.Background(), wf, srv.URL)
	if res.Passed || res.Steps[0].Message == "" {
		t.Errorf("expected an undefined-variable failure: %+v", res.Steps[0])
	}
}

func TestTargetMustBeAllowed(t *testing.T) {
	_, err := Runner{}.Run(context.Background(), lifecycle(), "http://127.0.0.1:9")
	if !errors.Is(err, security.ErrTargetNotAllowed) {
		t.Errorf("expected ErrTargetNotAllowed, got %v", err)
	}
	if _, err := (Runner{}).Run(context.Background(), &domain.Workflow{}, "http://x"); !errors.Is(err, domain.ErrInvalid) {
		t.Errorf("expected ErrInvalid for an empty workflow, got %v", err)
	}
}

func TestParseFile(t *testing.T) {
	doc := `{"name":"w","on_failure":"continue","steps":[{"name":"s","method":"GET","path":"/x","timeout_ms":250,"retry":{"max_attempts":2,"backoff_ms":10}}],"cleanup":[{"name":"c","method":"DELETE","path":"/x"}]}`
	wf, err := ParseFile([]byte(doc))
	if err != nil {
		t.Fatal(err)
	}
	if wf.Steps[0].Timeout != 250*time.Millisecond || wf.Steps[0].Retry.Backoff != 10*time.Millisecond || len(wf.Cleanup) != 1 {
		t.Errorf("unexpected workflow: %+v", wf)
	}
	for _, bad := range []string{`{`, `{"name":"w","steps":[],"extra":1}`, `{"name":"w","steps":[]}`, `{"name":"w","on_failure":"explode","steps":[{"name":"a"}]}`} {
		if _, err := ParseFile([]byte(bad)); !errors.Is(err, domain.ErrInvalid) {
			t.Errorf("%s: expected ErrInvalid, got %v", bad, err)
		}
	}
}

func TestJSONPathAndAssertions(t *testing.T) {
	var doc any
	_ = json.Unmarshal([]byte(`{"a":{"b":[{"c":7}]},"s":"hello"}`), &doc)
	if v, ok := jsonPath(doc, "a.b.0.c"); !ok || stringify(v) != "7" {
		t.Errorf("jsonPath = %v,%v", v, ok)
	}
	if _, ok := jsonPath(doc, "a.b.5"); ok {
		t.Error("out-of-range index must not resolve")
	}
	resp := &execution.Response{Status: 200, Header: http.Header{"X-Id": []string{"9"}}}
	tests := []struct {
		a    domain.Assertion
		exp  string
		fail bool
	}{
		{domain.Assertion{Target: "body.s", Operator: "contains"}, "ell", false},
		{domain.Assertion{Target: "body.s", Operator: "ne"}, "other", false},
		{domain.Assertion{Target: "body.a.b.0.c", Operator: "gt"}, "5", false},
		{domain.Assertion{Target: "body.a.b.0.c", Operator: "lt"}, "5", true},
		{domain.Assertion{Target: "header.X-Id", Operator: "eq"}, "9", false},
		{domain.Assertion{Target: "body.missing", Operator: "exists"}, "", true},
		{domain.Assertion{Target: "body.s", Operator: "regex"}, "x", true},
		{domain.Assertion{Target: "cookie.x", Operator: "eq"}, "x", true},
	}
	for _, tc := range tests {
		msg := evalAssertion(tc.a, tc.exp, resp, doc, true)
		if (msg != "") != tc.fail {
			t.Errorf("%+v expected failure=%v, got %q", tc.a, tc.fail, msg)
		}
	}
}

func TestSampleWorkflowFileIsValid(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("..", "..", "testdata", "workflows", "order-lifecycle.json"))
	if err != nil {
		t.Fatal(err)
	}
	wf, err := ParseFile(data)
	if err != nil {
		t.Fatal(err)
	}
	if len(wf.Steps) < 3 || len(wf.Cleanup) == 0 {
		t.Errorf("unexpected sample workflow: %+v", wf)
	}
}
