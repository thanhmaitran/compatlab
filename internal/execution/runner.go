package execution

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"reflect"
	"strconv"
	"strings"
	"time"

	"github.com/thanhmaitran/compatlab/internal/domain"
	"github.com/thanhmaitran/compatlab/internal/mocking"
	"github.com/thanhmaitran/compatlab/internal/security"
	"github.com/thanhmaitran/compatlab/internal/validation"
)

// Runner executes generated test suites against an allowed target.
type Runner struct {
	Config Config
}

type builtRequest struct {
	path    string
	query   url.Values
	headers map[string]string
	body    []byte
}

func hashHex(parts ...string) string {
	sum := sha256.Sum256([]byte(strings.Join(parts, "\x00")))
	return hex.EncodeToString(sum[:8])
}

// Run executes every case sequentially (for deterministic ordering) and
// returns the results. The target must be allowed by the configured policy.
func (r Runner) Run(ctx context.Context, api *domain.NormalizedAPI, suite *domain.TestSuite, target string) (*domain.TestRun, error) {
	if api == nil || suite == nil {
		return nil, fmt.Errorf("%w: API and suite are required", domain.ErrInvalid)
	}
	cfg := r.Config.withDefaults()
	if len(suite.Cases) > cfg.MaxRequests {
		return nil, fmt.Errorf("%w: suite has %d cases but the limit is %d", domain.ErrInvalid, len(suite.Cases), cfg.MaxRequests)
	}
	if err := cfg.Policy.Check(target); err != nil {
		return nil, err
	}
	api.Normalize()
	client := NewClient(cfg)
	run := &domain.TestRun{ID: hashHex(suite.ID, target), SuiteID: suite.ID, Results: make([]domain.TestResult, 0, len(suite.Cases))}
	for _, c := range suite.Cases {
		if err := ctx.Err(); err != nil {
			return run, err
		}
		run.Results = append(run.Results, r.runCase(ctx, client, cfg, api, c, target))
	}
	return run, nil
}

// RunOnMock starts an in-process mock of the API on a loopback port, runs the
// suite against it and stops the mock.
func RunOnMock(ctx context.Context, api *domain.NormalizedAPI, suite *domain.TestSuite) (*domain.TestRun, error) {
	if api == nil {
		return nil, fmt.Errorf("%w: API is required", domain.ErrInvalid)
	}
	base, stop, err := mocking.StartLocal(mocking.NewHandler(api, nil))
	if err != nil {
		return nil, err
	}
	defer stop()
	u, err := url.Parse(base)
	if err != nil {
		return nil, err
	}
	runner := Runner{Config: Config{Policy: security.TargetPolicy{AllowedHosts: []string{u.Host}}}}
	return runner.Run(ctx, api, suite, base)
}

func (r Runner) runCase(ctx context.Context, client *http.Client, cfg Config, api *domain.NormalizedAPI, c domain.TestCase, target string) domain.TestResult {
	res := domain.TestResult{CaseID: c.ID}
	start := time.Now()
	resp, ep, err := execute(ctx, client, cfg, api, c, target)
	res.Duration = time.Since(start)
	switch {
	case err != nil:
		res.Status, res.Message = domain.TestStatusErrored, err.Error()
	case resp.Status != c.ExpectedStatus:
		res.Status = domain.TestStatusFailed
		res.Message = fmt.Sprintf("expected status %d, got %d", c.ExpectedStatus, resp.Status)
	default:
		res.Status = domain.TestStatusPassed
		if c.Category == domain.CategorySchemaValidation {
			if sr := ep.SuccessResponse(); sr != nil && sr.Schema != nil {
				var v any
				if err := json.Unmarshal(resp.Body, &v); err != nil {
					res.Status, res.Message = domain.TestStatusFailed, "response body is not valid JSON"
				} else if problems := validation.Value(v, sr.Schema); len(problems) > 0 {
					res.Status, res.Message = domain.TestStatusFailed, strings.Join(problems, "; ")
				}
			}
		}
	}
	return res
}

func findEndpoint(api *domain.NormalizedAPI, c domain.TestCase) *domain.APIEndpoint {
	for i := range api.Endpoints {
		if api.Endpoints[i].Path == c.Path && api.Endpoints[i].Method == c.Method {
			return &api.Endpoints[i]
		}
	}
	return nil
}

func execute(ctx context.Context, client *http.Client, cfg Config, api *domain.NormalizedAPI, c domain.TestCase, target string) (*Response, *domain.APIEndpoint, error) {
	ep := findEndpoint(api, c)
	if ep == nil {
		return nil, nil, fmt.Errorf("case %s: endpoint %s %s is not in the specification", c.ID, c.Method, c.Path)
	}
	req := buildRequest(ep, c)
	full := strings.TrimRight(target, "/") + req.path
	if len(req.query) > 0 {
		full += "?" + req.query.Encode()
	}
	resp, err := Do(ctx, client, cfg, ep.Method, full, req.headers, req.body)
	return resp, ep, err
}

