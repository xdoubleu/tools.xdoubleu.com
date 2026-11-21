package shoppinglist_test

import (
	"context"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	shoppinglistv1 "tools.xdoubleu.com/gen/shoppinglist/v1"
	"tools.xdoubleu.com/internal/kit/constants"
	"tools.xdoubleu.com/internal/models"
)

func newMCPSession(t *testing.T, user *models.User) *mcp.ClientSession {
	t.Helper()
	ctx := context.Background()

	//nolint:exhaustruct // only Name/Version identify the test server
	srv := mcp.NewServer(&mcp.Implementation{Name: "test", Version: "0"}, nil)
	testApp.RegisterMCPTools(srv)
	srv.AddReceivingMiddleware(func(next mcp.MethodHandler) mcp.MethodHandler {
		return func(
			ctx context.Context, method string, req mcp.Request,
		) (mcp.Result, error) {
			if user != nil {
				ctx = context.WithValue(ctx, constants.UserContextKey, *user)
			}
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
) *mcp.CallToolResult {
	t.Helper()
	res, err := session.CallTool(
		t.Context(),
		//nolint:exhaustruct // name and arguments are all a call needs
		&mcp.CallToolParams{Name: name, Arguments: args},
	)
	require.NoError(t, err, name)
	return res
}

func TestMCPReadTools(t *testing.T) {
	client := newShoppingClient(t)
	_, err := client.CreateShoppingItem(
		t.Context(),
		&shoppinglistv1.CreateShoppingItemRequest{Name: "Milk", Amount: "2", Unit: "L"},
	)
	require.NoError(t, err)
	category := createCategory(t, client, "MCP dairy")
	store := createStore(t, client, "MCP store")
	planID := createTestPlan(t, "MCP plan")
	t.Cleanup(func() { deletePlan(t, planID) })

	user := &models.User{
		ID: userID, Email: "u@example.com", Role: models.RoleAdmin,
		AppAccess: nil, WriteAccess: nil, PrincipalID: "", HasMFA: false,
		DisplayName: "",
	}
	session := newMCPSession(t, user)

	calls := map[string]map[string]any{
		"shoppinglist_get_custom_list":            {},
		"shoppinglist_get_meal_plan_export_items": {"plan_id": planID.String()},
		"shoppinglist_get_plan_ingredient_groups": {"plan_id": planID.String()},
		"shoppinglist_list_categories":            {},
		"shoppinglist_list_stores":                {},
		"shoppinglist_get_store_categories":       {"store_id": store.Id},
		"shoppinglist_list_item_names":            {},
		"shoppinglist_list_item_categories":       {},
		"shoppinglist_list_ingredient_units":      {},
	}
	for name, args := range calls {
		assert.False(t, callTool(t, session, name, args).IsError, name)
	}

	res := callTool(t, session, "shoppinglist_list_categories", map[string]any{})
	text, ok := res.Content[0].(*mcp.TextContent)
	require.True(t, ok)
	assert.Contains(t, text.Text, category.Name)
}

func TestMCPReadToolsRejectBadInput(t *testing.T) {
	user := &models.User{
		ID: userID, Email: "u@example.com", Role: models.RoleAdmin,
		AppAccess: nil, WriteAccess: nil, PrincipalID: "", HasMFA: false,
		DisplayName: "",
	}
	session := newMCPSession(t, user)
	for name, args := range map[string]map[string]any{
		"shoppinglist_get_meal_plan_export_items": {"plan_id": "x"},
		"shoppinglist_get_plan_ingredient_groups": {"plan_id": "x"},
		"shoppinglist_get_store_categories":       {"store_id": "x"},
	} {
		assert.True(t, callTool(t, session, name, args).IsError, name)
	}
}

func TestMCPReadToolsNeedAppAccess(t *testing.T) {
	session := newMCPSession(t, nil)
	res := callTool(t, session, "shoppinglist_list_stores", map[string]any{})
	assert.True(t, res.IsError)
}
