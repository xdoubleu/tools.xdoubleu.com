package bmc

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"tools.xdoubleu.com/internal/logging"
)

func withHTTPScheme(t *testing.T) {
	t.Helper()
	prev := scheme
	scheme = "http"
	t.Cleanup(func() { scheme = prev })
}

func hostOf(t *testing.T, srv *httptest.Server) string {
	t.Helper()
	return strings.TrimPrefix(srv.URL, "http://")
}

func TestRateLimitedError_Message(t *testing.T) {
	err := &RateLimitedError{RetryAfter: 21 * time.Second}
	assert.Contains(t, err.Error(), "21s")
}

func TestFetchStatic_NotConfigured(t *testing.T) {
	c := New(logging.NewNopLogger(), "example.test", "")
	_, err := c.FetchStatic(
		context.Background(),
		StaticOptions{ETag: "", LastModified: ""},
	)
	assert.ErrorIs(t, err, ErrNotConfigured)
}

func TestFetchStatic_SendsPartnerKeyAndPath(t *testing.T) {
	withHTTPScheme(t)
	var gotKey, gotPath string
	srv := httptest.NewServer(http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			gotKey = r.Header.Get("bmc-partner-key")
			gotPath = r.URL.Path
			w.Header().Set("ETag", `"abc"`)
			w.Header().Set("Last-Modified", "Wed, 20 Aug 2026 00:00:00 GMT")
			_, _ = w.Write([]byte("PK\x03\x04rest-of-zip"))
		}))
	defer srv.Close()

	c := New(logging.NewNopLogger(), hostOf(t, srv), "secret-key")
	res, err := c.FetchStatic(
		context.Background(),
		StaticOptions{ETag: "", LastModified: ""},
	)
	require.NoError(t, err)

	assert.Equal(t, "secret-key", gotKey)
	assert.Equal(t, "/api/gtfs/feed/nmbssncb/static", gotPath)
	assert.Equal(t, `"abc"`, res.ETag)
	assert.Equal(t, "Wed, 20 Aug 2026 00:00:00 GMT", res.LastModified)
	assert.True(t, strings.HasPrefix(string(res.Body), "PK\x03\x04"))
	assert.False(t, res.NotModified)
}

func TestFetchStatic_ConditionalGet304(t *testing.T) {
	withHTTPScheme(t)
	srv := httptest.NewServer(http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, `"abc"`, r.Header.Get("If-None-Match"))
			assert.Equal(t, "last-mod", r.Header.Get("If-Modified-Since"))
			w.WriteHeader(http.StatusNotModified)
		}))
	defer srv.Close()

	c := New(logging.NewNopLogger(), hostOf(t, srv), "k")
	res, err := c.FetchStatic(context.Background(), StaticOptions{
		ETag: `"abc"`, LastModified: "last-mod",
	})
	require.NoError(t, err)
	assert.True(t, res.NotModified)
	assert.Empty(t, res.Body)
}

func TestFetchStatic_RateLimited(t *testing.T) {
	withHTTPScheme(t)
	srv := httptest.NewServer(http.HandlerFunc(
		func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Retry-After", "21")
			w.WriteHeader(http.StatusTooManyRequests)
		}))
	defer srv.Close()

	c := New(logging.NewNopLogger(), hostOf(t, srv), "k")
	_, err := c.FetchStatic(
		context.Background(),
		StaticOptions{ETag: "", LastModified: ""},
	)

	var rl *RateLimitedError
	require.True(t, errors.As(err, &rl))
	assert.Equal(t, 21*time.Second, rl.RetryAfter)
}
