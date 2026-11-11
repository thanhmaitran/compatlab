// Package auth provides password hashing (bcrypt) and signed session tokens
// (HS256 JWT implemented with the standard library).
package auth

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"golang.org/x/crypto/bcrypt"
)

// ErrInvalidToken is returned for malformed, tampered or expired tokens.
var ErrInvalidToken = errors.New("invalid token")

// MaxPasswordBytes is bcrypt's input limit.
const MaxPasswordBytes = 72

// HashPassword hashes a password with bcrypt.
func HashPassword(password string) (string, error) {
	if len(password) > MaxPasswordBytes {
		return "", fmt.Errorf("password is longer than %d bytes", MaxPasswordBytes)
	}
	h, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	return string(h), err
}

// CheckPassword reports whether password matches the bcrypt hash.
func CheckPassword(hash, password string) bool {
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(password)) == nil
}

// Claims identify the caller.
type Claims struct {
	Sub  string `json:"sub"`
	Org  string `json:"org"`
	Role string `json:"role"`
	Iat  int64  `json:"iat"`
	Exp  int64  `json:"exp"`
}

// TokenIssuer signs and verifies tokens with an HMAC secret.
type TokenIssuer struct {
	secret []byte
	ttl    time.Duration
	now    func() time.Time
}

// MinSecretBytes is the minimum accepted signing secret length.
const MinSecretBytes = 32

// NewTokenIssuer creates an issuer. The secret must be at least 32 bytes.
func NewTokenIssuer(secret string, ttl time.Duration) (*TokenIssuer, error) {
	if len(secret) < MinSecretBytes {
		return nil, fmt.Errorf("the signing secret must be at least %d bytes", MinSecretBytes)
	}
	if ttl <= 0 {
		return nil, errors.New("token lifetime must be positive")
	}
	return &TokenIssuer{secret: []byte(secret), ttl: ttl, now: time.Now}, nil
}

var enc = base64.RawURLEncoding

const header = `{"alg":"HS256","typ":"JWT"}`

func (t *TokenIssuer) sign(payload string) string {
	mac := hmac.New(sha256.New, t.secret)
	mac.Write([]byte(payload))
	return enc.EncodeToString(mac.Sum(nil))
}

// Issue creates a token for the claims; Iat and Exp are set by the issuer.
func (t *TokenIssuer) Issue(c Claims) (string, error) {
	now := t.now()
	c.Iat, c.Exp = now.Unix(), now.Add(t.ttl).Unix()
	body, err := json.Marshal(c)
	if err != nil {
		return "", err
	}
	signing := enc.EncodeToString([]byte(header)) + "." + enc.EncodeToString(body)
	return signing + "." + t.sign(signing), nil
}

// Verify checks signature, algorithm and expiry and returns the claims.
func (t *TokenIssuer) Verify(token string) (*Claims, error) {
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return nil, ErrInvalidToken
	}
	rawHeader, err := enc.DecodeString(parts[0])
	if err != nil || string(rawHeader) != header {
		return nil, ErrInvalidToken // rejects "none" and any other algorithm
	}
	sig, err := enc.DecodeString(parts[2])
	if err != nil {
		return nil, ErrInvalidToken
	}
	want, _ := enc.DecodeString(t.sign(parts[0] + "." + parts[1]))
	if !hmac.Equal(sig, want) {
		return nil, ErrInvalidToken
	}
	body, err := enc.DecodeString(parts[1])
	if err != nil {
		return nil, ErrInvalidToken
	}
	var c Claims
	if err := json.Unmarshal(body, &c); err != nil || c.Sub == "" || c.Org == "" {
		return nil, ErrInvalidToken
	}
	if t.now().Unix() >= c.Exp {
		return nil, ErrInvalidToken
	}
	return &c, nil
}
