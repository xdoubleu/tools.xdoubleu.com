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
