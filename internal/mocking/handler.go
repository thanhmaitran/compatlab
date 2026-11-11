package mocking

import (
	"bytes"
	"encoding/json"
	"io"
	"net"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/thanhmaitran/compatlab/internal/domain"
	"github.com/thanhmaitran/compatlab/internal/validation"
)

const maxBodyBytes = 1 << 20

// Handler is an http.Handler that mocks an API.
//
// Behaviour: unknown route 404; known route with another method 405; secured
// operations need an Authorization header (401); invalid parameters or body
// get the documented client-error status; otherwise the first documented 2xx
// response is returned with a deterministic example body.
//
// Headers: X-Mock-Status selects another documented status; X-Mock-Scenario
// selects a named scenario, which overrides everything else.
type Handler struct {
	api       *domain.NormalizedAPI
	scenarios []domain.MockScenario
}

// NewHandler builds a mock for the API. The API is normalized in place.
func NewHandler(api *domain.NormalizedAPI, scenarios []domain.MockScenario) *Handler {
	api.Normalize()
	return &Handler{api: api, scenarios: scenarios}
}

func writeJSON(w http.ResponseWriter, status int, body any) {
	data, err := json.Marshal(body)
	if err != nil {
		http.Error(w, "mock could not encode the response", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_, _ = w.Write(data)
}

func errBody(msg string) map[string]any { return map[string]any{"error": msg} }

func (h *Handler) route(method, path string) (*domain.APIEndpoint, []string) {
	var best *domain.APIEndpoint
	bestVars := -1
	methods := map[string]bool{}
	for i := range h.api.Endpoints {
		e := &h.api.Endpoints[i]
		vars, ok := domain.MatchPath(e.Path, path)
		if !ok {
			continue
		}
		methods[e.Method] = true
		if !strings.EqualFold(e.Method, method) {
			continue
		}
		if best == nil || vars < bestVars || (vars == bestVars && e.Path < best.Path) {
			best, bestVars = e, vars
		}
	}
	if best != nil {
		return best, nil
	}
	allowed := make([]string, 0, len(methods))
	for m := range methods {
		allowed = append(allowed, m)
	}
	sort.Strings(allowed)
	return nil, allowed
}

// ServeHTTP implements http.Handler.
func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	ep, allowed := h.route(r.Method, r.URL.Path)
	if ep == nil {
		if len(allowed) > 0 {
			w.Header().Set("Allow", strings.Join(allowed, ", "))
			writeJSON(w, http.StatusMethodNotAllowed, errBody("method not allowed"))
			return
		}
		writeJSON(w, http.StatusNotFound, errBody("no such route"))
		return
	}
	if name := r.Header.Get("X-Mock-Scenario"); name != "" {
		h.serveScenario(w, r, name)
		return
	}
	if len(ep.Security) > 0 && strings.TrimSpace(r.Header.Get("Authorization")) == "" {
		writeJSON(w, http.StatusUnauthorized, errBody("missing credentials"))
		return
	}
	if problems := validateRequest(ep, r); len(problems) > 0 {
		writeJSON(w, ep.ClientErrorStatus(), map[string]any{"error": "invalid request", "details": problems})
		return
	}
	status := ep.SuccessStatus()
	if raw := r.Header.Get("X-Mock-Status"); raw != "" {
		n, err := strconv.Atoi(raw)
		if err != nil || ep.ResponseFor(n) == nil {
			writeJSON(w, http.StatusBadRequest, errBody("status "+raw+" is not documented for this operation"))
			return
		}
		status = n
	}
	resp := ep.ResponseFor(status)
	if resp == nil || resp.Schema == nil {
		w.WriteHeader(status)
		return
	}
	writeJSON(w, status, Example(resp.Schema))
}

func (h *Handler) serveScenario(w http.ResponseWriter, r *http.Request, name string) {
	for _, sc := range h.scenarios {
		if sc.Name != name {
			continue
		}
		if sc.Method != "" && !strings.EqualFold(sc.Method, r.Method) {
			continue
		}
		if _, ok := domain.MatchPath(sc.Path, r.URL.Path); !ok {
			continue
		}
		for k, v := range sc.Headers {
			w.Header().Set(k, v)
		}
		status := sc.Status
		if status == 0 {
			status = http.StatusOK
		}
		if sc.Body == nil {
			w.WriteHeader(status)
			return
		}
		writeJSON(w, status, sc.Body)
		return
	}
	writeJSON(w, http.StatusNotFound, errBody("unknown scenario "+name))
}

func validateRequest(ep *domain.APIEndpoint, r *http.Request) []string {
	values := domain.PathValues(ep.Path, r.URL.Path)
	query := r.URL.Query()
	var out []string
	for _, p := range ep.Parameters {
		raw, present := "", false
		switch p.Location {
		case domain.LocationPath:
			raw = values[p.Name]
			present = raw != ""
		case domain.LocationQuery:
			if vs, ok := query[p.Name]; ok && len(vs) > 0 {
				raw, present = vs[0], true
			}
		case domain.LocationHeader:
			if vs := r.Header.Values(p.Name); len(vs) > 0 {
				raw, present = vs[0], true
			}
		case domain.LocationCookie:
			if c, err := r.Cookie(p.Name); err == nil {
				raw, present = c.Value, true
			}
		}
		if !present {
			if p.Required {
				out = append(out, "missing required "+string(p.Location)+" parameter "+p.Name)
			}
			continue
		}
		for _, m := range validation.Param(raw, p.Schema) {
			out = append(out, p.Name+": "+m)
		}
	}
	if ep.RequestBody != nil {
		data, err := io.ReadAll(io.LimitReader(r.Body, maxBodyBytes+1))
		switch {
		case err != nil:
			out = append(out, "request body could not be read")
		case len(data) > maxBodyBytes:
			out = append(out, "request body is too large")
		case len(bytes.TrimSpace(data)) == 0:
			if ep.RequestBody.Required {
				out = append(out, "request body is required")
			}
		default:
			var v any
			if err := json.Unmarshal(data, &v); err != nil {
				out = append(out, "request body is not valid JSON")
			} else {
				for _, m := range validation.Value(v, ep.RequestBody.Schema) {
					out = append(out, "body: "+m)
				}
			}
		}
	}
	return out
}

// StartLocal serves h on a random loopback port. It returns the base URL and a
// function that stops the server.
func StartLocal(h http.Handler) (baseURL string, stop func(), err error) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return "", nil, err
	}
	srv := &http.Server{Handler: h, ReadHeaderTimeout: 5 * time.Second}
	go func() { _ = srv.Serve(ln) }()
	return "http://" + ln.Addr().String(), func() { _ = srv.Close() }, nil
}
