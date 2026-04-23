package main

import (
	"errors"
	"net/http"

	httptools "github.com/xdoubleu/essentia/v3/pkg/communication/httptools"
	tpltools "github.com/xdoubleu/essentia/v3/pkg/tpl"
	"tools.xdoubleu.com/cmd/publish/internal/dtos"
)

func (app *Application) adminHandler(w http.ResponseWriter, r *http.Request) {
	users, err := app.appUsersRepo.GetAllWithAccess(r.Context())
	if err != nil {
		http.Error(w, "failed to load users", http.StatusInternalServerError)
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
	user := currentUser(r)
	if user == nil {
		panic(errors.New("not signed in"))
	}

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
		http.Error(w, "failed to update role", http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, "/admin", http.StatusSeeOther)
}

func (app *Application) adminSetAppAccessHandler(
	w http.ResponseWriter,
	r *http.Request,
) {
	user := currentUser(r)
	if user == nil {
		panic(errors.New("not signed in"))
	}

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
		http.Error(w, "failed to update app access", http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, "/admin", http.StatusSeeOther)
}
