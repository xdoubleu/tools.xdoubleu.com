package main

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestOriginCheckMiddleware(t *testing.T) {
	const (
		app  = "https://app.example.com"
		evil = "https://evil.example"
		bad  = "Origin"
		ref  = "Referer"
		auth = "Authorization"
		in   = "/api/auth/signin"
	)

	h := originCheckMiddleware([]string{app})(
		http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusOK)
		}),
	)

	tests := []struct {
		name   string
		method string
		path   string
		header string
		value  string
		want   int
	}{
		{"allowed origin", "POST", in, bad, app, 200},
		{"same host", "POST", in, bad, "http://api.test", 200},
		{"foreign origin", "POST", in, bad, evil, 403},
		{"foreign referer", "POST", in, ref, evil + "/x", 403},
		{"allowed referer", "POST", in, ref, app + "/x", 200},
		{"no origin", "POST", in, "", "", 200},
		{"bearer", "POST", "/mcp", auth, "Bearer t", 200},
		{"foreign GET", "GET", "/api/x", bad, evil, 200},
		{"token endpoint", "POST", "/oauth2/token", bad, evil, 200},
		{"register endpoint", "POST", "/oauth2/register", bad, evil, 200},
		{"opaque origin", "DELETE", "/api/x", bad, "null", 403},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := httptest.NewRequest(tt.method, tt.path, nil)
			r.Host = "api.test"
			if tt.header != "" {
				r.Header.Set(tt.header, tt.value)
			}
			w := httptest.NewRecorder()
			h.ServeHTTP(w, r)
			assert.Equal(t, tt.want, w.Code)
		})
	}

	t.Run("bearer with foreign origin", func(t *testing.T) {
		r := httptest.NewRequest(http.MethodPost, in, nil)
		r.Header.Set(auth, "Bearer t")
		r.Header.Set(bad, evil)
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		assert.Equal(t, http.StatusOK, w.Code)
	})
}
