package books_test

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"tools.xdoubleu.com/internal/kit/constants"
	"tools.xdoubleu.com/internal/models"
)

func newBooksMCPSession(t *testing.T, role models.Role) *mcp.ClientSession {
	t.Helper()
	ctx := context.Background()

	//nolint:exhaustruct // only Name/Version identify the test server
	srv := mcp.NewServer(&mcp.Implementation{Name: "test", Version: "0"}, nil)
	testApp.RegisterMCPTools(srv)
	user := models.User{
		ID: userID, Email: "u@example.com", Role: role,
		AppAccess: nil, WriteAccess: nil, PrincipalID: "", HasMFA: false,
		DisplayName: "",
	}
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

func callBooksTool(
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

func TestMCPCatalogTools_AdminReadsCatalog(t *testing.T) {
	session := newBooksMCPSession(t, models.RoleAdmin)

	calls := map[string]map[string]any{
		"books_list_resync_proposals":       {},
		"books_get_source_stats":            {},
		"books_list_books_in_exact_sources": {"sources": []string{"unicat"}},
		"books_find_duplicates":             {},
	}
	for name, args := range calls {
		res := callBooksTool(t, session, name, args)
		assert.False(t, res.IsError, name)
	}
}

func TestMCPCatalogTools_NonAdminDenied(t *testing.T) {
	session := newBooksMCPSession(t, models.RoleUser)

	calls := map[string]map[string]any{
		"books_list_resync_proposals":       {},
		"books_get_source_stats":            {},
		"books_list_books_in_exact_sources": {"sources": []string{"unicat"}},
		"books_find_duplicates":             {},
		"books_get_book_sources":            {"book_id": uuid.NewString()},
	}
	for name, args := range calls {
		res := callBooksTool(t, session, name, args)
		assert.True(t, res.IsError, name)
	}
}

func TestMCPGetBookSources_AdminInvalidAndUnknownBook(t *testing.T) {
	session := newBooksMCPSession(t, models.RoleAdmin)

	res := callBooksTool(t, session, "books_get_book_sources",
		map[string]any{"book_id": "not-a-uuid"})
	assert.True(t, res.IsError)

	res = callBooksTool(t, session, "books_get_book_sources",
		map[string]any{"book_id": uuid.NewString()})
	assert.True(t, res.IsError)
}
