package main

import (
	"net/http"

	httptools "github.com/xdoubleu/essentia/v4/pkg/communication/httptools"
	tpltools "github.com/xdoubleu/essentia/v4/pkg/tpl"
	"tools.xdoubleu.com/cmd/publish/internal/dtos"
	"tools.xdoubleu.com/internal/templates"
)

func (app *Application) adminHandler(w http.ResponseWriter, r *http.Request) {
	users, err := app.appUsersRepo.GetAllWithAccess(r.Context())
	if err != nil {
		templates.RenderError(
			app.tpl,
			w,
			http.StatusInternalServerError,
			"Failed to load users",
		)
		return
	}

	appNames := []string{}
	for _, a := range *app.apps {
		appNames = append(appNames, a.GetName())
	}

	tpltools.RenderWithPanic(app.tpl, w, "admin.html", map[string]any{
		"Users":    users,
		"AppNames": appNames,
	})
}

func (app *Application) adminSetRoleHandler(w http.ResponseWriter, r *http.Request) {
	userID := r.PathValue("userID")

	var dto dtos.SetRoleDto
	if err := httptools.ReadForm(r, &dto); err != nil {
		httptools.HandleError(w, r, err)
		return
	}

	if ok, errs := dto.Validate(); !ok {
		httptools.FailedValidationResponse(w, r, errs)
		return
	}

	if err := app.appUsersRepo.SetRole(r.Context(), userID, dto.Role); err != nil {
		templates.RenderError(
			app.tpl,
			w,
			http.StatusInternalServerError,
			"Failed to update role",
		)
		return
	}

	app.renderAdminRowOrRedirect(w, r, userID)
}

func (app *Application) adminSetAppAccessHandler(
	w http.ResponseWriter,
	r *http.Request,
) {
	userID := r.PathValue("userID")
	appName := r.PathValue("appName")

	var dto dtos.SetAppAccessDto
	if err := httptools.ReadForm(r, &dto); err != nil {
		httptools.HandleError(w, r, err)
		return
	}

	if err := app.appUsersRepo.SetAppAccess(
		r.Context(),
		userID,
		appName,
		dto.Grant); err != nil {
		templates.RenderError(
			app.tpl,
			w,
			http.StatusInternalServerError,
			"Failed to update app access",
		)
		return
	}

	app.renderAdminRowOrRedirect(w, r, userID)
}

// renderAdminRowOrRedirect renders the updated user row for HTMX requests,
// or redirects for plain form submissions.
func (app *Application) renderAdminRowOrRedirect(
	w http.ResponseWriter,
	r *http.Request,
	userID string,
) {
	if r.Header.Get("HX-Request") != "true" {
		http.Redirect(w, r, "/admin", http.StatusSeeOther)
		return
	}

	user, err := app.appUsersRepo.GetByID(r.Context(), userID)
	if err != nil {
		templates.RenderError(
			app.tpl,
			w,
			http.StatusInternalServerError,
			"Failed to load user",
		)
		return
	}

	appNames := []string{}
	for _, a := range *app.apps {
		appNames = append(appNames, a.GetName())
	}

	tpltools.RenderWithPanic(app.tpl, w, "admin_user_row", map[string]any{
		"User":     *user,
		"AppNames": appNames,
	})
}
