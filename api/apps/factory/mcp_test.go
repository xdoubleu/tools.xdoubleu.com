package factory_test

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"tools.xdoubleu.com/internal/constants"
	"tools.xdoubleu.com/internal/models"
)

const agentUserID = "9b7c3f0e-52d6-4f1c-8a58-1f4e2d7b6a11"

func asUser(role models.Role) *models.User {
	return &models.User{
		ID: agentUserID, Email: "agent@agent.invalid", Role: role,
		AppAccess: nil, HasMFA: false, DisplayName: "",
	}
}

// mcpSessionAs serves the factory's MCP tools to a client whose calls carry
// the given user.
func mcpSessionAs(t *testing.T, user *models.User) *mcp.ClientSession {
	t.Helper()
	ctx := context.Background()

	//nolint:exhaustruct // only Name/Version identify the test server
	srv := mcp.NewServer(&mcp.Implementation{Name: "test", Version: "0"}, nil)
	newApp(true, constants.ChannelBrowser).RegisterMCPTools(srv)
	srv.AddReceivingMiddleware(func(next mcp.MethodHandler) mcp.MethodHandler {
		return func(ctx context.Context, method string, req mcp.Request) (mcp.Result, error) {
			ctx = context.WithValue(ctx, constants.UserContextKey, *user)
			return next(ctx, method, req)
		}
	})

	serverTransport, clientTransport := mcp.NewInMemoryTransports()
	serverSession, err := srv.Connect(ctx, serverTransport, nil)
	require.NoError(t, err)
	t.Cleanup(func() { _ = serverSession.Close() })

	//nolint:exhaustruct // only Name/Version identify the test client
	client := mcp.NewClient(&mcp.Implementation{Name: "client", Version: "0"}, nil)
	session, err := client.Connect(ctx, clientTransport, nil)
	require.NoError(t, err)
	t.Cleanup(func() { _ = session.Close() })
	return session
}

func callTool(
	t *testing.T, session *mcp.ClientSession, name string, args map[string]any,
) (map[string]any, error) {
	t.Helper()
	//nolint:exhaustruct // name and arguments are all a call needs
	res, err := session.CallTool(context.Background(),
		&mcp.CallToolParams{Name: name, Arguments: args})
	if err != nil {
		return nil, err
	}
	if res.IsError {
		return nil, assert.AnError
	}
	text, ok := res.Content[0].(*mcp.TextContent)
	require.True(t, ok)
	var out map[string]any
	require.NoError(t, json.Unmarshal([]byte(text.Text), &out))
	return out, nil
}

// at follows keys (strings) and indexes (ints) down a decoded JSON value.
func at(t *testing.T, v any, path ...any) any {
	t.Helper()
	for _, step := range path {
		switch key := step.(type) {
		case string:
			m, ok := v.(map[string]any)
			require.True(t, ok, "%v is not an object at %q", v, key)
			v = m[key]
		case int:
			l, ok := v.([]any)
			require.True(t, ok, "%v is not a list at %d", v, key)
			require.Greater(t, len(l), key)
			v = l[key]
		}
	}
	return v
}

func str(t *testing.T, v any, path ...any) string {
	t.Helper()
	s, ok := at(t, v, path...).(string)
	require.True(t, ok, "not a string at %v", path)
	return s
}

func TestAgentSeesTheBoardTools(t *testing.T) {
	session := mcpSessionAs(t, asUser(models.RoleAgent))

	list, err := session.ListTools(context.Background(), nil)
	require.NoError(t, err)
	names := []string{}
	for _, tool := range list.Tools {
		names = append(names, tool.Name)
	}
	assert.ElementsMatch(t, []string{
		"factory_list_repos", "factory_list_tasks", "factory_get_task",
		"factory_create_task", "factory_comment_task", "factory_move_task",
	}, names)
}

