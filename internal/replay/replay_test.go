package replay

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/thanhmaitran/compatlab/internal/domain"
	"github.com/thanhmaitran/compatlab/internal/execution"
	"github.com/thanhmaitran/compatlab/internal/mocking"
	"github.com/thanhmaitran/compatlab/internal/security"
	"github.com/thanhmaitran/compatlab/internal/specification"
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

func record(t *testing.T) Recording {
	t.Helper()
	rec := NewRecorder(mocking.NewHandler(load(t, "compatible/users-v1.yaml"), nil))
	send := func(method, target, body string, headers map[string]string) {
		req := httptest.NewRequest(method, target, strings.NewReader(body))
		for k, v := range headers {
			req.Header.Set(k, v)
		}
		rec.ServeHTTP(httptest.NewRecorder(), req)
	}
	send("POST", "/users", `{"email":"alice@example.com","password":"hunter2","name":"Alice"}`,
		map[string]string{"Content-Type": "application/json", "Authorization": "Bearer super-secret-token"})
	send("GET", "/users/u-1?token=abc123", "", nil)
	return rec.Recording()
}

func allowing(t *testing.T, base string) execution.Config {
	t.Helper()
	u, err := url.Parse(base)
	if err != nil {
		t.Fatal(err)
	}
	return execution.Config{Policy: security.TargetPolicy{AllowedHosts: []string{u.Host}}}
}

func TestRecordingMasksSecretsAndPersonalData(t *testing.T) {
	data, err := json.Marshal(record(t))
	if err != nil {
		t.Fatal(err)
	}
	text := string(data)
	for _, secret := range []string{"super-secret-token", "hunter2", "alice@example.com", "abc123", "user@example.com"} {
		if strings.Contains(text, secret) {
			t.Errorf("recording leaks %q:\n%s", secret, text)
		}
	}
	if !strings.Contains(text, Masked) || !strings.Contains(text, "Alice") {
		t.Errorf("masking should replace secrets only:\n%s", text)
	}
}

func TestRecordingCapturesStatusAndOrder(t *testing.T) {
	rec := record(t)
	if len(rec.Entries) != 2 || rec.Entries[0].Method != "POST" || rec.Entries[0].Status != 201 || rec.Entries[1].Status != 200 {
		t.Errorf("unexpected entries: %+v", rec.Entries)
	}
	if rec.Entries[0].RequestHeaders["Authorization"] != Masked {
		t.Error("credentials must be masked")
	}
}

func TestReplayIsConsistentAgainstTheSameProvider(t *testing.T) {
	base, stop, err := mocking.StartLocal(mocking.NewHandler(load(t, "compatible/users-v1.yaml"), nil))
	if err != nil {
		t.Fatal(err)
	}
	defer stop()
	results, err := Replay(context.Background(), record(t), base, Options{Config: allowing(t, base)})
	if err != nil {
		t.Fatal(err)
	}
	for _, r := range results {
		if !r.Passed {
			t.Errorf("replay %d %s %s failed: %s", r.Index, r.Method, r.Path, r.Message)
		}
	}
}

func TestReplayDetectsAChangedProvider(t *testing.T) {
	base, stop, err := mocking.StartLocal(mocking.NewHandler(load(t, "breaking/users-v2-breaking.yaml"), nil))
	if err != nil {
		t.Fatal(err)
	}
	defer stop()
	results, err := Replay(context.Background(), record(t), base, Options{Config: allowing(t, base)})
	if err != nil {
		t.Fatal(err)
	}
	failed := 0
	for _, r := range results {
		if !r.Passed {
			failed++
		}
	}
	if failed == 0 {
		t.Error("replay against the breaking provider must report a difference")
	}
}

func TestReplayRefusesTargetsOutsideTheAllowlist(t *testing.T) {
	_, err := Replay(context.Background(), record(t), "http://127.0.0.1:9", Options{})
	if !errors.Is(err, security.ErrTargetNotAllowed) {
		t.Errorf("expected ErrTargetNotAllowed, got %v", err)
	}
}

func TestRecorderDoesNotBreakTheHandler(t *testing.T) {
	h := NewRecorder(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTeapot)
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest("GET", "/x", nil))
	if rec.Code != http.StatusTeapot || rec.Body.String() != `{"ok":true}` {
		t.Errorf("response was altered: %d %s", rec.Code, rec.Body.String())
	}
}
