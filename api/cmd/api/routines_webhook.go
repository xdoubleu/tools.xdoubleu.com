package main

import (
	"crypto/subtle"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
)

// routinesWebhookPath is called by a Grafana webhook contact point; each
// firing alert becomes a routines.Client.Fire call.
const routinesWebhookPath = "/webhooks/grafana-alert"

// defaultImmediateRoutineName is fired for alerts without a "routine" label;
// it receives the alert's context and decides what to do.
const defaultImmediateRoutineName = "immediate-response"

// grafanaWebhookAlert is one Alertmanager-shaped entry of the payload's
// `alerts`; unused fields are omitted.
type grafanaWebhookAlert struct {
	Status       string            `json:"status"`
	Labels       map[string]string `json:"labels"`
	Annotations  map[string]string `json:"annotations"`
	GeneratorURL string            `json:"generatorURL"`
}

// grafanaWebhookPayload is the body Grafana POSTs.
type grafanaWebhookPayload struct {
	Status string                `json:"status"`
	Alerts []grafanaWebhookAlert `json:"alerts"`
	Title  string                `json:"title"`
}

// routinesWebhookRoute authenticates via ROUTINE_FIRE_TOKEN; Grafana has no
// user session.
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
			// Resolved alerts don't fire a routine.
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
				// Log instead of non-2xx: Grafana would retry the whole payload, and Fire
				// isn't idempotent.
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

// formatGrafanaAlertText renders an alert as the routine's context.
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
