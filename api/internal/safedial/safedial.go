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

// blockedPrefixes are non-public ranges netip's predicates don't cover:
// "this network", CGNAT, IETF protocol assignments, benchmarking, reserved,
// and the NAT64/6to4 prefixes that translate to (possibly private) IPv4.
//
//nolint:gochecknoglobals //const
var blockedPrefixes = []netip.Prefix{
	netip.MustParsePrefix("0.0.0.0/8"),
	netip.MustParsePrefix("100.64.0.0/10"),
	netip.MustParsePrefix("192.0.0.0/24"),
	netip.MustParsePrefix("198.18.0.0/15"),
	netip.MustParsePrefix("240.0.0.0/4"),
	netip.MustParsePrefix("64:ff9b::/96"),
	netip.MustParsePrefix("64:ff9b:1::/48"),
	netip.MustParsePrefix("2002::/16"),
}

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
	// A proxy would dial the target itself, bypassing the check.
	transport.Proxy = nil

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
// private, link-local, multicast, unspecified, a blockedPrefixes range or
// unparseable).
func IsPublic(addr string) bool {
	ip, err := netip.ParseAddr(addr)
	if err != nil {
		return false
	}
	ip = ip.Unmap()

	if !ip.IsGlobalUnicast() || ip.IsPrivate() || ip.IsLoopback() ||
		ip.IsLinkLocalUnicast() {
		return false
	}
	for _, prefix := range blockedPrefixes {
		if prefix.Contains(ip) {
			return false
		}
	}
	return true
}
