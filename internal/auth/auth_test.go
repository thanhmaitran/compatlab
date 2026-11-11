package auth

import (
	"encoding/base64"
	"errors"
	"strings"
	"testing"
	"time"
)

const secret = "0123456789abcdef0123456789abcdef"

func TestPasswordHashing(t *testing.T) {
	h, err := HashPassword("correct horse")
	if err != nil {
		t.Fatal(err)
	}
	if h == "correct horse" || !CheckPassword(h, "correct horse") || CheckPassword(h, "wrong") {
		t.Error("hash/verify behaviour wrong")
	}
	if _, err := HashPassword(strings.Repeat("a", MaxPasswordBytes+1)); err == nil {
		t.Error("passwords above the bcrypt limit must be rejected")
	}
}

func TestTokenRoundTrip(t *testing.T) {
	iss, err := NewTokenIssuer(secret, time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	tok, err := iss.Issue(Claims{Sub: "u1", Org: "o1", Role: "admin"})
	if err != nil {
		t.Fatal(err)
	}
	c, err := iss.Verify(tok)
	if err != nil || c.Sub != "u1" || c.Org != "o1" || c.Role != "admin" {
		t.Errorf("Verify = %+v %v", c, err)
	}
}

func TestTokenRejections(t *testing.T) {
	iss, _ := NewTokenIssuer(secret, time.Hour)
	other, _ := NewTokenIssuer("ffffffffffffffffffffffffffffffff", time.Hour)
	tok, _ := iss.Issue(Claims{Sub: "u1", Org: "o1", Role: "member"})
	parts := strings.Split(tok, ".")

	forgedBody := base64.RawURLEncoding.EncodeToString([]byte(`{"sub":"u1","org":"o1","role":"admin","exp":9999999999}`))
	noneHeader := base64.RawURLEncoding.EncodeToString([]byte(`{"alg":"none","typ":"JWT"}`))
	bad := map[string]string{
		"empty":           "",
		"garbage":         "not-a-token",
		"tampered claims": parts[0] + "." + forgedBody + "." + parts[2],
		"alg none":        noneHeader + "." + parts[1] + ".",
		"wrong secret":    mustIssue(t, other),
		"bad signature":   parts[0] + "." + parts[1] + ".AAAA",
	}
	for name, token := range bad {
		if _, err := iss.Verify(token); !errors.Is(err, ErrInvalidToken) {
			t.Errorf("%s: expected ErrInvalidToken, got %v", name, err)
		}
	}
}

func mustIssue(t *testing.T, iss *TokenIssuer) string {
	t.Helper()
	tok, err := iss.Issue(Claims{Sub: "u1", Org: "o1", Role: "member"})
	if err != nil {
		t.Fatal(err)
	}
	return tok
}

func TestExpiredTokenIsRejected(t *testing.T) {
	iss, _ := NewTokenIssuer(secret, time.Hour)
	iss.now = func() time.Time { return time.Now().Add(-2 * time.Hour) }
	tok, _ := iss.Issue(Claims{Sub: "u1", Org: "o1"})
	iss.now = time.Now
	if _, err := iss.Verify(tok); !errors.Is(err, ErrInvalidToken) {
		t.Errorf("expired token must be rejected, got %v", err)
	}
}

func TestIssuerValidation(t *testing.T) {
	if _, err := NewTokenIssuer("short", time.Hour); err == nil {
		t.Error("short secrets must be rejected")
	}
	if _, err := NewTokenIssuer(secret, 0); err == nil {
		t.Error("zero lifetime must be rejected")
	}
}