func TestAgentFilesCommentsAndMovesATask(t *testing.T) {
	browserClient(t) // empties the board
	session := mcpSessionAs(t, asUser(models.RoleAgent))
	repoID := seededRepoID(t)

	created, err := callTool(t, session, "factory_create_task", map[string]any{
		"repo_id": repoID, "title": "  Fix flaky test ", "body": "details",
		"priority": "p1", "labels": []string{"bug"},
	})
	require.NoError(t, err)
	id := str(t, created, "task", "id")
	assert.Equal(t, "Fix flaky test", str(t, created, "task", "title"))
	assert.Equal(t, "TASK_STATUS_BACKLOG", str(t, created, "task", "status"))
	assert.Equal(t, "TASK_PRIORITY_P1", str(t, created, "task", "priority"))
	assert.Equal(t, "agent:"+agentUserID, str(t, created, "task", "createdBy"))
	assert.Equal(t, repoID, str(t, created, "task", "repoId"))

	_, err = callTool(t, session, "factory_comment_task",
		map[string]any{"task_id": id, "body": "On it"})
	require.NoError(t, err)

	moved, err := callTool(t, session, "factory_move_task",
		map[string]any{"id": id, "status": "in_progress"})
	require.NoError(t, err)
	assert.Equal(t, "TASK_STATUS_IN_PROGRESS", str(t, moved, "task", "status"))

	got, err := callTool(t, session, "factory_get_task", map[string]any{"id": id})
	require.NoError(t, err)
	assert.Equal(t, "details", str(t, got, "task", "body"))
	assert.Len(t, at(t, got, "comments"), 1)
	assert.Equal(t, "agent:"+agentUserID, str(t, got, "comments", 0, "author"))
	assert.Equal(t, "On it", str(t, got, "comments", 0, "body"))
}

func TestAgentCannotMarkATaskReady(t *testing.T) {
	browserClient(t)
	session := mcpSessionAs(t, asUser(models.RoleAgent))
	created, err := callTool(t, session, "factory_create_task", map[string]any{
		"repo_id": seededRepoID(t), "title": "needs a human",
	})
	require.NoError(t, err)
	id := str(t, created, "task", "id")

	_, err = callTool(t, session, "factory_move_task",
		map[string]any{"id": id, "status": "ready"})
	require.Error(t, err)

	var status string
	require.NoError(t, testDB.QueryRow(context.Background(),
		`SELECT status FROM factory.tasks WHERE id = $1`, id).Scan(&status))
	assert.Equal(t, "backlog", status)
}

func TestListTasksFiltersAndOmitsBodies(t *testing.T) {
	browserClient(t)
	session := mcpSessionAs(t, asUser(models.RoleAgent))
	repoID := seededRepoID(t)
	first, err := callTool(t, session, "factory_create_task", map[string]any{
		"repo_id": repoID, "title": "first", "body": "long body",
	})
	require.NoError(t, err)
	_, err = callTool(t, session, "factory_create_task",
		map[string]any{"repo_id": repoID, "title": "second"})
	require.NoError(t, err)
	_, err = callTool(t, session, "factory_move_task", map[string]any{
		"id": str(t, first, "task", "id"), "status": "done",
	})
	require.NoError(t, err)

	all, err := callTool(t, session, "factory_list_tasks", map[string]any{})
	require.NoError(t, err)
	assert.Len(t, at(t, all, "tasks"), 2)
	for i := range 2 {
		assert.Nil(t, at(t, all, "tasks", i, "body"))
	}

	done, err := callTool(t, session, "factory_list_tasks",
		map[string]any{"status": "done"})
	require.NoError(t, err)
	assert.Len(t, at(t, done, "tasks"), 1)
	assert.Equal(t, "first", str(t, done, "tasks", 0, "title"))

	other, err := callTool(t, session, "factory_list_tasks",
		map[string]any{"repo_id": "00000000-0000-0000-0000-000000000000"})
	require.NoError(t, err)
	assert.Nil(t, at(t, other, "tasks"))

	// The filter is by id, whatever its letter case; a bad id is an error, not
	// an empty list.
	same, err := callTool(t, session, "factory_list_tasks",
		map[string]any{"repo_id": strings.ToUpper(repoID)})
	require.NoError(t, err)
	assert.Len(t, at(t, same, "tasks"), 2)
	_, err = callTool(t, session, "factory_list_tasks",
		map[string]any{"repo_id": "not-a-uuid"})
	assert.Error(t, err)

	_, err = callTool(t, session, "factory_list_tasks",
		map[string]any{"status": "archived"})
	assert.Error(t, err)
}

