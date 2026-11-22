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
	user := models.User{
		ID: userID, Email: "u@example.com", Role: role,
		AppAccess: []string{"books"}, WriteAccess: nil, PrincipalID: "", HasMFA: false,
		DisplayName: "",
	}
	return newBooksMCPSessionAs(t, &user)
}

func newBooksMCPSessionAs(t *testing.T, user *models.User) *mcp.ClientSession {
	t.Helper()
	return newBooksMCPSessionCtx(t, user, false)
}

// newBooksMCPSessionCtx optionally hands handlers an already-canceled context
// so every service call fails.
func newBooksMCPSessionCtx(
	t *testing.T, user *models.User, canceled bool,
) *mcp.ClientSession {
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
			if canceled && method == "tools/call" {
				var cancel context.CancelFunc
				ctx, cancel = context.WithCancel(ctx)
				cancel()
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

func TestMCPLibraryTools_ReadOwnData(t *testing.T) {
	session := newBooksMCPSession(t, models.RoleUser)

	calls := map[string]map[string]any{
		"books_get_library":           {},
		"books_get_books_progress":    {"date_start": "2024-01-01", "date_end": "2024-02-01"},
		"books_search_library":        {"query": "anything"},
		"books_search_external":       {"query": ""},
		"books_get_reading_state":     {"book_id": uuid.NewString()},
		"books_list_kobo_devices":     {},
		"books_list_kobo_store_books": {},
	}
	for name, args := range calls {
		res := callBooksTool(t, session, name, args)
		assert.False(t, res.IsError, "%s: %s", name, toolText(res))
	}

	res := callBooksTool(t, session, "books_search_library", map[string]any{"query": ""})
	assert.False(t, res.IsError)
}

func TestMCPLibraryTools_RejectBadInput(t *testing.T) {
	session := newBooksMCPSession(t, models.RoleUser)

	calls := map[string]map[string]any{
		"books_get_series":           {"name": "  "},
		"books_get_reading_state":    {"book_id": "not-a-uuid"},
		"books_get_book_file":        {"book_id": "not-a-uuid"},
		"books_get_kepub_status":     {"book_id": "not-a-uuid"},
		"books_get_kobo_device_logs": {"id": "not-a-uuid"},
	}
	for name, args := range calls {
		res := callBooksTool(t, session, name, args)
		assert.True(t, res.IsError, name)
	}

	for _, name := range []string{
		"books_get_kepub_status", "books_get_book_file",
	} {
		res := callBooksTool(t, session, name, map[string]any{"book_id": uuid.NewString()})
		assert.True(t, res.IsError, name)
	}
	res := callBooksTool(t, session, "books_get_series", map[string]any{"name": "Nope"})
	assert.False(t, res.IsError, toolText(res))

	res = callBooksTool(t, session, "books_get_kobo_device_logs",
		map[string]any{"id": uuid.NewString()})
	assert.True(t, res.IsError)
	res = callBooksTool(t, session, "books_get_external_book",
		map[string]any{"provider": "nope", "provider_id": "1"})
	assert.True(t, res.IsError)
}

func toolText(res *mcp.CallToolResult) string {
	out := ""
	for _, c := range res.Content {
		if tc, ok := c.(*mcp.TextContent); ok {
			out += tc.Text
		}
	}
	return out
}

func TestMCPLibraryTools_RequireUser(t *testing.T) {
	session := newBooksMCPSessionAs(t, nil)
	id := uuid.NewString()

	calls := map[string]map[string]any{
		"books_get_library":           {},
		"books_get_books_progress":    {},
		"books_search_library":        {"query": "x"},
		"books_search_external":       {"query": "x"},
		"books_get_external_book":     {"provider": "unicat", "provider_id": "1"},
		"books_get_series":            {"name": "x"},
		"books_get_reading_state":     {"book_id": id},
		"books_get_book_file":         {"book_id": id},
		"books_get_kepub_status":      {"book_id": id},
		"books_list_kobo_devices":     {},
		"books_list_kobo_store_books": {},
		"books_get_kobo_device_logs":  {"id": id},
	}
	for name, args := range calls {
		res := callBooksTool(t, session, name, args)
		assert.True(t, res.IsError, name)
	}
}

func TestMCPLibraryTools_OwnFileAndDevice(t *testing.T) {
	session := newBooksMCPSession(t, models.RoleUser)

	_, bookID := uploadFileForOwner(t, userID, "pdf")
	for _, name := range []string{"books_get_book_file", "books_get_kepub_status"} {
		res := callBooksTool(t, session, name, map[string]any{"book_id": bookID.String()})
		assert.False(t, res.IsError, "%s: %s", name, toolText(res))
	}

	_, deviceID := registerDeviceReturningID(t, userID)
	res := callBooksTool(t, session, "books_get_kobo_device_logs",
		map[string]any{"id": deviceID})
	assert.False(t, res.IsError, toolText(res))

	res = callBooksTool(t, session, "books_search_external",
		map[string]any{"query": "dune"})
	assert.False(t, res.IsError, toolText(res))
}

func TestMCPTools_ServiceErrorsSurface(t *testing.T) {
	user := models.User{
		ID: userID, Email: "u@example.com", Role: models.RoleAdmin,
		AppAccess: []string{"books"}, WriteAccess: nil, PrincipalID: "", HasMFA: false,
		DisplayName: "",
	}
	session := newBooksMCPSessionCtx(t, &user, true)
	id := uuid.NewString()

	calls := map[string]map[string]any{
		"books_get_library":                 {},
		"books_get_books_progress":          {},
		"books_search_library":              {"query": "x"},
		"books_get_series":                  {"name": "x"},
		"books_get_external_book":           {"provider": "nope", "provider_id": "1"},
		"books_get_reading_state":           {"book_id": id},
		"books_list_resync_proposals":       {},
		"books_get_book_sources":            {"book_id": id},
		"books_get_source_stats":            {},
		"books_list_books_in_exact_sources": {"sources": []string{"unicat"}},
		"books_find_duplicates":             {},
		"books_get_book_file":               {"book_id": id},
		"books_get_kepub_status":            {"book_id": id},
		"books_list_kobo_devices":           {},
		"books_list_kobo_store_books":       {},
		"books_get_kobo_device_logs":        {"id": id},
	}
	for name, args := range calls {
		res := callBooksTool(t, session, name, args)
		assert.True(t, res.IsError, name)
	}
}
