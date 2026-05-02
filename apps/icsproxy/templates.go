package icsproxy

import (
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/google/uuid"
	httptools "github.com/xdoubleu/essentia/v4/pkg/communication/httptools"
	"github.com/xdoubleu/essentia/v4/pkg/contexttools"
	tpltools "github.com/xdoubleu/essentia/v4/pkg/tpl"
	"tools.xdoubleu.com/apps/icsproxy/internal/dtos"
	"tools.xdoubleu.com/apps/icsproxy/internal/models"
	"tools.xdoubleu.com/internal/constants"
	sharedmodels "tools.xdoubleu.com/internal/models"
	"tools.xdoubleu.com/internal/templates"
)

func currentUser(r *http.Request) *sharedmodels.User {
	return contexttools.GetValue[sharedmodels.User](
		r.Context(),
		constants.UserContextKey,
	)
}

// =======================
// HOME PAGE
// =======================

func (app *ICSProxy) indexHandler(w http.ResponseWriter, r *http.Request) {
	user := currentUser(r)
	if user == nil {
		panic(errors.New("not signed in"))
	}

	summaries, _ := app.services.Calendar.ListConfigSummaries(r.Context(), user.ID)

	tpltools.RenderWithPanic(app.Tpl, w, "index.html", map[string]any{
		"Configs": summaries,
	})
}

// =======================
// PREVIEW (NEW FILTER)
// =======================

func (app *ICSProxy) previewHandler(w http.ResponseWriter, r *http.Request) {
	var dto dtos.PreviewDto
	if err := httptools.ReadForm(r, &dto); err != nil {
		httptools.HandleError(w, r, err)
		return
	}

	if ok, errs := dto.Validate(); !ok {
		httptools.FailedValidationResponse(w, r, errs)
		return
	}

	data, err := app.services.Calendar.FetchICS(r.Context(), dto.SourceURL)
	if err != nil {
		templates.RenderError(
			app.Tpl,
			w,
			http.StatusBadGateway,
			"Failed to fetch calendar",
		)
		return
	}

	events, err := app.services.Calendar.ExtractEvents(r.Context(), data)
	if err != nil {
		templates.RenderError(
			app.Tpl,
			w,
			http.StatusInternalServerError,
			"Failed to parse calendar",
		)
		return
	}

	tpltools.RenderWithPanic(app.Tpl, w, "preview.html", map[string]any{
		"SourceURL":          dto.SourceURL,
		"Events":             events,
		"CheckedHideUIDs":    map[string]bool{},
		"CheckedHolidayUIDs": map[string]bool{},
		"CheckedRecs":        map[string]bool{},
		"Editing":            false,
	})
}

// =======================
// EDIT EXISTING FILTER
// =======================

func (app *ICSProxy) editHandler(w http.ResponseWriter, r *http.Request) {
	user := currentUser(r)
	if user == nil {
		panic(errors.New("not signed in"))
	}

	parts := strings.Split(strings.TrimSuffix(r.URL.Path, "/"), "/")
	token := parts[len(parts)-1]
	token = strings.TrimSuffix(token, ".ics")

	cfg, ok := app.services.Calendar.LoadConfig(r.Context(), token)
	if !ok {
		templates.RenderError(app.Tpl, w, http.StatusNotFound, "Filter not found")
		return
	}

	if cfg.UserID != user.ID {
		templates.RenderError(
			app.Tpl,
			w,
			http.StatusForbidden,
			"You do not have access to this filter",
		)
		return
	}

	data, err := app.services.Calendar.FetchICS(r.Context(), cfg.SourceURL)
	if err != nil {
		templates.RenderError(
			app.Tpl,
			w,
			http.StatusBadGateway,
			"Failed to fetch calendar",
		)
		return
	}

	events, err := app.services.Calendar.ExtractEvents(r.Context(), data)
	if err != nil {
		templates.RenderError(
			app.Tpl,
			w,
			http.StatusInternalServerError,
			"Failed to parse calendar",
		)
		return
	}

	hideUIDs := map[string]bool{}
	for _, uid := range cfg.HideEventUIDs {
		hideUIDs[uid] = true
	}

	holidayUIDs := map[string]bool{}
	for _, uid := range cfg.HolidayUIDs {
		holidayUIDs[uid] = true
	}

	tpltools.RenderWithPanic(app.Tpl, w, "preview.html", map[string]any{
		"SourceURL":          cfg.SourceURL,
		"Events":             events,
		"CheckedHideUIDs":    hideUIDs,
		"CheckedHolidayUIDs": holidayUIDs,
		"CheckedRecs":        cfg.HideSeries,
		"Editing":            true,
		"EditingToken":       token,
	})
}

// =======================
// CREATE / UPDATE FILTER
// =======================

func (app *ICSProxy) createHandler(w http.ResponseWriter, r *http.Request) {
	user := currentUser(r)
	if user == nil {
		panic(errors.New("not signed in"))
	}

	var dto dtos.CreateFilterDto
	if err := httptools.ReadForm(r, &dto); err != nil {
		httptools.HandleError(w, r, err)
		return
	}

	if ok, errs := dto.Validate(); !ok {
		httptools.FailedValidationResponse(w, r, errs)
		return
	}

	if dto.Token == "" {
		dto.Token = uuid.NewString()
	}

	cfg := models.FilterConfig{
		Token:         dto.Token,
		UserID:        user.ID,
		SourceURL:     dto.SourceURL,
		HideEventUIDs: dto.HideEventUIDs,
		HolidayUIDs:   dto.HolidayUIDs,
		HideSeries:    dto.HideSeries(r.Form),
	}

	if err := app.services.Calendar.SaveConfig(r.Context(), cfg); err != nil {
		app.Logger.ErrorContext(
			r.Context(),
			"Failed to save calendar config",
			"error",
			err,
		)
		templates.RenderError(
			app.Tpl,
			w,
			http.StatusInternalServerError,
			"Failed to save config",
		)
		return
	}

	downloadURL := fmt.Sprintf("/icsproxy/%s.ics", dto.Token)

	summaries, _ := app.services.Calendar.ListConfigSummaries(r.Context(), user.ID)

	tpltools.RenderWithPanic(app.Tpl, w, "index.html", map[string]any{
		"GeneratedURL": downloadURL,
		"Configs":      summaries,
	})
}

// =======================
// DELETE FILTER
// =======================

func (app *ICSProxy) deleteHandler(w http.ResponseWriter, r *http.Request) {
	user := currentUser(r)
	if user == nil {
		panic(errors.New("not signed in"))
	}

	parts := strings.Split(strings.TrimSuffix(r.URL.Path, "/"), "/")
	token := parts[len(parts)-1]

	if err := app.services.Calendar.DeleteConfig(r.Context(), token, user.ID); err != nil {
		app.Logger.ErrorContext(r.Context(), "Failed to delete filter", "error", err)
		templates.RenderError(
			app.Tpl,
			w,
			http.StatusInternalServerError,
			"Failed to delete filter",
		)
		return
	}

	summaries, _ := app.services.Calendar.ListConfigSummaries(r.Context(), user.ID)

	tpltools.RenderWithPanic(app.Tpl, w, "index.html", map[string]any{
		"Configs": summaries,
	})
}
