package middleware_test

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"tools.xdoubleu.com/internal/logging"
	"tools.xdoubleu.com/internal/middleware"
)

func TestDefault(t *testing.T) {
	t.Parallel()

	handlers, err := middleware.Default(
		logging.NewNopLogger(),
		[]string{"http://localhost"},
	)
	require.NoError(t, err)
	assert.NotEmpty(t, handlers)
}

func TestRedactPath(t *testing.T) {
	assert.Equal(t,
		"/api/books/kobo/redacted/v1/library/sync",
		middleware.RedactPath("/api/books/kobo/abc123/v1/library/sync?x=1"),
	)
	assert.Equal(t, "/books/kobo/redacted", middleware.RedactPath("/books/kobo/abc123"))
	assert.Equal(t, "/api/feeds", middleware.RedactPath("/api/feeds?token=t"))
}

func TestMaxBodyBytes(t *testing.T) {
	var readErr error
	handler := middleware.MaxBodyBytes(4)(http.HandlerFunc(
		func(_ http.ResponseWriter, r *http.Request) {
			_, readErr = io.ReadAll(r.Body)
		},
	))

	handler.ServeHTTP(httptest.NewRecorder(),
		httptest.NewRequest(http.MethodPost, "/", strings.NewReader("12345")))
	require.Error(t, readErr)

	handler.ServeHTTP(httptest.NewRecorder(),
		httptest.NewRequest(http.MethodPost, "/", strings.NewReader("1234")))
	require.NoError(t, readErr)
}
