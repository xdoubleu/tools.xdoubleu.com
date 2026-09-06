package bmc

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"tools.xdoubleu.com/internal/logging"
)

func TestFetchRealtime_NotConfigured(t *testing.T) {
	c := New(logging.NewNopLogger(), "example.test", "")
	_, err := c.FetchRealtime(context.Background(), FeedTripUpdate)
	assert.ErrorIs(t, err, ErrNotConfigured)
}

func TestFetchRealtime_RequestsProtobufAndSendsKey(t *testing.T) {
	withHTTPScheme(t)
	var gotKey, gotPath, gotQuery string
	srv := httptest.NewServer(http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			gotKey = r.Header.Get("bmc-partner-key")
			gotPath = r.URL.Path
			gotQuery = r.URL.RawQuery
			w.Header().Set("Content-Type", "application/protobuf")
			_, _ = w.Write([]byte("proto-bytes"))
		}))
	defer srv.Close()

	c := New(logging.NewNopLogger(), hostOf(t, srv), "secret-key")
	res, err := c.FetchRealtime(context.Background(), FeedTripUpdate)
	require.NoError(t, err)

	assert.Equal(t, "secret-key", gotKey)
	assert.Equal(t, "/api/gtfs/feed/nmbssncb/rt/trip-update", gotPath)
	assert.Equal(t, "format=protobuf", gotQuery)
	assert.Equal(t, "proto-bytes", string(res.Body))
}

// TestFetchRealtime_RejectsNonProtobuf guards against the gateway's
// documented JSON-by-default fallback (issue #1389) silently corrupting a
// naive delay parse — a non-protobuf Content-Type must fail loudly.
func TestFetchRealtime_RejectsNonProtobuf(t *testing.T) {
	withHTTPScheme(t)
	srv := httptest.NewServer(http.HandlerFunc(
		func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "application/json; charset=utf-8")
			_, _ = w.Write([]byte(`{"low":1,"high":0}`))
		}))
	defer srv.Close()

	c := New(logging.NewNopLogger(), hostOf(t, srv), "k")
	_, err := c.FetchRealtime(context.Background(), FeedTripUpdate)
	require.ErrorContains(t, err, "expected protobuf")
}

func TestFetchRealtime_RateLimited(t *testing.T) {
	withHTTPScheme(t)
	srv := httptest.NewServer(http.HandlerFunc(
		func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Retry-After", "21")
			w.WriteHeader(http.StatusTooManyRequests)
		}))
	defer srv.Close()

	c := New(logging.NewNopLogger(), hostOf(t, srv), "k")
	_, err := c.FetchRealtime(context.Background(), FeedAlert)

	var rl *RateLimitedError
	require.True(t, errors.As(err, &rl))
	assert.Equal(t, 21, int(rl.RetryAfter.Seconds()))
}

func TestFetchRealtime_UpstreamError(t *testing.T) {
	withHTTPScheme(t)
	srv := httptest.NewServer(http.HandlerFunc(
		func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusServiceUnavailable)
		}))
	defer srv.Close()

	c := New(logging.NewNopLogger(), hostOf(t, srv), "k")
	_, err := c.FetchRealtime(context.Background(), FeedAlert)

	var up *UpstreamError
	require.True(t, errors.As(err, &up))
	assert.Equal(t, http.StatusServiceUnavailable, up.StatusCode)
}
