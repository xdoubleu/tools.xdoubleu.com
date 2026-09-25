package main

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"tools.xdoubleu.com/internal/routines"
)

func withRoutineFireToken(t *testing.T, token string) {
	t.Helper()
	original := testApp.config.RoutineFireToken
	testApp.config.RoutineFireToken = token
	t.Cleanup(func() { testApp.config.RoutineFireToken = original })
}

// withRoutinesFireServer points routinesClient at an httptest server until
// cleanup and returns the requests it received.
func withRoutinesFireServer(
	t *testing.T, status int,
) (*[]*http.Request, *[][]byte) {
	t.Helper()

	var requests []*http.Request
	var bodies [][]byte
	server := httptest.NewServer(http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			body := make([]byte, r.ContentLength)
			_, _ = r.Body.Read(body)
			bodies = append(bodies, body)
			requests = append(requests, r)
			w.WriteHeader(status)
		},
	))
	t.Cleanup(server.Close)

	original := testApp.routinesClient
	testApp.routinesClient = routines.NewClient(
		server.URL, "fire-token", testApp.automatedActionsRepo,
	)
	t.Cleanup(func() { testApp.routinesClient = original })

	return &requests, &bodies
}

func clearAutomatedActionsForWebhookTest(t *testing.T) {
	t.Helper()
	_, err := testApp.db.Exec(
		context.Background(), "DELETE FROM global.automated_actions",
	)
	require.NoError(t, err)
}

func TestRoutinesWebhookRoute_MissingToken_Unauthorized(t *testing.T) {
	withRoutineFireToken(t, "")

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(
		http.MethodPost, routinesWebhookPath, bytes.NewBufferString(`{}`),
	)
	testApp.routinesWebhookRoute()(rec, req)

	assert.Equal(t, http.StatusUnauthorized, rec.Code)
}

func TestRoutinesWebhookRoute_WrongToken_Unauthorized(t *testing.T) {
	withRoutineFireToken(t, "correct-token")

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(
		http.MethodPost, routinesWebhookPath, bytes.NewBufferString(`{}`),
	)
	req.Header.Set("Authorization", "Bearer wrong-token")
	testApp.routinesWebhookRoute()(rec, req)

	assert.Equal(t, http.StatusUnauthorized, rec.Code)
}

func TestRoutinesWebhookRoute_MissingBearerPrefix_Unauthorized(t *testing.T) {
	withRoutineFireToken(t, "correct-token")

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(
		http.MethodPost, routinesWebhookPath, bytes.NewBufferString(`{}`),
	)
	req.Header.Set("Authorization", "correct-token")
	testApp.routinesWebhookRoute()(rec, req)

	assert.Equal(t, http.StatusUnauthorized, rec.Code)
}

