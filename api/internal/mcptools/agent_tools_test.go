package mcptools_test

import (
	"context"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"tools.xdoubleu.com/internal/constants"
	"tools.xdoubleu.com/internal/mcptools"
	"tools.xdoubleu.com/internal/models"
)

type noArgs struct{}

func callParams(name string) *mcp.CallToolParams {
	return &mcp.CallToolParams{
		Meta:           nil,
		Name:           name,
		Arguments:      nil,
		InputResponses: nil,
		RequestState:   "",
	}
}

func echoTool(
	context.Context, *mcp.CallToolRequest, noArgs,
) (*mcp.CallToolResult, any, error) {
	//nolint:exhaustruct // an empty result is enough
	return &mcp.CallToolResult{}, nil, nil
}

// withUser puts user, when set, on the context before the next handler.
func withUser(user *models.User) mcp.Middleware {
	return func(next mcp.MethodHandler) mcp.MethodHandler {
		return func(
			ctx context.Context, method string, req mcp.Request,
		) (mcp.Result, error) {
			if user != nil {
				ctx = context.WithValue(ctx, constants.UserContextKey, *user)
			}
			return next(ctx, method, req)
		}
	}
}

// connectAs serves a server with one factory tool and two others to a client
// whose calls carry the given user (none when nil).
func connectAs(t *testing.T, user *models.User) *mcp.ClientSession {
	t.Helper()
	ctx := context.Background()

	//nolint:exhaustruct // only Name/Version identify the test server
	srv := mcp.NewServer(&mcp.Implementation{Name: "test", Version: "0"}, nil)
	for _, name := range []string{"factory_list_tasks", "books_get_library", "get_logs"} {
		//nolint:exhaustruct // name and description are all a tool needs
		mcp.AddTool(srv, &mcp.Tool{Name: name, Description: name}, echoTool)
	}
	// withUser is added first, so it runs first and the filter sees the user.
	srv.AddReceivingMiddleware(withUser(user), mcptools.AgentToolsOnly())

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

func toolNames(t *testing.T, session *mcp.ClientSession) []string {
	t.Helper()
	list, err := session.ListTools(context.Background(), nil)
	require.NoError(t, err)
	names := []string{}
	for _, tool := range list.Tools {
		names = append(names, tool.Name)
	}
	return names
}

func user(role models.Role) *models.User {
	return &models.User{
		ID: "u", Email: "u@example.com", Role: role,
		AppAccess: nil, HasMFA: false, DisplayName: "",
	}
}

func TestAgentSeesAndCallsOnlyFactoryTools(t *testing.T) {
	session := connectAs(t, user(models.RoleAgent))
	ctx := context.Background()

	assert.Equal(t, []string{"factory_list_tasks"}, toolNames(t, session))

	_, err := session.CallTool(ctx, callParams("factory_list_tasks"))
	assert.NoError(t, err)
	for _, name := range []string{"books_get_library", "get_logs", "nonexistent"} {
		_, err = session.CallTool(ctx, callParams(name))
		require.Error(t, err, name)
		assert.Contains(t, err.Error(), "unknown tool", name)
	}
}

func TestAgentCannotReadResourcesOrPrompts(t *testing.T) {
	session := connectAs(t, user(models.RoleAgent))
	ctx := context.Background()

	_, err := session.ListResources(ctx, nil)
	assert.Error(t, err)
	_, err = session.ListPrompts(ctx, nil)
	assert.Error(t, err)
}

func TestOtherCallersKeepEveryTool(t *testing.T) {
	for name, u := range map[string]*models.User{
		"admin":   user(models.RoleAdmin),
		"user":    user(models.RoleUser),
		"service": user(models.RoleService),
		"nobody":  nil,
	} {
		session := connectAs(t, u)
		assert.ElementsMatch(t,
			[]string{"factory_list_tasks", "books_get_library", "get_logs"},
			toolNames(t, session), name)
		_, err := session.CallTool(context.Background(), callParams("books_get_library"))
		assert.NoError(t, err, name)
	}
}
