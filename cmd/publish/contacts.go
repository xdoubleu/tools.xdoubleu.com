package main

import (
	"net/http"

	"github.com/google/uuid"
	httptools "github.com/xdoubleu/essentia/v3/pkg/communication/httptools"
	tpltools "github.com/xdoubleu/essentia/v3/pkg/tpl"
	"tools.xdoubleu.com/internal/templates"
)

type createContactDto struct {
	Email       string `schema:"email"`
	DisplayName string `schema:"display_name"`
}

type acceptContactDto struct {
	DisplayName string `schema:"display_name"`
}

func (app *Application) renderContactsPage(
	w http.ResponseWriter,
	r *http.Request,
	userID string,
	errMsg string,
) {
	contactList, _ := app.contacts.List(r.Context(), userID)
	pending, _ := app.contacts.ListPending(r.Context(), userID)
	incoming, _ := app.contacts.ListIncoming(r.Context(), userID)
	tpltools.RenderWithPanic(app.tpl, w, "contacts.html", map[string]any{
		"Contacts": contactList,
		"Pending":  pending,
		"Incoming": incoming,
		"Error":    errMsg,
	})
}

func (app *Application) listContactsHandler(w http.ResponseWriter, r *http.Request) {
	user := currentUser(r)
	if user == nil {
		templates.RenderError(app.tpl, w, http.StatusUnauthorized,
			"Sign in to access this page")
		return
	}
	app.renderContactsPage(w, r, user.ID, "")
}

func (app *Application) createContactHandler(w http.ResponseWriter, r *http.Request) {
	user := currentUser(r)
	if user == nil {
		templates.RenderError(app.tpl, w, http.StatusUnauthorized,
			"Sign in to access this page")
		return
	}

	var dto createContactDto
	if err := httptools.ReadForm(r, &dto); err != nil {
		app.renderContactsPage(w, r, user.ID, "Invalid form data")
		return
	}

	if err := app.contacts.AddByEmail(
		r.Context(), user.ID, dto.Email, dto.DisplayName,
	); err != nil {
		app.renderContactsPage(w, r, user.ID,
			"No user found with that email address")
		return
	}

	http.Redirect(w, r, "/contacts", http.StatusSeeOther)
}

func (app *Application) acceptContactHandler(w http.ResponseWriter, r *http.Request) {
	user := currentUser(r)
	if user == nil {
		templates.RenderError(app.tpl, w, http.StatusUnauthorized,
			"Sign in to access this page")
		return
	}

	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		templates.RenderError(app.tpl, w, http.StatusNotFound, "Contact not found")
		return
	}

	var dto acceptContactDto
	if err = httptools.ReadForm(r, &dto); err != nil {
		templates.RenderError(app.tpl, w, http.StatusBadRequest, "Invalid form data")
		return
	}

	if err = app.contacts.Accept(r.Context(), id, user.ID, dto.DisplayName); err != nil {
		templates.RenderError(app.tpl, w, http.StatusInternalServerError,
			"Failed to accept contact request")
		return
	}

	http.Redirect(w, r, "/contacts", http.StatusSeeOther)
}

func (app *Application) declineContactHandler(w http.ResponseWriter, r *http.Request) {
	user := currentUser(r)
	if user == nil {
		templates.RenderError(app.tpl, w, http.StatusUnauthorized,
			"Sign in to access this page")
		return
	}

	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		templates.RenderError(app.tpl, w, http.StatusNotFound, "Contact not found")
		return
	}

	if err = app.contacts.Decline(r.Context(), id, user.ID); err != nil {
		templates.RenderError(app.tpl, w, http.StatusInternalServerError,
			"Failed to decline contact request")
		return
	}

	http.Redirect(w, r, "/contacts", http.StatusSeeOther)
}

func (app *Application) deleteContactHandler(w http.ResponseWriter, r *http.Request) {
	user := currentUser(r)
	if user == nil {
		templates.RenderError(app.tpl, w, http.StatusUnauthorized,
			"Sign in to access this page")
		return
	}

	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		templates.RenderError(app.tpl, w, http.StatusNotFound, "Contact not found")
		return
	}

	if err = app.contacts.Delete(r.Context(), id, user.ID); err != nil {
		templates.RenderError(app.tpl, w, http.StatusInternalServerError,
			"Failed to delete contact")
		return
	}

	http.Redirect(w, r, "/contacts", http.StatusSeeOther)
}
