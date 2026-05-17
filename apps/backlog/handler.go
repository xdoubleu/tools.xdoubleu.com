package backlog

import (
	"errors"
	"net/http"

	iapp "tools.xdoubleu.com/internal/app"
	"tools.xdoubleu.com/internal/templates"
)

func httpError(status int, message string) error {
	return &iapp.HTTPError{Status: status, Message: message}
}

// handler is a net/http handler that returns an error instead of panicking.
type handler func(http.ResponseWriter, *http.Request) error

// handle wraps an error-returning handler and renders an HTML error page on failure.
func (app *Backlog) handle(h handler) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if err := h(w, r); err != nil {
			var httpErr *iapp.HTTPError
			if errors.As(err, &httpErr) {
				app.Logger.WarnContext(r.Context(), httpErr.Message,
					"status", httpErr.Status,
					"method", r.Method,
					"path", r.URL.Path,
				)
				templates.RenderError(w, httpErr.Status, httpErr.Message)
			} else {
				app.Logger.ErrorContext(r.Context(), "unexpected handler error",
					"error", err,
					"method", r.Method,
					"path", r.URL.Path,
				)
				templates.RenderError(
					w, http.StatusInternalServerError,
					"An unexpected error occurred.",
				)
			}
		}
	}
}
