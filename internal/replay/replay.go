// Package replay records HTTP exchanges with secrets and personal data masked,
// and replays them against an allowed target to check consistency.
package replay

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"reflect"
	"sort"
	"strings"
	"sync"

	"github.com/thanhmaitran/compatlab/internal/execution"
)

// Masked replaces every sensitive value in a recording.
const Masked = "***MASKED***"

const maxRecordedBytes = 1 << 20

var defaultSensitiveKeys = []string{
	"password", "passwd", "token", "access_token", "refresh_token", "id_token", "secret", "client_secret",
	"api_key", "apikey", "authorization", "email", "phone", "ssn", "card_number", "cvv",
}

var recordedHeaders = []string{"Content-Type", "Accept", "Authorization", "Cookie", "X-Api-Key"}

var sensitiveHeaders = map[string]bool{"Authorization": true, "Cookie": true, "X-Api-Key": true}

// Entry is one recorded exchange. Bodies are parsed JSON when possible.
type Entry struct {
	Method         string            `json:"method"`
	Path           string            `json:"path"`
	Query          string            `json:"query,omitempty"`
	RequestHeaders map[string]string `json:"request_headers,omitempty"`
	RequestBody    any               `json:"request_body,omitempty"`
	Status         int               `json:"status"`
	ResponseBody   any               `json:"response_body,omitempty"`
}

// Recording is a replayable sequence of exchanges.
type Recording struct {
	Version       string   `json:"version"`
	SensitiveKeys []string `json:"sensitive_keys,omitempty"` // extra keys masked in addition to the defaults
	Entries       []Entry  `json:"entries"`
}

type masker struct{ keys map[string]bool }

func newMasker(extra []string) masker {
	m := masker{keys: map[string]bool{}}
	for _, k := range defaultSensitiveKeys {
		m.keys[k] = true
	}
	for _, k := range extra {
		m.keys[strings.ToLower(k)] = true
	}
	return m
}

func (m masker) json(v any) any {
	switch t := v.(type) {
	case map[string]any:
		out := make(map[string]any, len(t))
		for k, val := range t {
			if m.keys[strings.ToLower(k)] {
				out[k] = Masked
			} else {
				out[k] = m.json(val)
			}
		}
		return out
	case []any:
		out := make([]any, len(t))
		for i, val := range t {
			out[i] = m.json(val)
		}
		return out
	}
	return v
}

func (m masker) body(data []byte) any {
	if len(bytes.TrimSpace(data)) == 0 {
		return nil
	}
	var v any
	if err := json.Unmarshal(data, &v); err == nil {
		return m.json(v)
	}
	if len(data) > 256 {
		data = data[:256]
	}
	return string(data)
}

func (m masker) query(raw url.Values) string {
	out := url.Values{}
	for k, vs := range raw {
		for _, v := range vs {
			if m.keys[strings.ToLower(k)] {
				v = Masked
			}
			out.Add(k, v)
		}
	}
	return out.Encode()
}

// Recorder wraps a handler and records every exchange that passes through it.
type Recorder struct {
	next    http.Handler
	extra   []string
	masker  masker
	mu      sync.Mutex
	entries []Entry
}

// NewRecorder records exchanges handled by next. extraSensitive lists
// additional JSON/query keys to mask.
func NewRecorder(next http.Handler, extraSensitive ...string) *Recorder {
	return &Recorder{next: next, extra: extraSensitive, masker: newMasker(extraSensitive)}
}

type capture struct {
	http.ResponseWriter
	status int
	wrote  bool
	buf    bytes.Buffer
}

func (c *capture) WriteHeader(code int) {
	if !c.wrote {
		c.status, c.wrote = code, true
	}
	c.ResponseWriter.WriteHeader(code)
}

func (c *capture) Write(b []byte) (int, error) {
	if room := maxRecordedBytes - c.buf.Len(); room > 0 {
		c.buf.Write(b[:min(len(b), room)])
	}
	return c.ResponseWriter.Write(b)
}

