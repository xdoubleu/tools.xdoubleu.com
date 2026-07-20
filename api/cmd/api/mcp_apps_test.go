package main

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// appsToolNames lists every read-only tool the combined /apps/mcp server
// registers, grouped by app. Keep in sync with each apps/<app>/mcp.go.
//
//nolint:gochecknoglobals // shared expectations for the apps-MCP tests
var appsToolNames = []string{
	// games (5)
	"games_get_steam", "games_get_steam_game", "games_get_steam_distribution",
	"games_get_recently_active_games", "games_get_integrations",
	// reading (16)
	"reading_get_library", "reading_get_books_progress", "reading_search_library",
	"reading_search_external", "reading_get_external_book",
	"reading_get_reading_state", "reading_list_feeds",
	"reading_list_resync_proposals", "reading_get_book_sources",
	"reading_get_source_stats", "reading_list_books_in_exact_sources",
	"reading_find_duplicates", "reading_get_book_file", "reading_get_kepub_status",
	"reading_list_kobo_devices", "reading_get_kobo_device_logs",
	// recipes (3)
	"recipes_list_recipes", "recipes_get_recipe", "recipes_list_recipe_book_shares",
	// mealplans (3)
	"mealplans_list_plans", "mealplans_get_plan", "mealplans_suggest_recipes",
	// shoppinglist (10)
	"shoppinglist_get_custom_list", "shoppinglist_get_meal_plan_export_items",
	"shoppinglist_get_plan_ingredient_groups", "shoppinglist_list_categories",
	"shoppinglist_list_stores", "shoppinglist_get_store_categories",
	"shoppinglist_list_item_names", "shoppinglist_list_item_categories",
	"shoppinglist_list_shares", "shoppinglist_list_accessible_lists",
	// todos (4)
	"todos_list_tasks", "todos_get_task", "todos_search_tasks", "todos_get_settings",
	// icsproxy (3)
	"icsproxy_list_configs", "icsproxy_get_config", "icsproxy_preview_events",
}

// appsNetworkTools reach out to external providers, so the call tests skip them
// to stay hermetic.
//
//nolint:gochecknoglobals // shared expectations for the apps-MCP tests
var appsNetworkTools = map[string]bool{
	"reading_search_external":   true,
	"reading_get_external_book": true,
	"reading_get_book_sources":  true,
	"icsproxy_preview_events":   true,
}

func appsMCPSession(t *testing.T, token string) *mcp.ClientSession {
	t.Helper()
	ts := connectServer(t)

	transport := &mcp.StreamableClientTransport{
		Endpoint: ts.URL + appsMCPPath,
		HTTPClient: &http.Client{
			Transport: bearerRoundTripper{token: token, base: http.DefaultTransport},
		},
		MaxRetries:           -1,
		DisableStandaloneSSE: true,
		OAuthHandler:         nil,
	}

	//nolint:exhaustruct // only Name/Version identify the test client
	client := mcp.NewClient(&mcp.Implementation{Name: "test", Version: "0"}, nil)
	session, err := client.Connect(context.Background(), transport, nil)
	require.NoError(t, err)
	t.Cleanup(func() { _ = session.Close() })
	return session
}

// toolMessage returns the human-readable message of a tool call, whether the
// denial surfaced as a protocol error or an error result.
func toolMessage(res *mcp.CallToolResult, err error) string {
	if err != nil {
		return err.Error()
	}
	var b strings.Builder
	for _, c := range res.Content {
		if text, ok := c.(*mcp.TextContent); ok {
			b.WriteString(text.Text)
		}
	}
	return b.String()
}

func TestAppsMCPProtectedResourceMetadata(t *testing.T) {
	ts := connectServer(t)

	resp, err := ts.Client().Get(
		ts.URL + "/.well-known/oauth-protected-resource/apps/mcp",
	)
	require.NoError(t, err)
	defer func() { _ = resp.Body.Close() }()

	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var md struct {
		Resource             string   `json:"resource"`
		AuthorizationServers []string `json:"authorization_servers"`
	}
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&md))
	assert.Equal(t, testApp.config.APIURL+appsMCPPath, md.Resource)
	require.Len(t, md.AuthorizationServers, 1)
	assert.Equal(t, testApp.mcpAuthServerIssuer(), md.AuthorizationServers[0])
}

func TestAppsMCPUnauthenticated(t *testing.T) {
	ts := connectServer(t)

	req, err := http.NewRequestWithContext(
		context.Background(), http.MethodGet, ts.URL+appsMCPPath, nil,
	)
	require.NoError(t, err)
	resp, err := ts.Client().Do(req)
	require.NoError(t, err)
	defer func() { _ = resp.Body.Close() }()

	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
	assert.Contains(t, resp.Header.Get("WWW-Authenticate"), "resource_metadata")
}

func TestAppsMCPInvalidToken(t *testing.T) {
	ts := connectServer(t)

	req, err := http.NewRequestWithContext(
		context.Background(), http.MethodGet, ts.URL+appsMCPPath, nil,
	)
	require.NoError(t, err)
	req.Header.Set("Authorization", "Bearer not-a-real-token")
	resp, err := ts.Client().Do(req)
	require.NoError(t, err)
	defer func() { _ = resp.Body.Close() }()

	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
}

