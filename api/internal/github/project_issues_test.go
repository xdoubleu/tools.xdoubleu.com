package github_test

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"tools.xdoubleu.com/internal/github"
	"tools.xdoubleu.com/internal/logging"
)

func TestListProjectIssuesByStatus_FiltersByStatusAndOpenState(t *testing.T) {
	cleanup := buildServer(http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			require.Equal(t, "/graphql", r.URL.Path)
			require.Equal(t, http.MethodPost, r.Method)

			var body struct {
				Variables struct {
					Login  string `json:"login"`
					Number int    `json:"number"`
				} `json:"variables"`
			}
			require.NoError(t, json.NewDecoder(r.Body).Decode(&body))
			assert.Equal(t, "xdoubleu", body.Variables.Login)
			assert.Equal(t, 8, body.Variables.Number)

			_, _ = w.Write([]byte(`{"data":{"user":{"projectV2":{"items":{"nodes":[
				{"status":{"name":"Ready"},
				 "content":{"number":1357,"title":"Add MCP tool",
				            "url":"https://gh/issues/1357","state":"OPEN"}},
				{"status":{"name":"In progress"},
				 "content":{"number":1200,"title":"Other work",
				            "url":"https://gh/issues/1200","state":"OPEN"}},
				{"status":{"name":"Ready"},
				 "content":{"number":900,"title":"Already closed",
				            "url":"https://gh/issues/900","state":"CLOSED"}},
				{"status":{"name":"ready"},
				 "content":{"number":42,"title":"Case-insensitive match",
				            "url":"https://gh/issues/42","state":"OPEN"}}
			]}}}}}`))
		}))
	defer cleanup()

	issues, err := newClient().ListProjectIssuesByStatus(context.Background(), 8, "Ready")
	require.NoError(t, err)
	require.Len(t, issues, 2)
	assert.Equal(t, int64(1357), issues[0].Number)
	assert.Equal(t, "Add MCP tool", issues[0].Title)
	assert.Equal(t, "https://gh/issues/1357", issues[0].URL)
	assert.Equal(t, "Ready", issues[0].Status)
	assert.Equal(t, int64(42), issues[1].Number)
}

func TestListProjectIssuesByStatus_ApprovalFacts(t *testing.T) {
	cleanup := buildServer(http.HandlerFunc(
		func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"data":{"user":{"projectV2":{"items":{"nodes":[
				{"status":{"name":"Ready","updatedAt":"2026-09-01T10:00:00Z"},
				 "content":{"number":1,"state":"OPEN","body":"clean",
				            "lastEditedAt":null,
				            "author":{"__typename":"User","login":"xdoubleu"},
				            "authorAssociation":"OWNER","editor":null}},
				{"status":{"name":"Ready","updatedAt":"2026-09-01T10:00:00Z"},
				 "content":{"number":2,"state":"OPEN","body":"a <!-- hidden --> b",
				            "lastEditedAt":"2026-09-02T10:00:00Z",
				            "author":{"__typename":"Bot","login":"xdoubleu-routines"},
				            "authorAssociation":"CONTRIBUTOR",
				            "editor":{"login":"xdoubleu-routines"}}},
				{"status":{"name":"Ready","updatedAt":"2026-09-01T10:00:00Z"},
				 "content":{"number":3,"state":"OPEN","body":"owner edit",
				            "lastEditedAt":"2026-09-02T10:00:00Z",
				            "author":{"__typename":"Bot","login":"xdoubleu-routines"},
				            "authorAssociation":"CONTRIBUTOR",
				            "editor":{"login":"xdoubleu"}}},
				{"status":{"name":"Ready","updatedAt":"2026-09-03T10:00:00Z"},
				 "content":{"number":4,"state":"OPEN","body":"edited before Ready",
				            "lastEditedAt":"2026-09-02T10:00:00Z",
				            "author":{"__typename":"Bot","login":"xdoubleu-routines"},
				            "authorAssociation":"CONTRIBUTOR",
				            "editor":{"login":"xdoubleu-routines"}}}
			]}}}}}`))
		}))
	defer cleanup()

	issues, err := newClient().ListProjectIssuesByStatus(context.Background(), 8, "Ready")
	require.NoError(t, err)
	require.Len(t, issues, 4)

	assert.Equal(t, "xdoubleu", issues[0].AuthorLogin)
	assert.Equal(t, "OWNER", issues[0].AuthorAssociation)
	assert.Equal(t,
		time.Date(2026, 9, 1, 10, 0, 0, 0, time.UTC), issues[0].StatusUpdatedAt)
	assert.False(t, issues[0].BodyHasHTMLComment)
	assert.False(t, issues[0].BodyEditedAfterStatus)

	assert.Equal(t, "xdoubleu-routines[bot]", issues[1].AuthorLogin)
	assert.True(t, issues[1].BodyHasHTMLComment)
	assert.True(t, issues[1].BodyEditedAfterStatus)

	assert.False(t, issues[2].BodyEditedAfterStatus, "owner edits are approved")
	assert.False(t, issues[3].BodyEditedAfterStatus, "edit predates the Ready move")
}

func TestListProjectIssuesByStatus_NotConfigured(t *testing.T) {
	client := github.New(
		logging.NewNopLogger(),
		stubNotConnected(),
		configWithRepo(testRepo),
	)

	issues, err := client.ListProjectIssuesByStatus(context.Background(), 8, "Ready")
	require.ErrorIs(t, err, github.ErrNotConfigured)
	assert.Nil(t, issues)
}

func TestListProjectIssuesByStatus_GraphQLError(t *testing.T) {
	cleanup := buildServer(http.HandlerFunc(
		func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(
				`{"errors":[{"message":"Could not resolve to a User"}]}`,
			))
		}))
	defer cleanup()

	issues, err := newClient().ListProjectIssuesByStatus(context.Background(), 8, "Ready")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "Could not resolve to a User")
	assert.Nil(t, issues)
}

func TestListProjectIssuesByStatus_UpstreamError(t *testing.T) {
	cleanup := buildServer(http.HandlerFunc(
		func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
		}))
	defer cleanup()

	issues, err := newClient().ListProjectIssuesByStatus(context.Background(), 8, "Ready")
	require.Error(t, err)
	assert.Nil(t, issues)
}
