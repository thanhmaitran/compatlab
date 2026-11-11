// Package security restricts where the test engine may send requests, so
// CompatLab cannot be used to scan arbitrary systems (see ADR-0004).
package security

import (
	"errors"
	"fmt"
	"net"
	"net/url"
	"strings"
	"syscall"
)

// ErrTargetNotAllowed is returned for targets outside the allowlist or in a
// blocked address range.
var ErrTargetNotAllowed = errors.New("target not allowed")

// TargetPolicy is an explicit allowlist. Entries are either "host" (any port)
// or "host:port". Nothing is allowed by default, including localhost.
type TargetPolicy struct {
	AllowedHosts []string
}

// Check validates a target URL: http or https, no embedded credentials, host
// on the allowlist, and never a link-local, multicast or unspecified address
// (this blocks cloud metadata endpoints even if listed).
func (p TargetPolicy) Check(rawURL string) error {
	u, err := url.Parse(rawURL)
	if err != nil {
		return fmt.Errorf("%w: invalid URL: %v", ErrTargetNotAllowed, err)
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return fmt.Errorf("%w: scheme %q is not allowed", ErrTargetNotAllowed, u.Scheme)
	}
	if u.User != nil {
		return fmt.Errorf("%w: credentials in the URL are not allowed", ErrTargetNotAllowed)
	}
	host := u.Hostname()
	if host == "" {
		return fmt.Errorf("%w: missing host", ErrTargetNotAllowed)
	}
	if ip := net.ParseIP(host); ip != nil && blockedIP(ip) {
		return fmt.Errorf("%w: address %s is in a blocked range", ErrTargetNotAllowed, host)
	}
	for _, allowed := range p.AllowedHosts {
		if strings.EqualFold(allowed, host) || strings.EqualFold(allowed, u.Host) {
			return nil
		}
	}
	return fmt.Errorf("%w: host %q is not on the allowlist", ErrTargetNotAllowed, u.Host)
}

func blockedIP(ip net.IP) bool {
	return ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() || ip.IsMulticast() || ip.IsUnspecified()
}

// SafeControl is a net.Dialer Control function that refuses connections to
// blocked addresses at connect time, which also defeats DNS rebinding.
func SafeControl(network, address string, _ syscall.RawConn) error {
	host, _, err := net.SplitHostPort(address)
	if err != nil {
		return err
	}
	if ip := net.ParseIP(host); ip != nil && blockedIP(ip) {
		return fmt.Errorf("%w: connection to %s is blocked", ErrTargetNotAllowed, host)
	}
	return nil
}
