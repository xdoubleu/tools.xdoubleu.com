package mealplans_test

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/google/uuid"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"tools.xdoubleu.com/internal/kit/constants"
	sharedmodels "tools.xdoubleu.com/internal/models"
)

func newMCPSession(t *testing.T, user sharedmodels.User) *mcp.ClientSession {
	t.Helper()
	ctx := context.Background()

	//nolint:exhaustruct // only Name/Version identify the test server
	srv := mcp.NewServer(&mcp.Implementation{Name: "test", Version: "0"}, nil)
	testApp.RegisterMCPTools(srv)
	srv.AddReceivingMiddleware(func(next mcp.MethodHandler) mcp.MethodHandler {
		return func(
			ctx context.Context, method string, req mcp.Request,
		) (mcp.Result, error) {
			return next(context.WithValue(ctx, constants.UserContextKey, user), method, req)
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

func TestMCPListPlans_CreatesDefaultPlanForNewUser(t *testing.T) {
	//nolint:exhaustruct // only ID and role matter
	user := sharedmodels.User{
		ID:   "mcp-user-" + uuid.NewString(),
		Role: sharedmodels.RoleAdmin,
	}
	session := newMCPSession(t, user)

	res, err := session.CallTool(
		t.Context(),
		//nolint:exhaustruct // the tool takes no arguments
		&mcp.CallToolParams{Name: "mealplans_list_plans"},
	)
	require.NoError(t, err)
	require.False(t, res.IsError)
	require.Len(t, res.Content, 1)
	text, ok := res.Content[0].(*mcp.TextContent)
	require.True(t, ok)
	assert.Contains(t, text.Text, "My Meal Plan")
}

func TestMCPPlanTools(t *testing.T) {
	//nolint:exhaustruct // only ID and role matter
	user := sharedmodels.User{
		ID:   "mcp-user-" + uuid.NewString(),
		Role: sharedmodels.RoleAdmin,
	}
	session := newMCPSession(t, user)
	call := func(name string, args map[string]any) *mcp.CallToolResult {
		res, err := session.CallTool(
			t.Context(),
			//nolint:exhaustruct // only name and arguments matter
			&mcp.CallToolParams{Name: name, Arguments: args},
		)
		require.NoError(t, err)
		return res
	}
	text := func(res *mcp.CallToolResult) string {
		require.NotEmpty(t, res.Content)
		tc, ok := res.Content[0].(*mcp.TextContent)
		require.True(t, ok)
		return tc.Text
	}

	list := call("mealplans_list_plans", nil)
	require.False(t, list.IsError)
	var parsed struct {
		Plans []struct {
			ID string `json:"id"`
		} `json:"plans"`
	}
	require.NoError(t, json.Unmarshal([]byte(text(list)), &parsed))
	require.NotEmpty(t, parsed.Plans)
	planID := parsed.Plans[0].ID

	got := call("mealplans_get_plan", map[string]any{"id": planID})
	require.False(t, got.IsError)
	assert.Contains(t, text(got), planID)
	assert.Contains(t, text(got), "isOwner")

	assert.True(t, call("mealplans_get_plan", map[string]any{"id": "nope"}).IsError)

	sug := call("mealplans_suggest_recipes", map[string]any{
		"plan_id": planID, "meal_date": "2026-01-05", "meal_slot": "evening",
	})
	require.False(t, sug.IsError)

	assert.True(t, call("mealplans_suggest_recipes", map[string]any{
		"plan_id": "nope", "meal_date": "2026-01-05", "meal_slot": "evening",
	}).IsError)
	assert.True(t, call("mealplans_suggest_recipes", map[string]any{
		"plan_id": planID, "meal_date": "bad", "meal_slot": "evening",
	}).IsError)
}
