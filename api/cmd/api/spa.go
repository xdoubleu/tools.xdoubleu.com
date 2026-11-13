package main

import (
	"net/http"

	"tools.xdoubleu.com/internal/platform/observability"
	"tools.xdoubleu.com/internal/platform/spa"
)

// Handler is the whole server: the built web app in front of the API when
// WEB_DIR is set on the web role, else just the API (local development runs
// Vite's dev server).
// It strips the /api prefix kamal-proxy no longer splits off.
func (app *Application) Handler() (http.Handler, error) {
	api := stripAPIPathPrefix(app.handler())
	if app.config.WebDir == "" || app.config.IsWorkers() {
		return api, nil
	}

	var customDomains []string
	for _, a := range app.services() {
		if d := a.GetDomain(); d != "" {
			customDomains = append(customDomains, d)
		}
	}

	web, err := spa.New(spa.Config{
		Dir:                app.config.WebDir,
		APIURL:             app.config.APIURL,
		Release:            app.config.Release,
		KoboGatewayRelease: app.config.KoboGatewayRelease,
		SentryDSN:          app.config.SentryDsnWeb,
		PostHogKey:         app.config.PostHogKey,
		PostHogHost:        app.config.PostHogHost,
		OfflineDisabled:    app.config.OfflineDisabled,
		APIHosts:           customDomains,
		Logs:               observability.WebLogsHandler(app.logger, app.logsRepo),
	}, app.logger)
	if err != nil {
		return nil, err
	}
	return web.Wrap(api), nil
}
