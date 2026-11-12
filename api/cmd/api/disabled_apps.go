package main

import (
	"net/http"

	"github.com/getsentry/sentry-go"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

// appMigrationFailed backs Grafana's AppMigrationFailed alert.
//
//nolint:gochecknoglobals //Prometheus collectors are process-wide by design
var appMigrationFailed = promauto.NewGaugeVec(
	prometheus.GaugeOpts{
		Name: "app_migration_failed",
		Help: "1 when an app is disabled because its migration failed.",
	},
	[]string{"app"},
)

// reportDisabledApps logs, reports and gauges each app disabled at boot.
func (app *Application) reportDisabledApps() {
	for _, a := range *app.apps {
		appMigrationFailed.WithLabelValues(a.GetName()).Set(0)
	}
	for name, err := range app.disabledApps {
		app.logger.Error("app disabled: migration failed", "app", name, "error", err)
		appMigrationFailed.WithLabelValues(name).Set(1)
		sentry.CaptureException(err)
	}
}

func disabledAppHandler(name string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(
			w, name+" is temporarily unavailable", http.StatusServiceUnavailable,
		)
	})
}