func TestAppsMCPListToolsAsAdmin(t *testing.T) {
	promoteToAdmin(t)
	t.Cleanup(func() { demoteToUser(t) })

	session := appsMCPSession(t, accessToken.Value)
	res, err := session.ListTools(context.Background(), nil)
	require.NoError(t, err)

	got := make(map[string]bool, len(res.Tools))
	for _, tool := range res.Tools {
		got[tool.Name] = true
	}
	assert.Len(t, res.Tools, len(appsToolNames))
	for _, name := range appsToolNames {
		assert.Truef(t, got[name], "missing tool %s", name)
	}
}

// TestAppsMCPReadToolsReturnData calls a hermetic subset of list tools as an
// admin and asserts each returns text content without an error — proving the
// read handlers are wired through to the tools.
func TestAppsMCPReadToolsReturnData(t *testing.T) {
	promoteToAdmin(t)
	t.Cleanup(func() { demoteToUser(t) })

	session := appsMCPSession(t, accessToken.Value)
	tools := []string{
		"games_get_recently_active_games", "reading_get_library",
		"reading_list_feeds", "recipes_list_recipes", "mealplans_list_plans",
		"shoppinglist_list_accessible_lists", "todos_list_tasks",
		"icsproxy_list_configs",
	}
	for _, name := range tools {
		//nolint:exhaustruct // only the tool name is required to call it
		res, err := session.CallTool(context.Background(), &mcp.CallToolParams{
			Name: name,
		})
		require.NoErrorf(t, err, "tool %s", name)
		assert.Falsef(t, res.IsError, "tool %s returned an error result", name)
		require.Lenf(t, res.Content, 1, "tool %s", name)
		_, ok := res.Content[0].(*mcp.TextContent)
		assert.Truef(t, ok, "tool %s did not return text content", name)
	}
}

// TestAppsMCPCallAllToolsAsAdmin exercises every (hermetic) tool as an admin.
// Some calls return an error result for the dummy ids — that is fine; the point
// is that the per-app access gate never denies an admin, and every producer
// runs.
func TestAppsMCPCallAllToolsAsAdmin(t *testing.T) {
	promoteToAdmin(t)
	t.Cleanup(func() { demoteToUser(t) })

	uid := uuid.NewString()
	args := map[string]any{
		"games_get_steam_game":      map[string]any{"game_id": 1},
		"reading_search_library":    map[string]any{"query": "x"},
		"reading_get_reading_state": map[string]any{"book_id": uid},
		"reading_get_book_file": map[string]any{
			"book_id": uid,
			"format":  "epub",
		},
		"reading_get_kepub_status":     map[string]any{"book_id": uid},
		"reading_get_kobo_device_logs": map[string]any{"id": uid},
		"reading_list_books_in_exact_sources": map[string]any{
			"sources": []string{"unicat"},
		},
		"recipes_get_recipe": map[string]any{"id": uid},
		"mealplans_get_plan": map[string]any{"id": uid},
		"mealplans_suggest_recipes": map[string]any{
			"plan_id":   uid,
			"meal_date": "2026-01-01",
			"meal_slot": "noon",
		},
		"shoppinglist_get_meal_plan_export_items": map[string]any{"plan_id": uid},
		"shoppinglist_get_plan_ingredient_groups": map[string]any{"plan_id": uid},
		"shoppinglist_get_store_categories":       map[string]any{"store_id": uid},
		"todos_get_task":                          map[string]any{"id": uid},
		"todos_search_tasks":                      map[string]any{"query": "x"},
		"icsproxy_get_config":                     map[string]any{"token": "x"},
	}

	session := appsMCPSession(t, accessToken.Value)
	for _, name := range appsToolNames {
		if appsNetworkTools[name] {
			continue
		}
		//nolint:exhaustruct // name + optional arguments are all a call needs
		res, err := session.CallTool(context.Background(), &mcp.CallToolParams{
			Name:      name,
			Arguments: args[name],
		})
		assert.NotContainsf(t, strings.ToLower(toolMessage(res, err)),
			"access to", "admin was denied tool %s", name)
	}
}

// TestAppsMCPAccessGate is the #382 constraint: a non-admin sees only the apps
// they have access to. A user granted just "games" can call games tools but is
// denied another app's tools.
func TestAppsMCPAccessGate(t *testing.T) {
	ctx := context.Background()
	require.NoError(t,
		testApp.appUsersRepo.Upsert(ctx, testUserID, "user@example.com"))
	demoteToUser(t)
	grantAppAccess(t, testUserID, "games")
	t.Cleanup(func() { revokeAppAccess(t, testUserID, "games") })

	session := appsMCPSession(t, accessToken.Value)

	//nolint:exhaustruct // only the tool name is required to call it
	allowed, err := session.CallTool(ctx, &mcp.CallToolParams{
		Name: "games_get_recently_active_games",
	})
	require.NoError(t, err)
	assert.False(t, allowed.IsError, "user with games access was denied")

	for _, name := range []string{"reading_get_library", "todos_list_tasks"} {
		//nolint:exhaustruct // only the tool name is required to call it
		res, callErr := session.CallTool(ctx, &mcp.CallToolParams{Name: name})
		assert.Containsf(t, strings.ToLower(toolMessage(res, callErr)),
			"access to", "expected %s to be denied", name)
	}
}
