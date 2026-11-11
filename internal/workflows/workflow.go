// Package workflows runs stateful multi-step API scenarios: each step can
// extract values from its response and use them in later steps, with
// assertions, per-step timeouts, retries, a failure policy and cleanup steps.
package workflows

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/thanhmaitran/compatlab/internal/domain"
	"github.com/thanhmaitran/compatlab/internal/execution"
)

// StepResult is the outcome of one workflow step.
type StepResult struct {
	Name       string            `json:"name"`
	Status     domain.TestStatus `json:"status"`
	Attempts   int               `json:"attempts"`
	HTTPStatus int               `json:"http_status,omitempty"`
	Message    string            `json:"message,omitempty"`
	Duration   time.Duration     `json:"duration"`
}

// Result is the outcome of a workflow run.
type Result struct {
	Name    string       `json:"name"`
	Passed  bool         `json:"passed"`
	Steps   []StepResult `json:"steps"`
	Cleanup []StepResult `json:"cleanup,omitempty"`
}

// Runner executes workflows against an allowed target.
type Runner struct {
	Config execution.Config
}

// Run executes the workflow. Steps run in order; after a failure the remaining
// steps are skipped unless on_failure is "continue". Cleanup steps always run.
func (r Runner) Run(ctx context.Context, wf *domain.Workflow, baseURL string) (*Result, error) {
	if wf == nil {
		return nil, fmt.Errorf("%w: workflow is required", domain.ErrInvalid)
	}
	if err := wf.Validate(); err != nil {
		return nil, err
	}
	if err := r.Config.Policy.Check(baseURL); err != nil {
		return nil, err
	}
	client := execution.NewClient(r.Config)
	vars := map[string]string{}
	res := &Result{Name: wf.Name, Passed: true}
	aborted := false
	for _, step := range wf.Steps {
		if aborted {
			res.Steps = append(res.Steps, StepResult{Name: step.Name, Status: domain.TestStatusSkipped, Message: "skipped after an earlier failure"})
			continue
		}
		sr := r.runStep(ctx, client, baseURL, step, vars)
		res.Steps = append(res.Steps, sr)
		if sr.Status != domain.TestStatusPassed {
			res.Passed = false
			if wf.OnFailure != "continue" {
				aborted = true
			}
		}
	}
	for _, step := range wf.Cleanup {
		res.Cleanup = append(res.Cleanup, r.runStep(ctx, client, baseURL, step, vars))
	}
	return res, nil
}

var varPattern = regexp.MustCompile(`\{\{\s*([A-Za-z0-9_.-]+)\s*\}\}`)

func substitute(s string, vars map[string]string) (string, error) {
	var missing string
	out := varPattern.ReplaceAllStringFunc(s, func(m string) string {
		name := varPattern.FindStringSubmatch(m)[1]
		v, ok := vars[name]
		if !ok && missing == "" {
			missing = name
		}
		return v
	})
	if missing != "" {
		return "", fmt.Errorf("undefined variable %q", missing)
	}
	return out, nil
}

func substituteAny(v any, vars map[string]string) (any, error) {
	switch t := v.(type) {
	case string:
		return substitute(t, vars)
	case map[string]any:
		out := make(map[string]any, len(t))
		for k, val := range t {
			n, err := substituteAny(val, vars)
			if err != nil {
				return nil, err
			}
			out[k] = n
		}
		return out, nil
	case []any:
		out := make([]any, len(t))
		for i, val := range t {
			n, err := substituteAny(val, vars)
			if err != nil {
				return nil, err
			}
			out[i] = n
		}
		return out, nil
	}
	return v, nil
}

func jsonPath(v any, path string) (any, bool) {
	if path == "" {
		return v, true
	}
	cur := v
	for _, seg := range strings.Split(path, ".") {
		switch t := cur.(type) {
		case map[string]any:
			next, ok := t[seg]
			if !ok {
				return nil, false
			}
			cur = next
		case []any:
			i, err := strconv.Atoi(seg)
			if err != nil || i < 0 || i >= len(t) {
				return nil, false
			}
			cur = t[i]
		default:
			return nil, false
		}
	}
	return cur, true
}

func stringify(v any) string {
	switch t := v.(type) {
	case nil:
		return "null"
	case string:
		return t
	case float64:
		return strconv.FormatFloat(t, 'f', -1, 64)
	case bool:
		return strconv.FormatBool(t)
	}
	data, _ := json.Marshal(v)
	return string(data)
}

