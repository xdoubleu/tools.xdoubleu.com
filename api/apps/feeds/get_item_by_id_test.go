package feeds_test

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"tools.xdoubleu.com/internal/database"
)

func seedFeedItem(t *testing.T, owner, title string) uuid.UUID {
	t.Helper()
	ctx := context.Background()

	var feedID uuid.UUID
	err := testDB.QueryRow(ctx, `
		INSERT INTO feeds.feeds (id, user_id, url, title, source_type)
		VALUES (gen_random_uuid(), $1, $2, $3, 'rss')
		RETURNING id
	`, owner, "https://example.com/"+title, title+" Feed").Scan(&feedID)
	require.NoError(t, err)

	var itemID uuid.UUID
	err = testDB.QueryRow(ctx, `
		INSERT INTO feeds.items (feed_id, guid, title, source_url, published_at)
		VALUES ($1, $2, $3, $4, now())
		RETURNING id
	`, feedID, title+"-guid", title, "https://example.com/"+title+"/article").
		Scan(&itemID)
	require.NoError(t, err)

	return itemID
}

// TestGetItemByID_Owner confirms GetItemByID — the exported method the
// learningpaths app (#1474) uses to resolve a resource linked to a feeds
// item — returns the caller's own item.
func TestGetItemByID_Owner(t *testing.T) {
	itemID := seedFeedItem(t, userID, "GetItemByID Owner")

	result, err := testApp.GetItemByID(context.Background(), userID, itemID)
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, "GetItemByID Owner", result.Title)
}

// TestGetItemByID_OtherUserNotFound confirms an item that exists but
// belongs to a different user's feed 404s rather than leaking its
// existence or data.
func TestGetItemByID_OtherUserNotFound(t *testing.T) {
	itemID := seedFeedItem(t, "a-different-user", "GetItemByID Foreign")

	_, err := testApp.GetItemByID(context.Background(), userID, itemID)
	assert.ErrorIs(t, err, database.ErrResourceNotFound)
}

// TestGetItemByID_MissingNotFound confirms an item ID that doesn't exist at
// all also 404s.
func TestGetItemByID_MissingNotFound(t *testing.T) {
	_, err := testApp.GetItemByID(context.Background(), userID, uuid.New())
	assert.ErrorIs(t, err, database.ErrResourceNotFound)
}
