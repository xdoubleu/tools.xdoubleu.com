package main

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"connectrpc.com/connect"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	observabilityv1 "tools.xdoubleu.com/gen/observability/v1"
	"tools.xdoubleu.com/internal/github"
	"tools.xdoubleu.com/internal/logging"
)

func TestObservabilityGetProjectIssuesByStatus_AsAdmin(t *testing.T) {
	promoteToAdmin(t)
	t.Cleanup(func() { demoteToUser(t) })

	srv := httptest.NewServer(http.HandlerFunc(
		func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"data":{"user":{"projectV2":{"items":{"nodes":[
				{"status":{"name":"Ready","updatedAt":"2026-09-01T10:00:00Z"},
				 "content":{"number":1357,"title":"Add MCP tool",
				            "url":"https://gh/issues/1357","state":"OPEN",
				            "body":"x <!-- y -->",
				            "lastEditedAt":"2026-09-02T10:00:00Z",
				            "author":{"__typename":"Bot","login":"app"},
				            "authorAssociation":"CONTRIBUTOR",
				            "editor":{"login":"app"}}}
			]}}}}}`))
		}))
	t.Cleanup(srv.Close)
	github.SetBaseURL(srv.URL)
	t.Cleanup(func() { github.SetBaseURL("https://api.github.com") })
	testApp.githubClient = github.New(
		logging.NewNopLogger(),
		stubTok("tok"),
		testConfigJSON(t, map[string]string{"repo": "o/r"}),
	)

	resp, err := callProjectIssuesByStatus(t, 8, "Ready")
	require.NoError(t, err)
	assert.True(t, resp.Msg.Configured)
	require.Len(t, resp.Msg.Issues, 1)
	is := resp.Msg.Issues[0]
	assert.Equal(t, int64(1357), is.Number)
	assert.Equal(t, "Ready", is.Status)
	assert.Equal(t, "app[bot]", is.AuthorLogin)
	assert.Equal(t, "CONTRIBUTOR", is.AuthorAssociation)
	assert.Equal(t, "2026-09-01T10:00:00Z", is.StatusUpdatedAt)
	assert.True(t, is.BodyHasHtmlComment)
	assert.True(t, is.BodyEditedAfterStatus)
}

func TestObservabilityGetProjectIssuesByStatus_NotConfigured(t *testing.T) {
	promoteToAdmin(t)
	t.Cleanup(func() { demoteToUser(t) })
	testApp.githubClient = github.New(
		logging.NewNopLogger(),
		stubTok("tok"),
		configNotConnected(),
	)

	resp, err := callProjectIssuesByStatus(t, 8, "Ready")
	require.NoError(t, err)
	assert.False(t, resp.Msg.Configured)
	assert.Empty(t, resp.Msg.Issues)
}

func TestObservabilityGetProjectIssuesByStatus_UpstreamErrorsSurface(t *testing.T) {
	promoteToAdmin(t)
	t.Cleanup(func() { demoteToUser(t) })

	tests := []struct {
		name string
		body string
		code connect.Code
		msg  string
	}{
		{
			name: "missing scope",
			body: `{"errors":[{"type":"INSUFFICIENT_SCOPES",
				"message":"requires one of the following scopes: ['read:project']"}]}`,
			code: connect.CodeFailedPrecondition,
			msg:  "read:project",
		},
		{
			name: "other GraphQL error",
			body: `{"errors":[{"message":"Could not resolve to a User"}]}`,
			code: connect.CodeUnavailable,
			msg:  "Could not resolve to a User",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(
				func(w http.ResponseWriter, _ *http.Request) {
					w.Header().Set("Content-Type", "application/json")
					_, _ = w.Write([]byte(tt.body))
				}))
			t.Cleanup(srv.Close)
			github.SetBaseURL(srv.URL)
			t.Cleanup(func() { github.SetBaseURL("https://api.github.com") })
			testApp.githubClient = github.New(
				logging.NewNopLogger(),
				stubTok("tok"),
				testConfigJSON(t, map[string]string{"repo": "o/r"}),
			)

			_, err := callProjectIssuesByStatus(t, 9, "Backlog")
			require.Error(t, err)
			assert.Equal(t, tt.code, connect.CodeOf(err))
			assert.Contains(t, err.Error(), tt.msg)
		})
	}
}

func TestObservabilityGetProjectIssuesByStatus_NonAdmin(t *testing.T) {
	demoteToUser(t)
	_, err := callProjectIssuesByStatus(t, 8, "Ready")
	requirePermissionDenied(t, err)
}

func callProjectIssuesByStatus(
	t *testing.T, projectNumber int32, status string,
) (*connect.Response[observabilityv1.GetProjectIssuesByStatusResponse], error) {
	t.Helper()
	req := connect.NewRequest(&observabilityv1.GetProjectIssuesByStatusRequest{
		ProjectNumber: projectNumber,
		Status:        status,
	})
	setCookieOnRequest(req, accessToken)
	return observabilityClient(t).GetProjectIssuesByStatus(context.Background(), req)
}
