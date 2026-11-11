package security_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/thanhmaitran/compatlab/apps/api/routes"
	"github.com/thanhmaitran/compatlab/internal/auth"
	"github.com/thanhmaitran/compatlab/internal/persistence"
	"github.com/thanhmaitran/compatlab/internal/security"
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

// parseWithTimeout fails the test if parsing hangs, and turns a panic into a
// test failure: untrusted input must never crash or stall the parser.
func parseWithTimeout(t *testing.T, name string, p *specification.Parser, data []byte) error {
	t.Helper()
	type result struct {
		err      error
		panicked any
	}
	done := make(chan result, 1)
	go func() {
		var r result
		defer func() {
			r.panicked = recover()
			done <- r
		}()
		_, r.err = p.Parse(data)
	}()
	select {
	case r := <-done:
		if r.panicked != nil {
			t.Fatalf("%s: parser panicked: %v", name, r.panicked)
		}
		return r.err
	case <-time.After(10 * time.Second):
		t.Fatalf("%s: parser did not finish within 10s", name)
		return nil
	}
}

func TestParserRejectsHostileDocuments(t *testing.T) {
	header := `{"openapi":"3.0.3","info":{"title":"t","version":"1"},"paths":{},`
	deep := header + `"x":` + strings.Repeat(`{"a":`, 400) + `1` + strings.Repeat("}", 400) + `}`

	var laughs strings.Builder
	laughs.WriteString("openapi: 3.0.3\ninfo: {title: t, version: '1'}\npaths: {}\n")
	laughs.WriteString("a: &a [lol, lol, lol, lol, lol, lol, lol, lol, lol]\n")
	prev := "a"
	for _, name := range []string{"b", "c", "d", "e", "f", "g"} {
		laughs.WriteString(name + ": &" + name + " [" + strings.Repeat("*"+prev+", ", 9) + "]\n")
		prev = name
	}

	selfRef := `{"openapi":"3.0.3","info":{"title":"t","version":"1"},"components":{"schemas":{"A":{"$ref":"#/components/schemas/A"}}}}`
	pingPong := `{"openapi":"3.0.3","info":{"title":"t","version":"1"},"components":{"schemas":{"A":{"$ref":"#/components/schemas/B"},"B":{"$ref":"#/components/schemas/A"}}}}`
	external := func(ref string) string {
		return `{"openapi":"3.0.3","info":{"title":"t","version":"1"},"components":{"schemas":{"A":{"$ref":"` + ref + `"}}}}`
	}

	tests := []struct {
		name string
		data []byte
		want error // nil means "any error"
	}{
		{"deeply nested document", []byte(deep), specification.ErrValidation},
		{"alias bomb (billion laughs)", []byte(laughs.String()), nil},
		{"self-referencing $ref", []byte(selfRef), specification.ErrReference},
		{"ping-pong $ref chain", []byte(pingPong), specification.ErrReference},
		{"file reference", []byte(external("file:///etc/passwd")), specification.ErrReference},
		{"http reference", []byte(external("http://169.254.169.254/latest/meta-data")), specification.ErrReference},
		{"relative file reference", []byte(external("other.yaml#/components/schemas/X")), specification.ErrReference},
	}
	for _, tc := range tests {
		err := parseWithTimeout(t, tc.name, specification.NewParser(), tc.data)
		switch {
		case err == nil:
			t.Errorf("%s: expected an error", tc.name)
		case tc.want != nil && !errors.Is(err, tc.want):
			t.Errorf("%s: error = %v, want %v", tc.name, err, tc.want)
		}
	}

	if err := parseWithTimeout(t, "oversized", &specification.Parser{MaxBytes: 1024}, bytes.Repeat([]byte("a"), 2048)); !errors.Is(err, specification.ErrTooLarge) {
		t.Errorf("oversized document: %v", err)
	}
}

