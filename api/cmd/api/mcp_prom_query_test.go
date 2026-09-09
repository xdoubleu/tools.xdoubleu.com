package main

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPromQuery_ReturnsRawResponseBody(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "/api/v1/query", r.URL.Path)
			assert.Equal(t, `up{job="api"}`, r.URL.Query().Get("query"))
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(
				`{"status":"success","data":{"resultType":"vector","result":[]}}`,
			))
		},
	))
	t.Cleanup(srv.Close)

	body, err := promQuery(context.Background(), srv.URL, `up{job="api"}`)
	require.NoError(t, err)
	assert.JSONEq(
		t,
		`{"status":"success","data":{"resultType":"vector","result":[]}}`,
		string(body),
	)
}

func TestPromQuery_RequestErrorPropagates(t *testing.T) {
	_, err := promQuery(context.Background(), "http://[::1]:namedport", "up")
	require.Error(t, err)
}

func TestAppsMCPPromQuery_AsAdmin(t *testing.T) {
	promoteToAdmin(t)
	t.Cleanup(func() { demoteToUser(t) })

	srv := httptest.NewServer(http.HandlerFunc(
		func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"status":"success","data":{"result":[]}}`))
		},
	))
	t.Cleanup(srv.Close)

	original := testApp.config.PrometheusURL
	testApp.config.PrometheusURL = srv.URL
	t.Cleanup(func() { testApp.config.PrometheusURL = original })

	session := appsMCPSession(t, accessToken.Value)
	//nolint:exhaustruct // only Name/Arguments are needed to call the tool
	res, err := session.CallTool(context.Background(), &mcp.CallToolParams{
		Name:      "prom_query",
		Arguments: map[string]any{"query": "up"},
	})
	require.NoError(t, err)
	assert.False(t, res.IsError)
	assert.Contains(t, toolMessage(res, nil), `"status":"success"`)
}

func TestAppsMCPPromQuery_NonAdmin(t *testing.T) {
	demoteToUser(t)

	session := appsMCPSession(t, accessToken.Value)
	//nolint:exhaustruct // only Name/Arguments are needed to call the tool
	res, err := session.CallTool(context.Background(), &mcp.CallToolParams{
		Name:      "prom_query",
		Arguments: map[string]any{"query": "up"},
	})
	assert.Contains(t, toolMessage(res, err), "admin access required")
}
