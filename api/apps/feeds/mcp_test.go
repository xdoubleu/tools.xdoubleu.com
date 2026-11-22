package feeds_test

import (
	"context"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"tools.xdoubleu.com/internal/kit/constants"
	sharedmodels "tools.xdoubleu.com/internal/models"
)

func newMCPSession(t *testing.T) *mcp.ClientSession {
	t.Helper()
	ctx := context.Background()

	//nolint:exhaustruct // only Name/Version identify the test server
	srv := mcp.NewServer(&mcp.Implementation{Name: "test", Version: "0"}, nil)
	testApp.RegisterMCPTools(srv)
	srv.AddReceivingMiddleware(func(next mcp.MethodHandler) mcp.MethodHandler {
		return func(
			ctx context.Context, method string, req mcp.Request,
		) (mcp.Result, error) {
			ctx = context.WithValue(
				ctx,
				constants.UserContextKey,
				//nolint:exhaustruct // only ID matters to the feed tools
				sharedmodels.User{ID: userID},
			)
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

func callFeedsTool(
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

// TestMCPTools_ReadTools exercises every feeds tool against a real app.
func TestMCPTools_ReadTools(t *testing.T) {
	session := newMCPSession(t)

	for _, tc := range []struct {
		name string
		args map[string]any
	}{
		{"feeds_list_feeds", nil},
		{"feeds_list_items", nil},
		// A feed_id matching nothing still succeeds, proving the arg is
		// forwarded.
		{"feeds_list_items", map[string]any{
			"feed_id": "00000000-0000-0000-0000-000000000000",
		}},
		{"feeds_list_items", map[string]any{"filtered_only": true}},
		{"feeds_list_filter_rules", nil},
		{"feeds_get_filter_rule_suggestions", nil},
	} {
		res := callFeedsTool(t, session, tc.name, tc.args)
		assert.False(t, res.IsError, tc.name)
	}
}

func TestMCPTools_GetItemUnknownIsAnError(t *testing.T) {
	session := newMCPSession(t)

	res := callFeedsTool(t, session, "feeds_get_item", map[string]any{
		"item_id": "00000000-0000-0000-0000-000000000000",
	})
	assert.True(t, res.IsError)
}
