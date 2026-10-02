package main

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAppsMCPBooksGetReadingState_ExposesPosition(t *testing.T) {
	promoteToAdmin(t)
	t.Cleanup(func() { demoteToUser(t) })
	ctx := context.Background()
	var bookID string
	require.NoError(t, testApp.db.QueryRow(ctx, `
		INSERT INTO books.books (title) VALUES ('MCP reading state')
		RETURNING id
	`).Scan(&bookID))
	_, err := testApp.db.Exec(ctx, `
		INSERT INTO books.book_reading_state
		    (user_id, book_id, source, percent, position, read_at)
		VALUES ($1, $2, 'web', 37, '{"href": "OEBPS/ch3.xhtml", "offset": 120}',
		        '2026-09-30T08:15:42Z')
	`, testUserID, bookID)
	require.NoError(t, err)
	t.Cleanup(func() {
		_, _ = testApp.db.Exec(ctx, `DELETE FROM books.books WHERE id = $1`, bookID)
	})

	session := appsMCPSession(t, accessToken.Value)
	//nolint:exhaustruct // name and arguments identify the call
	res, err := session.CallTool(ctx, &mcp.CallToolParams{
		Name:      "books_get_reading_state",
		Arguments: map[string]any{"book_id": bookID},
	})
	require.NoError(t, err)
	require.False(t, res.IsError, toolMessage(res, nil))

	var out struct {
		State struct {
			Percent  int `json:"percent"`
			Position struct {
				Href   string `json:"href"`
				Offset int    `json:"offset"`
			} `json:"position"`
			ReadAt string `json:"readAt"`
		} `json:"state"`
	}
	require.NoError(t, json.Unmarshal([]byte(toolMessage(res, nil)), &out))
	assert.Equal(t, 37, out.State.Percent)
	assert.Equal(t, "OEBPS/ch3.xhtml", out.State.Position.Href)
	assert.Equal(t, 120, out.State.Position.Offset)
	assert.Equal(t, "2026-09-30T08:15:42Z", out.State.ReadAt)
}