func TestParserNeverPanicsOnMalformedInput(t *testing.T) {
	valid := read(t, "testdata/valid/payment-api.yaml")
	inputs := [][]byte{
		{}, []byte("\x00\x00\x00"), []byte("\xff\xfe\xfd"), []byte("openapi: 3.0.3\x00\xff"),
		[]byte("{"), []byte("[1,2,3]"), []byte("null"), []byte("42"), []byte(`{"openapi":3}`),
		[]byte(`{"openapi":"3.0.3","info":[],"paths":[]}`),
		[]byte(`{"openapi":"3.0.3","info":{"title":"t","version":"1"},"paths":{"/a":{"get":[]}}}`),
		[]byte(`{"openapi":"3.0.3","info":{"title":"t","version":"1"},"paths":{"/a":{"get":{"parameters":[{"in":"nowhere","name":"x"}]}}}}`),
		[]byte(`{"openapi":"3.0.3","info":{"title":"t","version":"1"},"paths":{"/a":{"get":{"responses":{"200":"oops"}}}}}`),
		[]byte(`{"openapi":"3.0.3","info":{"title":"t","version":"1"},"components":{"schemas":{"A":{"properties":{"x":5}}}}}`),
	}
	for i := 0; i < len(valid); i += 37 { // every truncation point sampled
		inputs = append(inputs, valid[:i])
	}
	for i, in := range inputs {
		_ = parseWithTimeout(t, "input "+string(rune('A'+i%26)), specification.NewParser(), in)
	}
}

func TestTargetPolicyBlocksSSRFVariants(t *testing.T) {
	none := security.TargetPolicy{}
	listed := security.TargetPolicy{AllowedHosts: []string{"example.com"}}
	blockedEverywhere := []string{
		"http://localhost:8080/", "http://[::1]:80/", "http://0.0.0.0/", "http://2130706433/", "http://127.1/",
		"http://metadata.google.internal/", "http://169.254.169.254/latest/meta-data/", "file:///etc/passwd", "gopher://example.com/",
		"http://example.com@127.0.0.1/", "http://example.com.evil.test/", "http://evil.test/example.com",
	}
	for _, u := range blockedEverywhere {
		if err := none.Check(u); !errors.Is(err, security.ErrTargetNotAllowed) {
			t.Errorf("empty policy must deny %s, got %v", u, err)
		}
		if err := listed.Check(u); !errors.Is(err, security.ErrTargetNotAllowed) {
			t.Errorf("policy allowing only example.com must deny %s, got %v", u, err)
		}
	}
	if err := listed.Check("https://example.com/api"); err != nil {
		t.Errorf("an allowlisted host must be allowed: %v", err)
	}
}

type failingStore struct{ *persistence.Memory }

func (failingStore) GetProject(context.Context, string, string) (*persistence.Project, error) {
	return nil, errors.New("pq: connection to host=db.internal user=compatlab password=SECRET-VALUE failed")
}