func (r Runner) runStep(ctx context.Context, client *http.Client, baseURL string, step domain.WorkflowStep, vars map[string]string) (sr StepResult) {
	sr.Name = step.Name
	start := time.Now()
	defer func() { sr.Duration = time.Since(start) }()
	fail := func(msg string) StepResult {
		sr.Status, sr.Message = domain.TestStatusFailed, msg
		return sr
	}

	path, err := substitute(step.Path, vars)
	if err != nil {
		return fail(err.Error())
	}
	if !strings.HasPrefix(path, "/") {
		return fail("path must start with /")
	}
	headers := map[string]string{}
	for k, v := range step.Headers {
		sv, err := substitute(v, vars)
		if err != nil {
			return fail(err.Error())
		}
		headers[k] = sv
	}
	var body []byte
	if step.Body != nil {
		sub, err := substituteAny(step.Body, vars)
		if err != nil {
			return fail(err.Error())
		}
		body, _ = json.Marshal(sub)
		headers["Content-Type"] = "application/json"
	}
	attempts := max(step.Retry.MaxAttempts, 1)
	cfg := r.Config
	cfg.Timeout = step.Timeout
	if cfg.Timeout <= 0 {
		cfg.Timeout = 10 * time.Second
	}
	var msg string
	for attempt := 1; attempt <= attempts; attempt++ {
		sr.Attempts = attempt
		extracted, status, errMsg := r.attempt(ctx, client, cfg, baseURL+path, step, headers, body, vars)
		sr.HTTPStatus = status
		if errMsg == "" {
			for k, v := range extracted {
				vars[k] = v
			}
			sr.Status, sr.Message = domain.TestStatusPassed, ""
			return sr
		}
		msg = errMsg
		if attempt < attempts {
			select {
			case <-ctx.Done():
				sr.Status, sr.Message = domain.TestStatusErrored, ctx.Err().Error()
				return sr
			case <-time.After(step.Retry.Backoff):
			}
		}
	}
	return fail(msg)
}

// attempt performs one request and checks it. It returns the extracted
// variables, the HTTP status, and an error message (empty on success).
func (r Runner) attempt(ctx context.Context, client *http.Client, cfg execution.Config, url string, step domain.WorkflowStep,
	headers map[string]string, body []byte, vars map[string]string) (map[string]string, int, string) {
	resp, err := execution.Do(ctx, client, cfg, step.Method, url, headers, body)
	if err != nil {
		return nil, 0, err.Error()
	}
	var parsed any
	bodyOK := json.Unmarshal(resp.Body, &parsed) == nil

	if len(step.Assertions) == 0 && (resp.Status < 200 || resp.Status > 299) {
		return nil, resp.Status, fmt.Sprintf("unexpected status %d", resp.Status)
	}
	for _, a := range step.Assertions {
		expected, err := substitute(a.Expected, vars)
		if err != nil {
			return nil, resp.Status, err.Error()
		}
		if msg := evalAssertion(a, expected, resp, parsed, bodyOK); msg != "" {
			return nil, resp.Status, msg
		}
	}
	extracted := map[string]string{}
	for name, p := range step.Extract {
		if !bodyOK {
			return nil, resp.Status, fmt.Sprintf("cannot extract %q: the response is not JSON", name)
		}
		v, ok := jsonPath(parsed, p)
		if !ok {
			return nil, resp.Status, fmt.Sprintf("cannot extract %q: path %q not found in the response", name, p)
		}
		extracted[name] = stringify(v)
	}
	return extracted, resp.Status, ""
}

// evalAssertion returns an empty string when the assertion holds.
func evalAssertion(a domain.Assertion, expected string, resp *execution.Response, body any, bodyOK bool) string {
	var actual string
	found := true
	switch {
	case a.Target == "status":
		actual = strconv.Itoa(resp.Status)
	case strings.HasPrefix(a.Target, "header."):
		name := strings.TrimPrefix(a.Target, "header.")
		found = len(resp.Header.Values(name)) > 0
		actual = resp.Header.Get(name)
	case strings.HasPrefix(a.Target, "body."):
		var v any
		v, found = jsonPath(body, strings.TrimPrefix(a.Target, "body."))
		found = found && bodyOK
		if found {
			actual = stringify(v)
		}
	default:
		return fmt.Sprintf("unknown assertion target %q", a.Target)
	}
	switch a.Operator {
	case "exists":
		if !found {
			return fmt.Sprintf("%s does not exist", a.Target)
		}
		return ""
	case "eq":
		if !found || actual != expected {
			return fmt.Sprintf("%s: expected %q, got %q", a.Target, expected, actual)
		}
	case "ne":
		if found && actual == expected {
			return fmt.Sprintf("%s: expected a value different from %q", a.Target, expected)
		}
	case "contains":
		if !found || !strings.Contains(actual, expected) {
			return fmt.Sprintf("%s: %q does not contain %q", a.Target, actual, expected)
		}
	case "gt", "lt":
		x, errX := strconv.ParseFloat(actual, 64)
		y, errY := strconv.ParseFloat(expected, 64)
		if !found || errX != nil || errY != nil {
			return fmt.Sprintf("%s: cannot compare %q with %q numerically", a.Target, actual, expected)
		}
		if (a.Operator == "gt" && !(x > y)) || (a.Operator == "lt" && !(x < y)) {
			return fmt.Sprintf("%s: %v is not %s %v", a.Target, x, a.Operator, y)
		}
	default:
		return fmt.Sprintf("unknown assertion operator %q", a.Operator)
	}
	return ""
}
