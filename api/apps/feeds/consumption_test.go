package feeds_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"tools.xdoubleu.com/internal/consumption"
)

func TestListConsumption(t *testing.T) {
	ctx := context.Background()
	const uid = "feeds-consumption-test-user"
	_, err := testDB.Exec(ctx, "DELETE FROM feeds.feeds WHERE user_id = $1", uid)
	require.NoError(t, err)

	var feedID string
	require.NoError(t, testDB.QueryRow(ctx, `
		INSERT INTO feeds.feeds (user_id, url, title, source_type)
		VALUES ($1, 'https://consumption.example/feed', 'Consumption Blog', 'rss')
		RETURNING id`, uid).Scan(&feedID))

	read := time.Date(2026, 5, 2, 10, 0, 0, 0, time.UTC)
	published := time.Date(2026, 5, 1, 10, 0, 0, 0, time.UTC)
	_, err = testDB.Exec(ctx, `
		INSERT INTO feeds.items (feed_id, guid, title, published_at, read_at,
		    dismissed, bookmarked, read_progress_pct, categories, filtered_at)
		VALUES
		    ($1, 'bm', 'Bookmarked', $2, NULL, false, true, 0, '{Go}', NULL),
		    ($1, 'full', 'Read in full', $2, $3, false, false, 90, '{}', NULL),
		    ($1, 'skim', 'Skimmed', $2, $3, false, false, 10, '{}', NULL),
		    ($1, 'dis', 'Dismissed', $2, NULL, true, false, 0, '{}', NULL),
		    ($1, 'unread', 'Unread', $2, NULL, false, false, 0, '{}', NULL),
		    ($1, 'filt', 'Filtered', $2, NULL, true, false, 0, '{}', now())`,
		feedID, published, read)
	require.NoError(t, err)

	events, err := testApp.ListConsumption(ctx, uid)
	require.NoError(t, err)

	got := map[string]consumption.Event{}
	for _, e := range events {
		got[e.Title] = e
	}
	require.Len(t, got, 4)
	assert.Equal(t, consumption.StrongPositive, got["Bookmarked"].Signal)
	assert.Equal(t, []string{"Go"}, got["Bookmarked"].Tags)
	assert.True(t, got["Bookmarked"].At.IsZero())
	assert.Equal(t, "Consumption Blog", got["Bookmarked"].Creator)
	assert.Equal(t, consumption.DomainFeeds, got["Bookmarked"].Domain)
	assert.Equal(t, consumption.Positive, got["Read in full"].Signal)
	assert.True(t, read.Equal(got["Read in full"].At))
	assert.Equal(t, consumption.Neutral, got["Skimmed"].Signal)
	assert.Equal(t, consumption.Negative, got["Dismissed"].Signal)
	assert.Equal(t, "dismissed", got["Dismissed"].Reason)
	assert.True(t, got["Dismissed"].At.IsZero())
}

func TestListConsumption_Error(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := testApp.ListConsumption(ctx, "consumption-error-user")
	assert.Error(t, err)
}