func newRouter(t *testing.T, store persistence.Store, cfg routes.Config) http.Handler {
	t.Helper()
	tokens, err := auth.NewTokenIssuer("0123456789abcdef0123456789abcdef", time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	return routes.New(cfg, service.New(store, tokens, nil))
}

func call(h http.Handler, token, method, path string, body any) (int, string) {
	var rdr *bytes.Reader
	if body != nil {
		data, _ := json.Marshal(body)
		rdr = bytes.NewReader(data)
	} else {
		rdr = bytes.NewReader(nil)
	}
	req := httptest.NewRequest(method, path, rdr)
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec.Code, rec.Body.String()
}

func register(t *testing.T, h http.Handler, email string) string {
	t.Helper()
	code, body := call(h, "", "POST", "/api/v1/auth/register", map[string]any{"organization": "Acme", "email": email, "password": "correct-horse-1"})
	if code != 201 {
		t.Fatalf("register: %d %s", code, body)
	}
	var m map[string]string
	_ = json.Unmarshal([]byte(body), &m)
	return m["token"]
}

func TestInternalErrorsDoNotLeakDetails(t *testing.T) {
	h := newRouter(t, failingStore{persistence.NewMemory()}, routes.Config{})
	token := register(t, h, "admin@acme.io")
	code, body := call(h, token, "GET", "/api/v1/projects/anything", nil)
	if code != 500 {
		t.Fatalf("status %d, want 500", code)
	}
	for _, secret := range []string{"SECRET-VALUE", "db.internal", "password"} {
		if strings.Contains(body, secret) {
			t.Errorf("the response leaks %q: %s", secret, body)
		}
	}
}

func TestInjectionPayloadsAreHandledSafely(t *testing.T) {
	h := newRouter(t, persistence.NewMemory(), routes.Config{})
	token := register(t, h, "admin@acme.io")

	sqli := "'; DROP TABLE users; --"
	if code, body := call(h, token, "POST", "/api/v1/projects", map[string]any{"name": sqli}); code != 201 || !strings.Contains(body, "DROP TABLE") {
		t.Errorf("a hostile name must be stored verbatim as data: %d %s", code, body)
	}
	code, body := call(h, token, "POST", "/api/v1/projects", map[string]any{"name": "<script>alert(1)</script>"})
	if code != 201 || strings.Contains(body, "<script>") || !strings.Contains(body, `\u003cscript`) {
		t.Errorf("HTML in JSON output must be escaped: %d %s", code, body)
	}
	for _, email := range []string{"' OR '1'='1", "admin@acme.io' --", "x@y.z\r\nBcc: evil@example.com"} {
		if code, _ := call(h, "", "POST", "/api/v1/auth/login", map[string]any{"email": email, "password": "' OR '1'='1"}); code != 401 {
			t.Errorf("login with %q: status %d, want 401", email, code)
		}
	}
	if code, _ := call(h, token, "GET", "/api/v1/projects/..%2F..%2Fetc%2Fpasswd", nil); code != 404 {
		t.Errorf("path traversal in an ID must be a 404, got %d", code)
	}
}

func TestForgedAndMalformedTokensAreRejected(t *testing.T) {
	h := newRouter(t, persistence.NewMemory(), routes.Config{})
	other, _ := auth.NewTokenIssuer("ffffffffffffffffffffffffffffffff", time.Hour)
	forged, _ := other.Issue(auth.Claims{Sub: "u", Org: "o", Role: "admin"})
	for name, token := range map[string]string{
		"other secret": forged, "none algorithm": "eyJhbGciOiJub25lIiwidHlwIjoiSldUIn0.eyJzdWIiOiJ1Iiwib3JnIjoibyIsInJvbGUiOiJhZG1pbiIsImV4cCI6OTk5OTk5OTk5OX0.",
		"garbage": "a.b.c", "empty bearer": "",
	} {
		if code, _ := call(h, token, "GET", "/api/v1/projects", nil); code != 401 {
			t.Errorf("%s: status %d, want 401", name, code)
		}
	}
}

func TestOversizedUploadsAreRejected(t *testing.T) {
	h := newRouter(t, persistence.NewMemory(), routes.Config{MaxBodyBytes: 4096})
	token := register(t, h, "admin@acme.io")
	code, body := call(h, token, "POST", "/api/v1/projects", map[string]any{"name": "API"})
	if code != 201 {
		t.Fatal(body)
	}
	var proj map[string]any
	_ = json.Unmarshal([]byte(body), &proj)
	huge := strings.Repeat("x", 100000)
	if code, _ := call(h, token, "POST", "/api/v1/projects/"+proj["id"].(string)+"/specifications", map[string]any{"version_label": "v1", "content": huge}); code != 413 {
		t.Errorf("oversized upload: status %d, want 413", code)
	}
}

// Static guard: SQL text in the PostgreSQL store may only be built with
// fmt.Sprintf to inject a table name taken from the fixed docTables map.
// Everything else must use $n placeholders.
func TestPostgresStoreUsesParameterizedQueries(t *testing.T) {
	src := string(read(t, "internal/persistence/postgres.go"))
	sqlSprintf := regexp.MustCompile("fmt\\.Sprintf\\(\x60(?i:SELECT|INSERT|UPDATE|DELETE)[^\x60]*\x60([^)]*)\\)")
	matches := sqlSprintf.FindAllStringSubmatch(src, -1)
	if len(matches) == 0 {
		t.Fatal("the guard found no dynamic SQL; update the test if the store changed")
	}
	for _, m := range matches {
		if strings.TrimSpace(m[1]) != ", table" {
			t.Errorf("dynamic SQL may only interpolate the table name, found arguments %q", m[1])
		}
	}
	if regexp.MustCompile(`Sprintf\("(?i:SELECT|INSERT|UPDATE|DELETE)`).MatchString(src) {
		t.Error("SQL must not be built from double-quoted format strings")
	}
	if regexp.MustCompile("\x60(?i:SELECT|INSERT|UPDATE|DELETE)[^\x60]*\x60\\s*\\+").MatchString(src) {
		t.Error("SQL must not be built by string concatenation")
	}
}
