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
		{"well-known left untouched", "/.well-known/oauth-protected-resource", "/.well-known/oauth-protected-resource"},
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
