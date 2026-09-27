package middleware_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"tools.xdoubleu.com/internal/errortools"
	"tools.xdoubleu.com/internal/middleware"
)

func newRateLimitedHandler(t *testing.T) http.Handler {
	t.Helper()

	next := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	return middleware.RateLimit(0, 1, time.Hour, time.Hour)(next)
}

func TestRateLimitReturnsConnectErrorForRPCRequests(t *testing.T) {
	t.Parallel()

	handler := newRateLimitedHandler(t)

	post := func() *httptest.ResponseRecorder {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(
			http.MethodPost,
			"/feeds.v1.FeedsService/ListFeeds",
			nil,
		)
		req.RemoteAddr = "203.0.113.1:1234"
		req.Header.Set("Content-Type", "application/proto")
		handler.ServeHTTP(rec, req)

		return rec
	}

	first := post()
	assert.Equal(t, http.StatusOK, first.Code)

	second := post()
	assert.Equal(t, http.StatusTooManyRequests, second.Code)

	var body map[string]any
	require.NoError(t, json.Unmarshal(second.Body.Bytes(), &body))
	assert.Equal(t, "resource_exhausted", body["code"])
	assert.Equal(t, errortools.MessageTooManyRequests, body["message"])
}

func TestRateLimitReturnsRESTErrorForNonRPCRequests(t *testing.T) {
	t.Parallel()

	handler := newRateLimitedHandler(t)

	get := func() *httptest.ResponseRecorder {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/api/version", nil)
		req.RemoteAddr = "203.0.113.2:1234"
		handler.ServeHTTP(rec, req)

		return rec
	}

	first := get()
	assert.Equal(t, http.StatusOK, first.Code)

	second := get()
	assert.Equal(t, http.StatusTooManyRequests, second.Code)

	var body map[string]any
	require.NoError(t, json.Unmarshal(second.Body.Bytes(), &body))
	assert.InEpsilon(t, float64(http.StatusTooManyRequests), body["status"], 0)
	assert.Equal(t, errortools.MessageTooManyRequests, body["message"])
}

func TestClientIP(t *testing.T) {
	cases := []struct {
		name, remote, forwarded, want string
	}{
		{"public peer ignores the header", "203.0.113.5:1", "198.51.100.1", "203.0.113.5"},
		{
			"proxy peer uses the last hop",
			"172.18.0.2:1", "1.1.1.1, 198.51.100.7", "198.51.100.7",
		},
		{"loopback peer uses the header", "127.0.0.1:1", "198.51.100.8", "198.51.100.8"},
		{"proxy peer without header", "172.18.0.2:1", "", "172.18.0.2"},
		{"unparseable hop falls back", "10.0.0.2:1", "not-an-ip", "10.0.0.2"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/", nil)
			req.RemoteAddr = tc.remote
			if tc.forwarded != "" {
				req.Header.Set("X-Forwarded-For", tc.forwarded)
			}
			got, err := middleware.ClientIP(req)
			require.NoError(t, err)
			assert.Equal(t, tc.want, got)
		})
	}
}

func TestClientIP_BadRemoteAddr(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.RemoteAddr = "no-port"
	_, err := middleware.ClientIP(req)
	require.Error(t, err)
}
