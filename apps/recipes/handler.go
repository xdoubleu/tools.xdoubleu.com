package recipes

import (
	"errors"
	"net/http"

	"github.com/xdoubleu/essentia/v3/pkg/database"
	"tools.xdoubleu.com/apps/recipes/internal/services"
	"tools.xdoubleu.com/internal/templates"
)

type handler func(http.ResponseWriter, *http.Request) error

func (a *Recipes) handle(h handler) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if err := h(w, r); err != nil {
			var svcErr *services.HTTPError
			switch {
			case errors.As(err, &svcErr):
				a.Logger.WarnContext(r.Context(), svcErr.Message,
					"status", svcErr.Status,
					"method", r.Method,
					"path", r.URL.Path,
				)
				templates.RenderError(a.Tpl, w, svcErr.Status, svcErr.Message)
			case errors.Is(err, database.ErrResourceNotFound):
				templates.RenderError(a.Tpl, w, http.StatusNotFound, "Not found")
			case errors.Is(err, database.ErrResourceConflict):
				templates.RenderError(
					a.Tpl,
					w,
					http.StatusConflict,
					"Resource already exists",
				)
			default:
				a.Logger.ErrorContext(r.Context(), "unexpected handler error",
					"error", err,
					"method", r.Method,
					"path", r.URL.Path,
				)
				templates.RenderError(
					a.Tpl, w, http.StatusInternalServerError,
					"An unexpected error occurred.",
				)
			}
		}
	}
}
