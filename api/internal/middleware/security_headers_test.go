package middleware_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"

	"tools.xdoubleu.com/internal/middleware"
)

func TestSecurityHeaders_SetsExactHeaderSet(t *testing.T) {
	handler := middleware.SecurityHeaders(http.HandlerFunc(
		func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusNoContent)
		},
	))

	rr := httptest.NewRecorder()
	rr.Header().Set("X-Powered-By", "upstream")
	handler.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/", nil))

	assert.Equal(t, http.Header{
		"Strict-Transport-Security": {"max-age=5184000; includeSubDomains"},
		"X-Content-Type-Options":    {"nosniff"},
		"X-Dns-Prefetch-Control":    {"off"},
		"X-Download-Options":        {"noopen"},
		"X-Frame-Options":           {"SAMEORIGIN"},
		"X-Xss-Protection":          {"1; mode=block"},
	}, rr.Result().Header)
}
