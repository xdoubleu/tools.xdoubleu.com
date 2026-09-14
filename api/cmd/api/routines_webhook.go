package main

import (
	"crypto/subtle"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
)

// routinesWebhookPath is a plain HTTP endpoint (not ConnectRPC) that a
// Grafana webhook contact point calls directly — this is the inbound half
// of issue #1444's immediate-fire path: a Grafana-detected problem
// (Sentry-unresolved, security-alert, and #1443's future staleness rule)
// reaches this route and gets translated into a routines.Client.Fire call,
// letting the alert trigger a routine directly instead of the terminal
// step being an email a human has to notice.
const routinesWebhookPath = "/webhooks/grafana-alert"

// defaultImmediateRoutineName is the routine fired when a firing alert
// carries no "routine" label of its own — a single shared routine that
// receives the alert's full context (rule name, labels, annotations) and
// decides what to do, rather than provisioning a separate routine per
// alert rule.
const defaultImmediateRoutineName = "immediate-response"

// grafanaWebhookAlert is one entry of a Grafana alerting webhook's `alerts`
// array. Grafana's webhook payload is Prometheus Alertmanager-shaped;
// fields not used here (startsAt, endsAt, fingerprint, silenceURL,
// dashboardURL, panelURL, values) are intentionally omitted rather than
// exhaustively modeled.
type grafanaWebhookAlert struct {
	Status       string            `json:"status"`
	Labels       map[string]string `json:"labels"`
	Annotations  map[string]string `json:"annotations"`
	GeneratorURL string            `json:"generatorURL"`
}

// grafanaWebhookPayload is the top-level body Grafana POSTs to a webhook
// contact point.
type grafanaWebhookPayload struct {
	Status string                `json:"status"`
	Alerts []grafanaWebhookAlert `json:"alerts"`
	Title  string                `json:"title"`
}

// routinesWebhookRoute authenticates via a shared bearer token
// (ROUTINE_FIRE_TOKEN) rather than the cookie-session middleware most
// routes in this file use, since Grafana's webhook contact point has no
// user session to present — same shape as observabilityIngestRoute's
// shared-secret auth.
func (app *Application) routinesWebhookRoute() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !app.routinesWebhookAuthorized(r) {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}

		var payload grafanaWebhookPayload
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			http.Error(w, "invalid request body", http.StatusBadRequest)
			return
		}

		for _, alert := range payload.Alerts {
			// Only a new/ongoing problem should trigger a routine — a
			// "resolved" entry means the alert already cleared, so firing
			// on it would send a routine to fix something that's no
			// longer broken.
			if alert.Status != "firing" {
				continue
			}

			routineName := alert.Labels["routine"]
			if routineName == "" {
				routineName = defaultImmediateRoutineName
			}

			if err := app.routinesClient.Fire(
				r.Context(), routineName, formatGrafanaAlertText(alert),
			); err != nil {
				// Logged, not surfaced as a failure response: returning a
				// non-2xx here would make Grafana retry the whole webhook
				// payload, re-firing any alert in it that already
				// succeeded (Fire is not idempotent — each call opens a
				// fresh automated_actions row).
				app.logger.ErrorContext(
					r.Context(), "failed to fire routine from grafana alert",
					"error", err,
					"routine", routineName,
					"alertname", alert.Labels["alertname"],
				)
			}
		}

		w.WriteHeader(http.StatusNoContent)
	}
}

func (app *Application) routinesWebhookAuthorized(r *http.Request) bool {
	if app.config.RoutineFireToken == "" {
		return false
	}

	const prefix = "Bearer "
	auth := r.Header.Get("Authorization")
	if !strings.HasPrefix(auth, prefix) {
		return false
	}
	provided := strings.TrimPrefix(auth, prefix)

	return subtle.ConstantTimeCompare(
		[]byte(provided), []byte(app.config.RoutineFireToken),
	) == 1
}

// formatGrafanaAlertText turns one firing alert into the freeform text
// routines.Client.Fire hands the routine as context, so it doesn't have to
// rediscover what triggered it.
func formatGrafanaAlertText(alert grafanaWebhookAlert) string {
	var b strings.Builder

	fmt.Fprintf(
		&b,
		"Grafana alert %q is %s.\n",
		alert.Labels["alertname"],
		alert.Status,
	)
	if summary := alert.Annotations["summary"]; summary != "" {
		fmt.Fprintf(&b, "Summary: %s\n", summary)
	}
	if description := alert.Annotations["description"]; description != "" {
		fmt.Fprintf(&b, "Description: %s\n", description)
	}
	if len(alert.Labels) > 0 {
		fmt.Fprintf(&b, "Labels: %v\n", alert.Labels)
	}
	if alert.GeneratorURL != "" {
		fmt.Fprintf(&b, "Details: %s\n", alert.GeneratorURL)
	}

	return b.String()
}
