package icsproxy

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/google/uuid"
	tpltools "github.com/xdoubleu/essentia/v2/pkg/tpl"
	"tools.xdoubleu.com/apps/icsproxy/internal/models"
)

// =======================
// HOME PAGE
// =======================

func (app *ICSProxy) indexHandler(w http.ResponseWriter, r *http.Request) {
	summaries, _ := app.services.Calendar.ListConfigSummaries(r.Context())

	tpltools.RenderWithPanic(app.tpl, w, "index.html", map[string]any{
		"Configs": summaries,
	})
}

// =======================
// PREVIEW (NEW FILTER)
// =======================

func (app *ICSProxy) previewHandler(w http.ResponseWriter, r *http.Request) {
	sourceURL := r.FormValue("source_url")

	data, err := app.services.Calendar.FetchICS(r.Context(), sourceURL)
	if err != nil {
		http.Error(w, "Failed to fetch calendar", http.StatusBadGateway)
		return
	}

	events, err := app.services.Calendar.ExtractEvents(data)
	if err != nil {
		http.Error(w, "Failed to parse calendar", http.StatusInternalServerError)
		return
	}

	tpltools.RenderWithPanic(app.tpl, w, "preview.html", map[string]any{
		"SourceURL":          sourceURL,
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
	parts := strings.Split(strings.TrimSuffix(r.URL.Path, "/"), "/")
	token := parts[len(parts)-1]

	token = strings.TrimSuffix(token, ".ics")

	cfg, ok := app.services.Calendar.LoadConfig(r.Context(), token)
	if !ok {
		http.Error(w, "Filter not found", http.StatusNotFound)
		return
	}

	data, err := app.services.Calendar.FetchICS(r.Context(), cfg.SourceURL)
	if err != nil {
		http.Error(w, "Failed to fetch calendar", http.StatusBadGateway)
		return
	}

	events, err := app.services.Calendar.ExtractEvents(data)
	if err != nil {
		http.Error(w, "Failed to parse calendar", http.StatusInternalServerError)
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

	tpltools.RenderWithPanic(app.tpl, w, "preview.html", map[string]any{
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
	sourceURL := r.FormValue("source_url")

	token := r.FormValue("token")
	if token == "" {
		token = uuid.NewString()
	}

	cfg := models.FilterConfig{
		Token:         token,
		SourceURL:     sourceURL,
		HideEventUIDs: r.Form["hide_uid"],
		HolidayUIDs:   r.Form["holiday_uid"],
		HideSeries:    map[string]bool{},
	}

	for key := range r.Form {
		if strings.HasPrefix(key, "hide_rec_") {
			recKey := strings.TrimPrefix(key, "hide_rec_")
			cfg.HideSeries[recKey] = true
		}
	}

	if err := app.services.Calendar.SaveConfig(r.Context(), cfg); err != nil {
		app.logger.Error("Failed to save calendar config", "error", err)
		http.Error(w, "Failed to save config", http.StatusInternalServerError)
		return
	}

	downloadURL := fmt.Sprintf("/icsproxy/%s.ics", token)

	summaries, _ := app.services.Calendar.ListConfigSummaries(r.Context())

	tpltools.RenderWithPanic(app.tpl, w, "index.html", map[string]any{
		"GeneratedURL": downloadURL,
		"Configs":      summaries,
	})
}

// =======================
// DELETE FILTER
// =======================

func (app *ICSProxy) deleteHandler(w http.ResponseWriter, r *http.Request) {
	parts := strings.Split(strings.TrimSuffix(r.URL.Path, "/"), "/")
	token := parts[len(parts)-1]

	if err := app.services.Calendar.DeleteConfig(r.Context(), token); err != nil {
		app.logger.Error("Failed to delete filter", "error", err)
		http.Error(w, "Failed to delete filter", http.StatusInternalServerError)
		return
	}

	summaries, _ := app.services.Calendar.ListConfigSummaries(r.Context())

	tpltools.RenderWithPanic(app.tpl, w, "index.html", map[string]any{
		"Configs": summaries,
	})
}
