package observability_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"

	"tools.xdoubleu.com/internal/platform/observability"
)

func scrape(
	t *testing.T,
	h http.Handler,
	authorization string,
) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, observability.MetricsPath, nil)
	if authorization != "" {
		req.Header.Set("Authorization", authorization)
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func TestMetricsHandler_OpenWithoutAToken(t *testing.T) {
	rec := scrape(t, observability.MetricsHandler(""), "")

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Contains(t, rec.Body.String(), "go_goroutines")
}

func TestMetricsHandler_RequiresTheBearerToken(t *testing.T) {
	h := observability.MetricsHandler("s3cret")

	assert.Equal(t, http.StatusUnauthorized, scrape(t, h, "").Code)
	assert.Equal(t, http.StatusUnauthorized, scrape(t, h, "Bearer wrong").Code)
	assert.Equal(t, http.StatusUnauthorized, scrape(t, h, "s3cret").Code)
	assert.Equal(t, http.StatusUnauthorized, scrape(t, h, "Basic s3cret").Code)

	ok := scrape(t, h, "Bearer s3cret")
	assert.Equal(t, http.StatusOK, ok.Code)
	assert.Contains(t, ok.Body.String(), "go_goroutines")
}
