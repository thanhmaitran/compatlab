// Package execution sends requests to allowed targets with strict limits:
// an explicit host allowlist, per-request timeouts, a response size cap, a
// per-run request cap, and no connections to link-local addresses.
package execution

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"time"

	"github.com/thanhmaitran/compatlab/internal/security"
)

// ErrLimit is returned when a response exceeds the configured size.
var ErrLimit = errors.New("limit exceeded")

// Config bounds request execution. Zero values select safe defaults.
type Config struct {
	Policy           security.TargetPolicy
	Timeout          time.Duration // per request; default 10s
	MaxResponseBytes int64         // default 1 MiB
	MaxRequests      int           // per run; default 1000
}

func (c Config) withDefaults() Config {
	if c.Timeout <= 0 {
		c.Timeout = 10 * time.Second
	}
	if c.MaxResponseBytes <= 0 {
		c.MaxResponseBytes = 1 << 20
	}
	if c.MaxRequests <= 0 {
		c.MaxRequests = 1000
	}
	return c
}

// NewClient returns an HTTP client that ignores proxy environment variables,
// blocks connections to link-local addresses and re-checks every redirect
// against the allowlist.
func NewClient(cfg Config) *http.Client {
	cfg = cfg.withDefaults()
	dialer := &net.Dialer{Timeout: 5 * time.Second, Control: security.SafeControl}
	return &http.Client{
		Timeout:   cfg.Timeout,
		Transport: &http.Transport{DialContext: dialer.DialContext, Proxy: nil, MaxIdleConnsPerHost: 4},
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= 5 {
				return errors.New("too many redirects")
			}
			return cfg.Policy.Check(req.URL.String())
		},
	}
}

// Response is a fully read, size-limited HTTP response.
type Response struct {
	Status int
	Header http.Header
	Body   []byte
}

// Do performs one request after checking the target against the policy.
func Do(ctx context.Context, client *http.Client, cfg Config, method, rawURL string, headers map[string]string, body []byte) (*Response, error) {
	cfg = cfg.withDefaults()
	if err := cfg.Policy.Check(rawURL); err != nil {
		return nil, err
	}
	ctx, cancel := context.WithTimeout(ctx, cfg.Timeout)
	defer cancel()
	var rdr io.Reader
	if body != nil {
		rdr = bytes.NewReader(body)
	}
	req, err := http.NewRequestWithContext(ctx, method, rawURL, rdr)
	if err != nil {
		return nil, err
	}
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(io.LimitReader(resp.Body, cfg.MaxResponseBytes+1))
	if err != nil {
		return nil, err
	}
	if int64(len(data)) > cfg.MaxResponseBytes {
		return nil, fmt.Errorf("%w: response exceeds %d bytes", ErrLimit, cfg.MaxResponseBytes)
	}
	return &Response{Status: resp.StatusCode, Header: resp.Header, Body: data}, nil
}
