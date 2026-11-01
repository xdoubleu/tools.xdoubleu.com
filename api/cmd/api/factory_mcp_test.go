package main

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"tools.xdoubleu.com/internal/models"
)

func callAgentTool(
	t *testing.T, session *mcp.ClientSession, name string, args map[string]any,
) (map[string]any, error) {
	t.Helper()
	res, err := session.CallTool(context.Background(), toolCallWith(name, args))
	if err != nil {
		return nil, err
	}
	text := toolMessage(res, nil)
	if res.IsError {
		return nil, assert.AnError
	}
	var out map[string]any
	require.NoError(t, json.Unmarshal([]byte(text), &out))
	return out, nil
}

// str reads a string out of a decoded tool result.
func str(t *testing.T, m map[string]any, keys ...string) string {
	t.Helper()
	var v any = m
	for _, k := range keys {
		obj, ok := v.(map[string]any)
		require.True(t, ok, "not an object at %q", k)
		v = obj[k]
	}
	s, ok := v.(string)
	require.True(t, ok, "not a string at %v", keys)
	return s
}

func toolCallWith(name string, args map[string]any) *mcp.CallToolParams {
	return &mcp.CallToolParams{
		Meta:           nil,
		Name:           name,
		Arguments:      args,
		InputResponses: nil,
		RequestState:   "",
	}
}

func toolNames(t *testing.T, session *mcp.ClientSession) []string {
	t.Helper()
	tools, err := session.ListTools(context.Background(), nil)
	require.NoError(t, err)
	names := []string{}
	for _, tool := range tools.Tools {
		names = append(names, tool.Name)
	}
	return names
}

// TestFactoryMCP_AgentWorksTheBoardButCannotMarkReady drives the tools through
// a real agent account: the token endpoint, /apps/mcp and the board.
func TestFactoryMCP_AgentWorksTheBoardButCannotMarkReady(t *testing.T) {
	admin := newAgentAdmin(t, models.RoleAdmin)
	created := createAgent(t, admin, "board agent")
	token, status := agentToken(t, created.Account.ClientId, created.ClientSecret)
	require.Equal(t, 200, status)
	session := appsMCPSession(t, token)

	assert.ElementsMatch(t, factoryToolNames, toolNames(t, session))

	var repoID string
	require.NoError(t, testApp.db.QueryRow(context.Background(),
		`SELECT id::text FROM factory.repos ORDER BY created_at LIMIT 1`,
	).Scan(&repoID))

	filed, err := callAgentTool(t, session, "factory_create_task", map[string]any{
		"repo_id": repoID, "title": "filed by an agent",
	})
	require.NoError(t, err)
	id := str(t, filed, "task", "id")
	assert.Equal(t, "TASK_STATUS_BACKLOG", str(t, filed, "task", "status"))
	assert.Equal(t, "agent:"+created.Account.UserId, str(t, filed, "task", "createdBy"))

	_, err = callAgentTool(t, session, "factory_comment_task",
		map[string]any{"task_id": id, "body": "first note"})
	require.NoError(t, err)

	_, err = callAgentTool(t, session, "factory_move_task",
		map[string]any{"id": id, "status": "ready"})
	require.Error(t, err, "only a person in a browser marks a task Ready")

	moved, err := callAgentTool(t, session, "factory_move_task",
		map[string]any{"id": id, "status": "in_review"})
	require.NoError(t, err)
	assert.Equal(t, "TASK_STATUS_IN_REVIEW", str(t, moved, "task", "status"))

	var author string
	require.NoError(t, testApp.db.QueryRow(context.Background(),
		`SELECT author FROM factory.task_comments WHERE task_id = $1`, id,
	).Scan(&author))
	assert.Equal(t, "agent:"+created.Account.UserId, author)
}

// TestFactoryMCP_PeopleGetNoBoardTools: a session signed in as a person, even
// an admin, neither sees the tools nor can call them.
func TestFactoryMCP_PeopleGetNoBoardTools(t *testing.T) {
	adminToken := freshTestUser(t)
	user, err := testApp.auth.ResolveToken(context.Background(), adminToken)
	require.NoError(t, err)
	require.NoError(
		t,
		testApp.appUsersRepo.Upsert(context.Background(), user.ID, user.Email),
	)
	require.NoError(
		t,
		testApp.appUsersRepo.SetRole(context.Background(), user.ID, models.RoleAdmin),
	)
	testApp.auth.InvalidateUserCache()
	session := appsMCPSession(t, adminToken)

	for _, name := range toolNames(t, session) {
		assert.NotContains(t, name, "factory_")
	}
	_, err = callAgentTool(t, session, "factory_list_tasks", map[string]any{})
	assert.Error(t, err)
}
