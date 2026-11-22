package mealplans_test

import (
	"context"
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
