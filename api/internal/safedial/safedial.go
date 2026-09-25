// Package safedial builds http.Clients that refuse non-public IPs, blocking
// SSRF against metadata, loopback and RFC1918. The check runs in
// [net.Dialer.Control] on every resolved IP, covering redirects and DNS
// rebinding.
package safedial

import (
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/netip"
	"syscall"
	"time"
)

// ErrBlockedAddress is returned when a connection resolves to a non-public IP.
var ErrBlockedAddress = errors.New("connection to non-public address blocked")

// cgnat is RFC 6598 shared address space — not covered by netip's IsPrivate.
var cgnat = netip.MustParsePrefix("100.64.0.0/10") //nolint:gochecknoglobals //const

// Client blocks non-public IPs and stops after maxRedirects hops. allowPrivate
// disables the check; pass cfg.Env != config.ProdEnv (tests use loopback).
func Client(timeout time.Duration, maxRedirects int, allowPrivate bool) *http.Client {
	dialer := newDialer(timeout)
	if !allowPrivate {
		dialer.Control = control
	}

	transport, ok := http.DefaultTransport.(*http.Transport)
	if !ok { // unreachable with the stdlib default, but don't assume
		transport = &http.Transport{}
	}
	transport = transport.Clone()
	transport.DialContext = dialer.DialContext

	return &http.Client{
		Timeout:   timeout,
		Transport: transport,
		CheckRedirect: func(_ *http.Request, via []*http.Request) error {
			if len(via) >= maxRedirects {
				return fmt.Errorf("stopped after %d redirects", maxRedirects)
			}
			return nil
		},
	}
}

// newDialer is split out so its KeepAlive is unit-testable.
func newDialer(timeout time.Duration) *net.Dialer {
	return &net.Dialer{ //nolint:exhaustruct //defaults are fine
		Timeout:   timeout,
		KeepAlive: 30 * time.Second, //nolint:mnd //net/http's own default
	}
}

// control rejects the dial when address resolves to a non-public IP.
func control(_, address string, _ syscall.RawConn) error {
	host, _, err := net.SplitHostPort(address)
	if err != nil {
		return fmt.Errorf("%w: %s", ErrBlockedAddress, address)
	}
	if !IsPublic(host) {
		return fmt.Errorf("%w: %s", ErrBlockedAddress, host)
	}
	return nil
}

// IsPublic reports whether addr is a routable public IP (not loopback,
// private, link-local, CGNAT, multicast, unspecified or unparseable).
func IsPublic(addr string) bool {
	ip, err := netip.ParseAddr(addr)
	if err != nil {
		return false
	}
	ip = ip.Unmap()

	switch {
	case !ip.IsGlobalUnicast(),
		ip.IsPrivate(),
		ip.IsLoopback(),
		ip.IsLinkLocalUnicast(),
		cgnat.Contains(ip):
		return false
	default:
		return true
	}
}
