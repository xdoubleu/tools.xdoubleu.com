package main

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestStripAPIPathPrefix(t *testing.T) {
	tests := []struct {
		name     string
		path     string
		wantPath string
	}{
		{"api prefix stripped", "/api/version", "/version"},
		{"bare /api becomes root", "/api", "/"},
		{
			"well-known left untouched",
			"/.well-known/oauth-protected-resource",
			"/.well-known/oauth-protected-resource",
		},
		{"unrelated path left untouched", "/health", "/health"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var gotPath string
			handler := stripAPIPathPrefix(http.HandlerFunc(
				func(_ http.ResponseWriter, r *http.Request) {
					gotPath = r.URL.Path
				},
			))

			req := httptest.NewRequest(http.MethodGet, tt.path, nil)
			handler.ServeHTTP(httptest.NewRecorder(), req)

			assert.Equal(t, tt.wantPath, gotPath)
		})
	}
}

// /metrics is scraped over the Docker network; through kamal-proxy it is 404.
func TestStripAPIPathPrefix_MetricsNotPublic(t *testing.T) {
	called := false
	handler := stripAPIPathPrefix(http.HandlerFunc(
		func(_ http.ResponseWriter, _ *http.Request) { called = true },
	))

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/metrics", nil))
	assert.Equal(t, http.StatusNotFound, rec.Code)
	assert.False(t, called)

	handler.ServeHTTP(
		httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/metrics", nil),
	)
	assert.True(t, called)
}