func TestListReposGivesAgentsNoSetupCommandsOrInstructions(t *testing.T) {
	session := mcpSessionAs(t, asUser(models.RoleAgent))

	out, err := callTool(t, session, "factory_list_repos", map[string]any{})
	require.NoError(t, err)

	repos, ok := at(t, out, "repos").([]any)
	require.True(t, ok)
	found := false
	for i := range repos {
		if str(t, out, "repos", i, "name") != "tools.xdoubleu.com" {
			continue
		}
		found = true
		assert.Equal(t, seededRepoID(t), str(t, out, "repos", i, "id"))
		assert.Equal(t, "xdoubleu", str(t, out, "repos", i, "owner"))
		assert.Equal(t, "main", str(t, out, "repos", i, "defaultBranch"))
		assert.Nil(t, at(t, out, "repos", i, "agentInstructions"))
		assert.Nil(t, at(t, out, "repos", i, "setupCommands"))
	}
	assert.True(t, found)
}

func TestToolInputErrors(t *testing.T) {
	browserClient(t)
	session := mcpSessionAs(t, asUser(models.RoleAgent))
	repoID := seededRepoID(t)
	missing := "00000000-0000-0000-0000-000000000000"
	create := func(repo, title string) map[string]any {
		return map[string]any{"repo_id": repo, "title": title}
	}
	comment := func(id, body string) map[string]any {
		return map[string]any{"task_id": id, "body": body}
	}
	move := func(id, status string) map[string]any {
		return map[string]any{"id": id, "status": status}
	}

	for name, tc := range map[string]struct {
		tool string
		args map[string]any
	}{
		"create without a title":  {"factory_create_task", create(repoID, " ")},
		"create with a bad repo":  {"factory_create_task", create("nope", "x")},
		"create for unknown repo": {"factory_create_task", create(missing, "x")},
		"create with bad priority": {"factory_create_task", map[string]any{
			"repo_id": repoID, "title": "x", "priority": "urgent",
		}},
		"get a bad id":       {"factory_get_task", map[string]any{"id": "nope"}},
		"get an unknown id":  {"factory_get_task", map[string]any{"id": missing}},
		"comment a bad id":   {"factory_comment_task", comment("nope", "x")},
		"comment unknown":    {"factory_comment_task", comment(missing, "x")},
		"blank comment":      {"factory_comment_task", comment(missing, " ")},
		"move a bad id":      {"factory_move_task", move("nope", "done")},
		"move unknown":       {"factory_move_task", move(missing, "done")},
		"move to bad status": {"factory_move_task", move(missing, "later")},
	} {
		_, err := callTool(t, session, tc.tool, tc.args)
		assert.Error(t, err, name)
	}
}

func TestPeopleAreRefusedTheAgentTools(t *testing.T) {
	browserClient(t)
	agent := mcpSessionAs(t, asUser(models.RoleAgent))
	repoID := seededRepoID(t)
	created, err := callTool(t, agent, "factory_create_task",
		map[string]any{"repo_id": repoID, "title": "agent task"})
	require.NoError(t, err)
	id := str(t, created, "task", "id")

	// Every call is valid, so only the role can refuse it.
	for _, role := range []models.Role{
		models.RoleAdmin, models.RoleUser, models.RoleService,
	} {
		session := mcpSessionAs(t, asUser(role))
		for _, tc := range []struct {
			tool string
			args map[string]any
		}{
			{"factory_list_repos", map[string]any{}},
			{"factory_list_tasks", map[string]any{}},
			{"factory_get_task", map[string]any{"id": id}},
			{"factory_create_task", map[string]any{"repo_id": repoID, "title": "sneaky"}},
			{"factory_comment_task", map[string]any{"task_id": id, "body": "sneaky"}},
			{"factory_move_task", map[string]any{"id": id, "status": "done"}},
		} {
			_, callErr := callTool(t, session, tc.tool, tc.args)
			assert.Error(t, callErr, string(role)+" "+tc.tool)
		}
	}

	var tasks, comments int
	var status string
	require.NoError(t, testDB.QueryRow(context.Background(), `
		SELECT (SELECT count(*) FROM factory.tasks),
		       (SELECT count(*) FROM factory.task_comments),
		       (SELECT status FROM factory.tasks WHERE id = $1)`, id,
	).Scan(&tasks, &comments, &status))
	assert.Equal(t, 1, tasks)
	assert.Zero(t, comments)
	assert.Equal(t, "backlog", status)
}