func formatValue(v any) string {
	switch t := v.(type) {
	case nil:
		return ""
	case string:
		return t
	case float64:
		return strconv.FormatFloat(t, 'f', -1, 64)
	case bool:
		return strconv.FormatBool(t)
	}
	return fmt.Sprint(v)
}

// buildRequest creates a valid baseline request for the endpoint and applies
// the case's override (a replacement value, or omission when Value is nil).
func buildRequest(ep *domain.APIEndpoint, c domain.TestCase) builtRequest {
	req := builtRequest{path: ep.Path, query: url.Values{}, headers: map[string]string{}}
	for _, p := range ep.Parameters {
		target := string(p.Location) + "." + p.Name
		val, include := "", false
		switch {
		case c.Target == target && c.Value != nil:
			val, include = formatValue(c.Value), true
		case c.Target == target:
			include = false // omitted by the case
		case p.Required:
			val, include = formatValue(mocking.Example(p.Schema)), true
		}
		if p.Location == domain.LocationPath && !include {
			val, include = formatValue(mocking.Example(p.Schema)), true // path values cannot be omitted
		}
		if !include {
			continue
		}
		switch p.Location {
		case domain.LocationPath:
			req.path = strings.ReplaceAll(req.path, "{"+p.Name+"}", url.PathEscape(val))
		case domain.LocationQuery:
			req.query.Set(p.Name, val)
		case domain.LocationHeader:
			req.headers[p.Name] = val
		case domain.LocationCookie:
			if prev := req.headers["Cookie"]; prev != "" {
				req.headers["Cookie"] = prev + "; " + p.Name + "=" + val
			} else {
				req.headers["Cookie"] = p.Name + "=" + val
			}
		}
	}
	if ep.RequestBody != nil && ep.RequestBody.Schema != nil {
		v := mocking.Example(ep.RequestBody.Schema)
		if name, ok := strings.CutPrefix(c.Target, "body."); ok {
			if obj, isObj := v.(map[string]any); isObj {
				if c.Value == nil {
					delete(obj, name)
				} else {
					obj[name] = c.Value
				}
			}
		}
		if data, err := json.Marshal(v); err == nil {
			req.body = data
			req.headers["Content-Type"] = "application/json"
		}
	}
	if len(ep.Security) > 0 && !(c.Category == domain.CategoryAuthentication && c.Variant == "missing") {
		req.headers["Authorization"] = "Bearer test-token"
	}
	return req
}

// Difference compares one case executed against two targets.
type Difference struct {
	CaseID  string `json:"case_id"`
	Name    string `json:"name"`
	StatusA int    `json:"status_a"`
	StatusB int    `json:"status_b"`
	Same    bool   `json:"same"`
	Message string `json:"message,omitempty"`
}

func sameBody(a, b []byte) bool {
	var va, vb any
	if json.Unmarshal(a, &va) == nil && json.Unmarshal(b, &vb) == nil {
		return reflect.DeepEqual(va, vb)
	}
	return bytes.Equal(a, b)
}

// RunDifferential executes every case against two targets and reports where
// status codes or response bodies differ. Both targets must be allowed.
func (r Runner) RunDifferential(ctx context.Context, api *domain.NormalizedAPI, suite *domain.TestSuite, targetA, targetB string) ([]Difference, error) {
	if api == nil || suite == nil {
		return nil, fmt.Errorf("%w: API and suite are required", domain.ErrInvalid)
	}
	cfg := r.Config.withDefaults()
	if len(suite.Cases) > cfg.MaxRequests {
		return nil, fmt.Errorf("%w: suite has %d cases but the limit is %d", domain.ErrInvalid, len(suite.Cases), cfg.MaxRequests)
	}
	for _, t := range []string{targetA, targetB} {
		if err := cfg.Policy.Check(t); err != nil {
			return nil, err
		}
	}
	api.Normalize()
	client := NewClient(cfg)
	out := make([]Difference, 0, len(suite.Cases))
	for _, c := range suite.Cases {
		if err := ctx.Err(); err != nil {
			return out, err
		}
		d := Difference{CaseID: c.ID, Name: c.Name}
		ra, _, errA := execute(ctx, client, cfg, api, c, targetA)
		rb, _, errB := execute(ctx, client, cfg, api, c, targetB)
		switch {
		case errA != nil || errB != nil:
			d.Message = fmt.Sprintf("execution failed (A: %v, B: %v)", errA, errB)
		default:
			d.StatusA, d.StatusB = ra.Status, rb.Status
			switch {
			case ra.Status != rb.Status:
				d.Message = fmt.Sprintf("status %d vs %d", ra.Status, rb.Status)
			case !sameBody(ra.Body, rb.Body):
				d.Message = "response bodies differ"
			default:
				d.Same = true
			}
		}
		out = append(out, d)
	}
	return out, nil
}
