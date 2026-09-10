package main

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const grafanaRulesJSON = `{"status":"success","data":{"groups":[` +
	`{"name":"host","rules":[{"name":"PostgresDown","state":"inactive",` +
	`"alerts":[]}]}]}}`

func TestGrafanaAlerts_ReturnsRawResponseBodyWithBasicAuth(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(
				t, "/api/prometheus/grafana/api/v1/rules", r.URL.Path,
			)
			user, pass, ok := r.BasicAuth()
			assert.True(t, ok)
			assert.Equal(t, "admin", user)
			assert.Equal(t, "s3cret", pass)
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(grafanaRulesJSON))
		},
	))
	t.Cleanup(srv.Close)

	body, err := grafanaAlerts(context.Background(), srv.URL, "s3cret")
	require.NoError(t, err)
	assert.JSONEq(t, grafanaRulesJSON, string(body))
}

func TestGrafanaAlerts_NotConfigured(t *testing.T) {
	_, err := grafanaAlerts(context.Background(), "http://grafana", "")
	require.ErrorIs(t, err, errGrafanaNotConfigured)
}

func TestGrafanaAlerts_RequestErrorPropagates(t *testing.T) {
	_, err := grafanaAlerts(
		context.Background(), "http://[::1]:namedport", "s3cret",
	)
	require.Error(t, err)
}

func TestAppsMCPGrafanaAlerts_AsAdmin(t *testing.T) {
	promoteToAdmin(t)
	t.Cleanup(func() { demoteToUser(t) })

	srv := httptest.NewServer(http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			assert.True(
				t,
				strings.HasPrefix(
					r.Header.Get("Authorization"), "Basic ",
				),
			)
			_, pass, _ := r.BasicAuth()
			assert.Equal(t, "test-pw", pass)
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(grafanaRulesJSON))
		},
	))
	t.Cleanup(srv.Close)

	originalURL := testApp.config.GrafanaURL
	originalPW := testApp.config.GrafanaAdminPassword
	testApp.config.GrafanaURL = srv.URL
	testApp.config.GrafanaAdminPassword = "test-pw"
	t.Cleanup(func() {
		testApp.config.GrafanaURL = originalURL
		testApp.config.GrafanaAdminPassword = originalPW
	})

	session := appsMCPSession(t, accessToken.Value)
	//nolint:exhaustruct // only Name/Arguments are needed to call the tool
	res, err := session.CallTool(context.Background(), &mcp.CallToolParams{
		Name:      "get_grafana_alerts",
		Arguments: map[string]any{},
	})
	require.NoError(t, err)
	assert.False(t, res.IsError)
	assert.Contains(t, toolMessage(res, nil), `"status":"success"`)
}

func TestAppsMCPGrafanaAlerts_NotConfiguredReturnsToolError(t *testing.T) {
	promoteToAdmin(t)
	t.Cleanup(func() { demoteToUser(t) })

	original := testApp.config.GrafanaAdminPassword
	testApp.config.GrafanaAdminPassword = ""
	t.Cleanup(func() { testApp.config.GrafanaAdminPassword = original })

	session := appsMCPSession(t, accessToken.Value)
	//nolint:exhaustruct // only Name/Arguments are needed to call the tool
	res, err := session.CallTool(context.Background(), &mcp.CallToolParams{
		Name:      "get_grafana_alerts",
		Arguments: map[string]any{},
	})
	assert.Contains(t, toolMessage(res, err), "GRAFANA_ADMIN_PASSWORD")
}

func TestAppsMCPGrafanaAlerts_NonAdmin(t *testing.T) {
	demoteToUser(t)

	session := appsMCPSession(t, accessToken.Value)
	//nolint:exhaustruct // only Name/Arguments are needed to call the tool
	res, err := session.CallTool(context.Background(), &mcp.CallToolParams{
		Name:      "get_grafana_alerts",
		Arguments: map[string]any{},
	})
	assert.Contains(t, toolMessage(res, err), "admin access required")
}
