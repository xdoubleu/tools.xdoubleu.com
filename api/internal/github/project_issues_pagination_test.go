package github_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"tools.xdoubleu.com/internal/github"
)

func decodeAfter(t *testing.T, r *http.Request) *string {
	t.Helper()
	var body struct {
		Variables struct {
			After *string `json:"after"`
		} `json:"variables"`
	}
	require.NoError(t, json.NewDecoder(r.Body).Decode(&body))
	return body.Variables.After
}

func TestListProjectIssuesByStatus_FollowsCursorAcrossPages(t *testing.T) {
	var afters []*string
	cleanup := buildServer(http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			after := decodeAfter(t, r)
			afters = append(afters, after)
			switch {
			case after == nil:
				_, _ = w.Write([]byte(`{"data":{"user":{"projectV2":{"items":{
					"pageInfo":{"hasNextPage":true,"endCursor":"c1"},
					"nodes":[{"status":{"name":"Backlog"},
					  "content":{"number":1,"state":"OPEN"}}]}}}}}`))
			case *after == "c1":
				_, _ = w.Write([]byte(`{"data":{"user":{"projectV2":{"items":{
					"pageInfo":{"hasNextPage":true,"endCursor":"c2"},
					"nodes":[{"status":{"name":"Ready"},
					  "content":{"number":2,"state":"OPEN"}}]}}}}}`))
			default:
				_, _ = w.Write([]byte(`{"data":{"user":{"projectV2":{"items":{
					"pageInfo":{"hasNextPage":false,"endCursor":"c3"},
					"nodes":[{"status":{"name":"Ready"},
					  "content":{"number":3,"state":"OPEN"}}]}}}}}`))
			}
		}))
	defer cleanup()

	issues, err := newClient().ListProjectIssuesByStatus(context.Background(), 8, "Ready")
	require.NoError(t, err)
	require.Len(t, issues, 2)
	assert.Equal(t, int64(2), issues[0].Number)
	assert.Equal(t, int64(3), issues[1].Number)

	require.Len(t, afters, 3)
	assert.Nil(t, afters[0])
	assert.Equal(t, "c1", *afters[1])
	assert.Equal(t, "c2", *afters[2])
}

func TestListProjectIssuesByStatus_PageCapErrors(t *testing.T) {
	calls := 0
	cleanup := buildServer(http.HandlerFunc(
		func(w http.ResponseWriter, _ *http.Request) {
			calls++
			w.Header().Set("Content-Type", "application/json")
			_, _ = fmt.Fprintf(w, `{"data":{"user":{"projectV2":{"items":{
				"pageInfo":{"hasNextPage":true,"endCursor":"c%d"},
				"nodes":[]}}}}}`, calls)
		}))
	defer cleanup()

	issues, err := newClient().ListProjectIssuesByStatus(context.Background(), 8, "Ready")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "more than")
	assert.Nil(t, issues)
	assert.Equal(t, 50, calls)
}

func TestListProjectIssuesByStatus_NullProjectErrors(t *testing.T) {
	cleanup := buildServer(http.HandlerFunc(
		func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"data":{"user":{"projectV2":null}}}`))
		}))
	defer cleanup()

	issues, err := newClient().ListProjectIssuesByStatus(context.Background(), 8, "Ready")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "project 8 not found")
	assert.Nil(t, issues)
}

func TestListProjectIssuesByStatus_InsufficientScopes(t *testing.T) {
	cleanup := buildServer(http.HandlerFunc(
		func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"data":{"user":{"projectV2":null}},
				"errors":[{"type":"INSUFFICIENT_SCOPES",
				"message":"requires one of the following scopes: ['read:project']"}]}`))
		}))
	defer cleanup()

	issues, err := newClient().ListProjectIssuesByStatus(context.Background(), 8, "Ready")
	require.ErrorIs(t, err, github.ErrInsufficientScopes)
	assert.Contains(t, err.Error(), "read:project")
	assert.Nil(t, issues)
}
