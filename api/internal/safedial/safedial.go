// Package safedial builds http.Clients that refuse to connect to non-public
// IP addresses, so a user-supplied URL can't be turned into a request against
// the container's own network (SSRF): cloud metadata (169.254.169.254), api
// itself on loopback, or anything on an internal RFC1918 range.
//
// The check runs in [net.Dialer.Control], i.e. on the resolved IP of every
// connection attempt. That covers redirect hops and DNS rebinding for free —
// no URL re-validation per hop, no hostname allowlist.
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

// ErrBlockedAddress is returned by the dialer when a connection resolves to a
// non-public address.
var ErrBlockedAddress = errors.New("connection to non-public address blocked")

// cgnat is RFC 6598 shared address space — not covered by netip's IsPrivate.
var cgnat = netip.MustParsePrefix("100.64.0.0/10") //nolint:gochecknoglobals //const

// Client returns an http.Client that blocks connections to non-public IPs and
// stops after maxRedirects hops. allowPrivate disables the IP check entirely —
// pass cfg.Env != config.ProdEnv, since tests and local development legitimately
// fetch from httptest servers on loopback.
func Client(timeout time.Duration, maxRedirects int, allowPrivate bool) *http.Client {
	dialer := &net.Dialer{ //nolint:exhaustruct //defaults are fine
		Timeout:   timeout,
		KeepAlive: 30 * time.Second, //nolint:mnd //net/http's own default
	}
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

// IsPublic reports whether addr is a routable public IP. Anything unparseable,
// loopback, private, link-local (incl. 169.254.169.254), CGNAT, multicast or
// unspecified is not.
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
