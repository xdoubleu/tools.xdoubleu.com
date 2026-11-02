package main

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"tools.xdoubleu.com/apps/feeds"
	"tools.xdoubleu.com/internal/observability/jobs"
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

// TestFeedsDigestAdapterListDigestItems checks the adapter maps the newest and
// last-chance items to jobs.FeedDigestItem.
func TestFeedsDigestAdapterListDigestItems(t *testing.T) {
	ctx := context.Background()
	feedsApp := findFeedsApp(t)
	feedURL := "https://wiring-test-open-" + uuid.NewString() + ".example.com/feed.xml"

	var feedID uuid.UUID
	err := testApp.db.QueryRow(ctx, `
		INSERT INTO feeds.feeds (id, user_id, url, title, source_type)
		VALUES (gen_random_uuid(), $1, $2, 'Wiring Test Digest Feed', 'rss')
		RETURNING id
	`, testUserID, feedURL).Scan(&feedID)
	require.NoError(t, err)

	// The second item arrived 6.5 days ago: inside the 48h last-chance window
	// of a 7-day lifetime.
	_, err = testApp.db.Exec(ctx, `
		INSERT INTO feeds.items
			(feed_id, guid, title, source_url, published_at, created_at)
		VALUES
			($1, 'wiring-open-1', 'Item 1', 'https://example.com/1', now(), now()),
			($1, 'wiring-open-2', 'Item 2', 'https://example.com/2',
			 now() - interval '6 days 12 hours', now() - interval '6 days 12 hours')
	`, feedID)
	require.NoError(t, err)

	adapter := feedsDigestAdapter{feeds: feedsApp}
	digest, err := adapter.ListDigestItems(ctx)
	require.NoError(t, err)

	// The shared table may hold newer items from other tests, so only the
	// soonest-expiring item is asserted.
	var lastChance *jobs.FeedDigestItem
	for i, item := range digest.LastChance {
		if item.URL == "https://example.com/2" {
			lastChance = &digest.LastChance[i]
		}
	}
	require.NotNil(t, lastChance)
	assert.Equal(t, "Wiring Test Digest Feed", lastChance.FeedTitle)
	assert.NotNil(t, lastChance.ExpiresAt)
}