func TestRoutinesWebhookRoute_InvalidBody_BadRequest(t *testing.T) {
	withRoutineFireToken(t, "correct-token")

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(
		http.MethodPost, routinesWebhookPath, bytes.NewBufferString(`not json`),
	)
	req.Header.Set("Authorization", "Bearer correct-token")
	testApp.routinesWebhookRoute()(rec, req)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestRoutinesWebhookRoute_FiringAlert_FiresDefaultRoutine(t *testing.T) {
	withRoutineFireToken(t, "correct-token")
	clearAutomatedActionsForWebhookTest(t)
	requests, bodies := withRoutinesFireServer(t, http.StatusNoContent)

	//nolint:exhaustruct // Title intentionally omitted, unused by the handler
	payload := grafanaWebhookPayload{
		Status: "firing",
		Alerts: []grafanaWebhookAlert{
			{
				Status: "firing",
				Labels: map[string]string{"alertname": "SentryUnresolved"},
				Annotations: map[string]string{
					"summary": "unresolved Sentry issues detected",
				},
				GeneratorURL: "https://example.com/alert/1",
			},
		},
	}
	body, err := json.Marshal(payload)
	require.NoError(t, err)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(
		http.MethodPost,
		routinesWebhookPath,
		bytes.NewReader(body),
	)
	req.Header.Set("Authorization", "Bearer correct-token")
	testApp.routinesWebhookRoute()(rec, req)

	assert.Equal(t, http.StatusNoContent, rec.Code)
	require.Len(t, *requests, 1)
	assert.Equal(t, "/immediate-response/fire", (*requests)[0].URL.Path)
	assert.Equal(t, "Bearer fire-token", (*requests)[0].Header.Get("Authorization"))

	var sentBody struct {
		Text string `json:"text"`
	}
	require.NoError(t, json.Unmarshal((*bodies)[0], &sentBody))
	assert.Contains(t, sentBody.Text, "SentryUnresolved")
	assert.Contains(t, sentBody.Text, "unresolved Sentry issues detected")
	assert.Contains(t, sentBody.Text, "https://example.com/alert/1")

	var count int
	err = testApp.db.QueryRow(
		context.Background(),
		"SELECT count(*) FROM global.automated_actions "+
			"WHERE routine_name = 'immediate-response'",
	).Scan(&count)
	require.NoError(t, err)
	assert.Equal(t, 1, count)
}

func TestRoutinesWebhookRoute_RoutineLabel_OverridesDefault(t *testing.T) {
	withRoutineFireToken(t, "correct-token")
	clearAutomatedActionsForWebhookTest(t)
	requests, _ := withRoutinesFireServer(t, http.StatusNoContent)

	//nolint:exhaustruct // Title/Annotations/GeneratorURL unused by the handler
	payload := grafanaWebhookPayload{
		Status: "firing",
		Alerts: []grafanaWebhookAlert{
			{
				Status: "firing",
				Labels: map[string]string{
					"alertname": "SecurityAlert",
					"routine":   "security-response",
				},
			},
		},
	}
	body, err := json.Marshal(payload)
	require.NoError(t, err)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(
		http.MethodPost,
		routinesWebhookPath,
		bytes.NewReader(body),
	)
	req.Header.Set("Authorization", "Bearer correct-token")
	testApp.routinesWebhookRoute()(rec, req)

	assert.Equal(t, http.StatusNoContent, rec.Code)
	require.Len(t, *requests, 1)
	assert.Equal(t, "/security-response/fire", (*requests)[0].URL.Path)
}

func TestRoutinesWebhookRoute_ResolvedAlert_DoesNotFire(t *testing.T) {
	withRoutineFireToken(t, "correct-token")
	clearAutomatedActionsForWebhookTest(t)
	requests, _ := withRoutinesFireServer(t, http.StatusNoContent)

	//nolint:exhaustruct // Title unused by the handler
	payload := grafanaWebhookPayload{
		Status: "resolved",
		Alerts: []grafanaWebhookAlert{
			//nolint:exhaustruct // Annotations/GeneratorURL unused by the handler
			{
				Status: "resolved",
				Labels: map[string]string{"alertname": "SentryUnresolved"},
			},
		},
	}
	body, err := json.Marshal(payload)
	require.NoError(t, err)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(
		http.MethodPost,
		routinesWebhookPath,
		bytes.NewReader(body),
	)
	req.Header.Set("Authorization", "Bearer correct-token")
	testApp.routinesWebhookRoute()(rec, req)

	assert.Equal(t, http.StatusNoContent, rec.Code)
	assert.Empty(t, *requests)
}

func TestRoutinesWebhookRoute_FireFails_StillReturnsNoContent(t *testing.T) {
	withRoutineFireToken(t, "correct-token")
	clearAutomatedActionsForWebhookTest(t)
	requests, _ := withRoutinesFireServer(t, http.StatusInternalServerError)

	//nolint:exhaustruct // Title unused by the handler
	payload := grafanaWebhookPayload{
		Status: "firing",
		Alerts: []grafanaWebhookAlert{
			//nolint:exhaustruct // Annotations/GeneratorURL unused by the handler
			{
				Status: "firing",
				Labels: map[string]string{"alertname": "SentryUnresolved"},
			},
		},
	}
	body, err := json.Marshal(payload)
	require.NoError(t, err)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(
		http.MethodPost,
		routinesWebhookPath,
		bytes.NewReader(body),
	)
	req.Header.Set("Authorization", "Bearer correct-token")
	testApp.routinesWebhookRoute()(rec, req)

	// No retry: Fire already wrote the automated_actions row.
	assert.Equal(t, http.StatusNoContent, rec.Code)
	require.Len(t, *requests, 1)
}

func TestRoutinesWebhookAuthorized(t *testing.T) {
	withRoutineFireToken(t, "shhh")

	req := httptest.NewRequest(http.MethodPost, routinesWebhookPath, nil)
	assert.False(t, testApp.routinesWebhookAuthorized(req))

	req.Header.Set("Authorization", "Bearer shhh")
	assert.True(t, testApp.routinesWebhookAuthorized(req))
}

func TestFormatGrafanaAlertText(t *testing.T) {
	text := formatGrafanaAlertText(grafanaWebhookAlert{
		Status: "firing",
		Labels: map[string]string{"alertname": "TargetDown"},
		Annotations: map[string]string{
			"summary":     "a target is down",
			"description": "prometheus can't scrape it",
		},
		GeneratorURL: "https://example.com/g",
	})

	assert.Contains(t, text, "TargetDown")
	assert.Contains(t, text, "a target is down")
	assert.Contains(t, text, "prometheus can't scrape it")
	assert.Contains(t, text, "https://example.com/g")
}
