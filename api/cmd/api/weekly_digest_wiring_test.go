package main

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"tools.xdoubleu.com/apps/feeds"
)

// findFeedsApp locates the concrete *feeds.Feeds among testApp.apps, the
// same instance NewApplication wired into feedsHealthAdapter.
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

// TestFeedsHealthAdapterListUnhealthy exercises the same adapter wired into
// app.weeklyDigestJob by newWeeklyDigestJob, proving it maps a real
// feeds.UnhealthyFeed into jobs.UnhealthyFeed correctly end to end (issue
// #1014) — the job itself won't run within a test's lifetime since its
// RunEvery is 7 days.
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
