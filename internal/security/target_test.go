package security

import (
	"errors"
	"testing"
)

func TestCheck(t *testing.T) {
	p := TargetPolicy{AllowedHosts: []string{"api.example.com", "127.0.0.1:8081", "169.254.169.254"}}
	allowed := []string{"https://api.example.com/v1", "http://API.example.com:8080/x", "http://127.0.0.1:8081/users"}
	for _, u := range allowed {
		if err := p.Check(u); err != nil {
			t.Errorf("%s should be allowed: %v", u, err)
		}
	}
	denied := []string{
		"https://other.example.com/",         // not on the allowlist
		"http://127.0.0.1:9999/",             // allowlisted with a different port
		"ftp://api.example.com/",             // scheme
		"http://user:pw@api.example.com/",    // credentials
		"http://169.254.169.254/latest/meta", // blocked even when listed
		"http://0.0.0.0:80/",                 // unspecified
		"http:///nohost",                     // missing host
		"://bad",                             // invalid URL
	}
	for _, u := range denied {
		if err := p.Check(u); !errors.Is(err, ErrTargetNotAllowed) {
			t.Errorf("%s should be denied, got %v", u, err)
		}
	}
}

func TestEmptyPolicyAllowsNothing(t *testing.T) {
	if err := (TargetPolicy{}).Check("http://127.0.0.1:8080/"); !errors.Is(err, ErrTargetNotAllowed) {
		t.Errorf("empty policy must deny everything, got %v", err)
	}
}

func TestSafeControl(t *testing.T) {
	if err := SafeControl("tcp", "169.254.169.254:80", nil); !errors.Is(err, ErrTargetNotAllowed) {
		t.Errorf("link-local must be blocked, got %v", err)
	}
	if err := SafeControl("tcp", "127.0.0.1:80", nil); err != nil {
		t.Errorf("loopback is the allowlist's decision, not the dialer's: %v", err)
	}
	if err := SafeControl("tcp", "no-port", nil); err == nil {
		t.Error("malformed address must be rejected")
	}
}
