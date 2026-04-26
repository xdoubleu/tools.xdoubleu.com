package backlog

import (
	"errors"
	"net/http"

	"tools.xdoubleu.com/internal/templates"
)

// HTTPError carries an HTTP status alongside a user-facing message.
type HTTPError struct {
	Status  int
	Message string
}

func (e *HTTPError) Error() string { return e.Message }

func httpError(status int, message string) error {
	return &HTTPError{Status: status, Message: message}
}

// handler is a net/http handler that returns an error instead of panicking.
type handler func(http.ResponseWriter, *http.Request) error

// handle wraps an error-returning handler and renders an HTML error page on failure.
func (app *Backlog) handle(h handler) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if err := h(w, r); err != nil {
			var httpErr *HTTPError
			if errors.As(err, &httpErr) {
				app.logger.WarnContext(r.Context(), httpErr.Message,
					"status", httpErr.Status,
					"method", r.Method,
					"path", r.URL.Path,
				)
				templates.RenderError(app.tpl, w, httpErr.Status, httpErr.Message)
			} else {
				app.logger.ErrorContext(r.Context(), "unexpected handler error",
					"error", err,
					"method", r.Method,
					"path", r.URL.Path,
				)
				templates.RenderError(
					app.tpl, w, http.StatusInternalServerError,
					"An unexpected error occurred.",
				)
			}
		}
	}
}
