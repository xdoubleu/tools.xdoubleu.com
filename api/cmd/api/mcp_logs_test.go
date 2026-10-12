package main

import (
	"context"
	"testing"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAppsMCPGetLogs_ForwardsFilters(t *testing.T) {
	promoteToAdmin(t)
	t.Cleanup(func() { demoteToUser(t) })
	_, err := testApp.db.Exec(
		context.Background(), "DELETE FROM global.log_entries",
	)
	require.NoError(t, err)
	_, err = testApp.db.Exec(context.Background(), `
		INSERT INTO global.log_entries (occurred_at, source, level, message, attrs)
		VALUES
		  (now() - interval '2 hours', 'api', 'ERROR', 'kobo failed', NULL),
		  (now() - interval '1 minute', 'api', 'ERROR', 'kobo too new', NULL),
		  (now() - interval '2 hours', 'api', 'INFO', 'kobo quiet', NULL),
		  (now() - interval '2 hours', 'api', 'ERROR', 'metrics', NULL)
	`)
	require.NoError(t, err)

	session := appsMCPSession(t, accessToken.Value)
	//nolint:exhaustruct // only Name/Arguments are needed to call the tool
	res, err := session.CallTool(context.Background(), &mcp.CallToolParams{
		Name: "get_logs",
		Arguments: map[string]any{
			"min_level": "warn",
			"until":     time.Now().Add(-time.Hour).Format(time.RFC3339),
			"contains":  "KOBO",
		},
	})
	require.NoError(t, err)
	assert.False(t, res.IsError)
	msg := toolMessage(res, nil)
	assert.Contains(t, msg, "kobo failed")
	assert.NotContains(t, msg, "kobo too new")
	assert.NotContains(t, msg, "kobo quiet")
	assert.NotContains(t, msg, "metrics")
}

func TestAppsMCPGetLogs_InvalidUntil(t *testing.T) {
	promoteToAdmin(t)
	t.Cleanup(func() { demoteToUser(t) })

	session := appsMCPSession(t, accessToken.Value)
	//nolint:exhaustruct // only Name/Arguments are needed to call the tool
	res, err := session.CallTool(context.Background(), &mcp.CallToolParams{
		Name:      "get_logs",
		Arguments: map[string]any{"until": "not-a-time"},
	})
	assert.Contains(t, toolMessage(res, err), "until")
}
