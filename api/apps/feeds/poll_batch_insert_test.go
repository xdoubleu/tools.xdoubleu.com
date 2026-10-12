package feeds_test

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strconv"
	"strings"
	"testing"
	"time"

	"connectrpc.com/connect"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	feedsv1 "tools.xdoubleu.com/gen/feeds/v1"
)

// A poll with more new items than the per-poll cap stores the capped items
// and marks the overflow seen, all through batched inserts.
func TestPollBatchesCapAndSeenMarkers(t *testing.T) {
	client := newFeedsClient(t)
	base := uniqueBlogBase()
	feedURL := base + "/feed.xml"

	items := make([]rssItem, 0, 25)
	for i := range 25 {
		items = append(items, rssItem{
			fmt.Sprintf("Post %d", i), fmt.Sprintf("%s/p%d", base, i),
			fmt.Sprintf("batch-guid-%d", i), itemContent,
		})
	}
	mockWebFetch.SetBody(
		feedURL, "application/rss+xml", []byte(rssXML("Batch Blog", items...)),
	)

	created, err := client.CreateFeed(
		context.Background(),
		connect.NewRequest(&feedsv1.CreateFeedRequest{Url: feedURL}),
	)
	require.NoError(t, err)
	feedID := created.Msg.Feed.Id
	waitForSeenRows(t, feedID, 25)

	var skipped int
	require.NoError(t, testDB.QueryRow(context.Background(), `
		SELECT count(*) FROM feeds.items
		WHERE feed_id = $1 AND ingest_error = 'skipped: over per-poll cap'`,
		feedID).Scan(&skipped))
	assert.Equal(t, 5, skipped)
}

// A row the batch cannot store must not block the rest of the poll.
func TestPollBatchFailureFallsBackToPerRow(t *testing.T) {
	client := newFeedsClient(t)
	base := uniqueBlogBase()
	feedURL := base + "/feed.xml"

	// Incompressible and over the btree index row limit, so storing it fails.
	var huge strings.Builder
	for i := range 100 {
		sum := sha256.Sum256([]byte(strconv.Itoa(i)))
		huge.WriteString(hex.EncodeToString(sum[:]))
	}
	hugeGUID := huge.String()
	mockWebFetch.SetBody(feedURL, "application/rss+xml", []byte(rssXML(
		"Fallback Blog",
		rssItem{"Bad", base + "/bad", hugeGUID, itemContent},
		rssItem{"Good", base + "/good", "fallback-good", itemContent},
	)))

	created, err := client.CreateFeed(
		context.Background(),
		connect.NewRequest(&feedsv1.CreateFeedRequest{Url: feedURL}),
	)
	require.NoError(t, err)
	feedID := created.Msg.Feed.Id
	waitForSeenRows(t, feedID, 1)

	var n int
	require.NoError(t, testDB.QueryRow(context.Background(),
		"SELECT count(*) FROM feeds.items WHERE feed_id = $1 AND guid = 'fallback-good'",
		feedID).Scan(&n))
	assert.Equal(t, 1, n)
}

// waitForSeenRows waits until the background import stored at least want rows.
func waitForSeenRows(t *testing.T, feedID string, want int) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if countSeenRows(t, feedID) >= want {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatalf("feed %s never stored %d rows", feedID, want)
}
