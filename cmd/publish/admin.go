package main

import (
	"errors"
	"net/http"

	tpltools "github.com/xdoubleu/essentia/v3/pkg/tpl"
	"tools.xdoubleu.com/internal/models"
)

const adminMaxBodySize = 1 << 20

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

	r.Body = http.MaxBytesReader(w, r.Body, adminMaxBodySize)
	role := models.Role(r.FormValue("role"))

	if role != models.RoleAdmin && role != models.RoleUser {
		http.Error(w, "invalid role", http.StatusBadRequest)
		return
	}

	if err := app.appUsersRepo.SetRole(r.Context(), userID, role); err != nil {
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

	r.Body = http.MaxBytesReader(w, r.Body, adminMaxBodySize)
	grant := r.FormValue("grant") == "true"

	if err := app.appUsersRepo.SetAppAccess(
		r.Context(),
		userID,
		appName,
		grant); err != nil {
		http.Error(w, "failed to update app access", http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, "/admin", http.StatusSeeOther)
}
