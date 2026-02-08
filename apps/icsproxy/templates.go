package icsproxy

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/google/uuid"
	tpltools "github.com/xdoubleu/essentia/v2/pkg/tpl"
	"tools.xdoubleu.com/apps/icsproxy/internal/models"
)

func (app *ICSProxy) indexHandler(w http.ResponseWriter, _ *http.Request) {
	tpltools.RenderWithPanic(app.tpl, w, "index.html", nil)
}

func (app *ICSProxy) createHandler(w http.ResponseWriter, r *http.Request) {
	sourceURL := r.FormValue("source_url")

	// Generate token FIRST
	token := uuid.NewString()

	// Build config WITH token
	cfg := models.FilterConfig{
		Token:         token, // <-- CRITICAL FIX
		SourceURL:     sourceURL,
		HideEventUIDs: r.Form["hide_uid"],
		HolidayUIDs:   r.Form["holiday_uid"],
		HideSeries:    map[string]bool{},
	}

	// collect recurring hides
	for key := range r.Form {
		if strings.HasPrefix(key, "hide_rec_") {
			recKey := strings.TrimPrefix(key, "hide_rec_")
			cfg.HideSeries[recKey] = true
		}
	}

	// Persist config
	if err := app.services.Calendar.SaveConfig(r.Context(), cfg); err != nil {
		app.logger.Error("Failed to save calendar config", "error", err)
		http.Error(w, "Failed to save config", http.StatusInternalServerError)
		return
	}

	downloadURL := fmt.Sprintf("/icsproxy/%s.ics", token)

	tpltools.RenderWithPanic(app.tpl, w, "index.html", map[string]string{
		"GeneratedURL": downloadURL,
	})
}

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
		"SourceURL": sourceURL,
		"Events":    events,
	})
}
