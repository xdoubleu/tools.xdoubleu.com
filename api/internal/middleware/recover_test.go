package middleware_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"

	"tools.xdoubleu.com/internal/logging"
	"tools.xdoubleu.com/internal/middleware"
)

func TestRecoverHandlesPanic(t *testing.T) {
	t.Parallel()

	panicky := http.HandlerFunc(func(_ http.ResponseWriter, _ *http.Request) {
		panic("boom")
	})

	handler := middleware.Recover(logging.NewNopLogger())(panicky)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)

	assert.NotPanics(t, func() {
		handler.ServeHTTP(rec, req)
	})
	assert.Equal(t, http.StatusInternalServerError, rec.Code)
	assert.Equal(t, "close", rec.Header().Get("connection"))
}
