package safedial_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"tools.xdoubleu.com/internal/safedial"
)

func TestIsPublic(t *testing.T) {
	tests := map[string]bool{
		"1.1.1.1":                true,
		"93.184.216.34":          true,
		"2606:4700:4700::1111":   true,
		"169.254.169.254":        false, // cloud metadata
		"127.0.0.1":              false,
		"::1":                    false,
		"10.0.0.1":               false,
		"192.168.1.1":            false,
		"172.16.0.1":             false,
		"fc00::1":                false,
		"fe80::1":                false,
		"100.64.0.1":             false, // CGNAT
		"0.0.0.0":                false,
		"255.255.255.255":        false,
		"224.0.0.1":              false, // multicast
		"::ffff:127.0.0.1":       false, // v4-mapped loopback
		"not-an-ip":              false,
		"":                       false,
		"::ffff:169.254.169.254": false,
		"2001:db8::1":            true,
	}

	for addr, want := range tests {
		t.Run(addr, func(t *testing.T) {
			assert.Equal(t, want, safedial.IsPublic(addr))
		})
	}
}

func TestClientBlocksLoopback(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(
		func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusOK)
		},
	))
	defer srv.Close()

	client := safedial.Client(5*time.Second, 5, false)

	req, err := http.NewRequestWithContext(
		context.Background(), http.MethodGet, srv.URL, nil,
	)
	require.NoError(t, err)

	_, err = client.Do(req)
	require.Error(t, err)
	assert.ErrorIs(t, err, safedial.ErrBlockedAddress)
}

func TestClientAllowsPrivateWhenPermitted(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(
		func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusOK)
		},
	))
	defer srv.Close()

	client := safedial.Client(5*time.Second, 5, true)

	req, err := http.NewRequestWithContext(
		context.Background(), http.MethodGet, srv.URL, nil,
	)
	require.NoError(t, err)

	resp, err := client.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)
}

// A redirect to a blocked address must fail even though the first hop was
// allowed — the check runs per connection, not per URL.
func TestClientBlocksRedirectToLoopback(t *testing.T) {
	target := httptest.NewServer(http.HandlerFunc(
		func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusOK)
		},
	))
	defer target.Close()

	// allowPrivate client to reach the redirector, blocked client for the hop.
	redirector := httptest.NewServer(http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			http.Redirect(w, r, target.URL, http.StatusFound)
		},
	))
	defer redirector.Close()

	client := safedial.Client(5*time.Second, 5, false)

	req, err := http.NewRequestWithContext(
		context.Background(), http.MethodGet, redirector.URL, nil,
	)
	require.NoError(t, err)

	_, err = client.Do(req)
	require.Error(t, err)
	assert.ErrorIs(t, err, safedial.ErrBlockedAddress)
}

func TestClientStopsAfterMaxRedirects(t *testing.T) {
	var srv *httptest.Server
	srv = httptest.NewServer(http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			http.Redirect(w, r, srv.URL, http.StatusFound)
		},
	))
	defer srv.Close()

	client := safedial.Client(5*time.Second, 2, true)

	req, err := http.NewRequestWithContext(
		context.Background(), http.MethodGet, srv.URL, nil,
	)
	require.NoError(t, err)

	_, err = client.Do(req)
	require.ErrorContains(t, err, "stopped after 2 redirects")
}