// ServeHTTP implements http.Handler.
func (r *Recorder) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	var reqBody []byte
	if req.Body != nil {
		orig := req.Body
		reqBody, _ = io.ReadAll(io.LimitReader(orig, maxRecordedBytes+1))
		req.Body = io.NopCloser(io.MultiReader(bytes.NewReader(reqBody), orig))
		if len(reqBody) > maxRecordedBytes {
			reqBody = reqBody[:maxRecordedBytes]
		}
	}
	cw := &capture{ResponseWriter: w, status: http.StatusOK}
	r.next.ServeHTTP(cw, req)

	headers := map[string]string{}
	for _, name := range recordedHeaders {
		if v := req.Header.Get(name); v != "" {
			if sensitiveHeaders[name] {
				v = Masked
			}
			headers[name] = v
		}
	}
	entry := Entry{
		Method:         req.Method,
		Path:           req.URL.Path,
		Query:          r.masker.query(req.URL.Query()),
		RequestHeaders: headers,
		RequestBody:    r.masker.body(reqBody),
		Status:         cw.status,
		ResponseBody:   r.masker.body(cw.buf.Bytes()),
	}
	r.mu.Lock()
	r.entries = append(r.entries, entry)
	r.mu.Unlock()
}

// Recording returns a copy of everything recorded so far.
func (r *Recorder) Recording() Recording {
	r.mu.Lock()
	defer r.mu.Unlock()
	extra := append([]string(nil), r.extra...)
	sort.Strings(extra)
	return Recording{Version: "1", SensitiveKeys: extra, Entries: append([]Entry(nil), r.entries...)}
}

// Options configure a replay.
type Options struct {
	Config     execution.Config
	AuthHeader string // sent when the original request carried credentials; default "Bearer replay-token"
}

// Result is the outcome of replaying one entry.
type Result struct {
	Index   int    `json:"index"`
	Method  string `json:"method"`
	Path    string `json:"path"`
	Passed  bool   `json:"passed"`
	Message string `json:"message,omitempty"`
}

// Replay sends the recorded requests in order and compares status codes and
// masked response bodies with the recording. Masked placeholders are sent in
// place of secrets, so replay is meant for mocks and test environments.
func Replay(ctx context.Context, rec Recording, baseURL string, opts Options) ([]Result, error) {
	cfg := opts.Config
	if err := cfg.Policy.Check(baseURL); err != nil {
		return nil, err
	}
	auth := opts.AuthHeader
	if auth == "" {
		auth = "Bearer replay-token"
	}
	m := newMasker(rec.SensitiveKeys)
	client := execution.NewClient(cfg)
	results := make([]Result, 0, len(rec.Entries))
	for i, e := range rec.Entries {
		if err := ctx.Err(); err != nil {
			return results, err
		}
		res := Result{Index: i, Method: e.Method, Path: e.Path}
		target := strings.TrimRight(baseURL, "/") + e.Path
		if e.Query != "" {
			target += "?" + e.Query
		}
		headers := map[string]string{}
		if ct := e.RequestHeaders["Content-Type"]; ct != "" {
			headers["Content-Type"] = ct
		}
		if v := e.RequestHeaders["Accept"]; v != "" {
			headers["Accept"] = v
		}
		if e.RequestHeaders["Authorization"] != "" {
			headers["Authorization"] = auth
		}
		var body []byte
		switch b := e.RequestBody.(type) {
		case nil:
		case string:
			body = []byte(b)
		default:
			body, _ = json.Marshal(b)
		}
		resp, err := execution.Do(ctx, client, cfg, e.Method, target, headers, body)
		switch {
		case err != nil:
			res.Message = err.Error()
		case resp.Status != e.Status:
			res.Message = fmt.Sprintf("expected status %d, got %d", e.Status, resp.Status)
		case !reflect.DeepEqual(normalize(m.body(resp.Body)), normalize(e.ResponseBody)):
			res.Message = "response body differs from the recording"
		default:
			res.Passed = true
		}
		results = append(results, res)
	}
	return results, nil
}

// normalize round-trips a value through JSON so recordings loaded from disk
// and freshly decoded values compare equal.
func normalize(v any) any {
	data, err := json.Marshal(v)
	if err != nil {
		return v
	}
	var out any
	if err := json.Unmarshal(data, &out); err != nil {
		return v
	}
	return out
}
