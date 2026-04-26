//nolint:testpackage // testing unexported package-level helpers
package backlog

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestHTTPError_Error(t *testing.T) {
	err := &HTTPError{Status: http.StatusNotFound, Message: "not found"}
	assert.Equal(t, "not found", err.Error())
}

func TestHTTPError_As(t *testing.T) {
	err := httpError(http.StatusBadRequest, "bad request")
	var httpErr *HTTPError
	assert.True(t, errors.As(err, &httpErr))
	assert.Equal(t, http.StatusBadRequest, httpErr.Status)
	assert.Equal(t, "bad request", httpErr.Message)
}

func TestHandle_HTTPError(t *testing.T) {
	app := &Backlog{} //nolint:exhaustruct //only tpl needed
	h := func(_ http.ResponseWriter, _ *http.Request) error {
		return httpError(http.StatusForbidden, "forbidden")
	}
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)

	wrapped := app.handle(h)

	// We expect a panic from tpltools.RenderWithPanic on nil template —
	// capture it so the test doesn't crash.
	assert.Panics(t, func() {
		wrapped(rr, req)
	})
}

func TestHandle_GenericError(t *testing.T) {
	app := &Backlog{} //nolint:exhaustruct //nil tpl panics on render
	h := func(_ http.ResponseWriter, _ *http.Request) error {
		return errors.New("something broke")
	}
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)

	wrapped := app.handle(h)

	assert.Panics(t, func() {
		wrapped(rr, req)
	})
}

func TestHandle_NoError(t *testing.T) {
	app := &Backlog{} //nolint:exhaustruct //tpl not used when handler succeeds
	h := func(w http.ResponseWriter, _ *http.Request) error {
		w.WriteHeader(http.StatusOK)
		return nil
	}
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)

	wrapped := app.handle(h)
	wrapped(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
}
