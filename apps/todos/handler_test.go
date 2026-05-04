//nolint:testpackage // tests unexported helpers
package todos

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestHandle_NoError(t *testing.T) {
	a := &Todos{} //nolint:exhaustruct // only handle() needed for middleware test
	h := func(w http.ResponseWriter, _ *http.Request) error {
		w.WriteHeader(http.StatusOK)
		return nil
	}
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)

	a.handle(h)(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
}

func TestHandle_HTTPError(t *testing.T) {
	a := &Todos{} //nolint:exhaustruct // only handle() needed for middleware test
	h := func(_ http.ResponseWriter, _ *http.Request) error {
		return &stubHTTPError{status: http.StatusForbidden}
	}
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	assert.Panics(t, func() { a.handle(h)(rr, req) })
}

func TestHandle_GenericError(t *testing.T) {
	a := &Todos{} //nolint:exhaustruct // only handle() needed for middleware test
	h := func(_ http.ResponseWriter, _ *http.Request) error {
		return errors.New("boom")
	}
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	assert.Panics(t, func() { a.handle(h)(rr, req) })
}

// stubHTTPError is a small stand-in used only to exercise the handler middleware.
type stubHTTPError struct{ status int }

func (e *stubHTTPError) Error() string { return http.StatusText(e.status) }
