package main

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"tools.xdoubleu.com/apps/feeds"
)

// findFeedsApp returns the *feeds.Feeds in testApp.apps.
func findFeedsApp(t *testing.T) *feeds.Feeds {
	t.Helper()
	for _, a := range *testApp.apps {
		if fa, ok := a.(*feeds.Feeds); ok {
			return fa
		}
	}
	t.Fatal("feeds app not found in testApp.apps")
	return nil
}

// TestFeedsHealthAdapterListUnhealthy checks the adapter maps
// feeds.UnhealthyFeed to jobs.UnhealthyFeed.
func TestFeedsHealthAdapterListUnhealthy(t *testing.T) {
	ctx := context.Background()
	feedsApp := findFeedsApp(t)
	feedURL := "https://wiring-test-" + uuid.NewString() + ".example.com/feed.xml"

	_, err := testApp.db.Exec(ctx, `
		INSERT INTO feeds.feeds
			(id, user_id, url, title, source_type, consecutive_failures, last_error)
		VALUES
			(gen_random_uuid(), $1, $2, 'Wiring Test Feed', 'rss', 5, 'boom')
	`, testUserID, feedURL)
	require.NoError(t, err)

	adapter := feedsHealthAdapter{feeds: feedsApp}
	unhealthy, err := adapter.ListUnhealthy(ctx)
	require.NoError(t, err)

	var found bool
	for _, feed := range unhealthy {
		if feed.URL == feedURL {
			found = true
			assert.Equal(t, "Wiring Test Feed", feed.Title)
			assert.Equal(t, 5, feed.ConsecutiveFailures)
			assert.Equal(t, "boom", feed.LastError)
		}
	}
	assert.True(t, found)
}

// TestFeedsOpenItemsAdapterListOpenItems checks the adapter maps unread item
// counts to jobs.OpenFeedItem.
func TestFeedsOpenItemsAdapterListOpenItems(t *testing.T) {
	ctx := context.Background()
	feedsApp := findFeedsApp(t)
	feedURL := "https://wiring-test-open-" + uuid.NewString() + ".example.com/feed.xml"

	var feedID uuid.UUID
	err := testApp.db.QueryRow(ctx, `
		INSERT INTO feeds.feeds (id, user_id, url, title, source_type)
		VALUES (gen_random_uuid(), $1, $2, 'Wiring Test Open Items Feed', 'rss')
		RETURNING id
	`, testUserID, feedURL).Scan(&feedID)
	require.NoError(t, err)

	_, err = testApp.db.Exec(ctx, `
		INSERT INTO feeds.items (feed_id, guid, title, source_url, published_at)
		VALUES
			($1, 'wiring-open-1', 'Item 1', 'https://example.com/1', now()),
			($1, 'wiring-open-2', 'Item 2', 'https://example.com/2', now())
	`, feedID)
	require.NoError(t, err)

	adapter := feedsOpenItemsAdapter{feeds: feedsApp}
	open, err := adapter.ListOpenItems(ctx)
	require.NoError(t, err)

	var found bool
	for _, feed := range open {
		if feed.URL == feedURL {
			found = true
			assert.Equal(t, "Wiring Test Open Items Feed", feed.Title)
			assert.Equal(t, 2, feed.Count)
		}
	}
	assert.True(t, found)
}
