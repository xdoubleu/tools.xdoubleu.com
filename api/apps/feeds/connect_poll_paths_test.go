package feeds_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"connectrpc.com/connect"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"tools.xdoubleu.com/apps/feeds/internal/models"
	feedsv1 "tools.xdoubleu.com/gen/feeds/v1"
)

// pollScrapeFeed's error paths can't be reached through the happy-path
// create/import tests; drive them via the poll job (RunPollNow) against
// deliberately broken mock responses.
func TestPollScrapeFeed_PollPaths(t *testing.T) {
	base := uniqueBlogBase()
	indexURL := base + "/blog-poll-paths"
	postURL := base + "/posts/poll-paths-post"
	mockWebFetch.SetHTML(
		indexURL,
		blogIndexHTML(postURL, "A poll paths post with a long title"),
	)
	mockWebFetch.SetHTML(postURL, articlePageHTML("Poll Paths Post"))

	client := newFeedsClient(t)
	created, err := client.CreateFeed(
		context.Background(),
		connect.NewRequest(&feedsv1.CreateFeedRequest{
			Url:  indexURL,
			Kind: feedsv1.FeedKind_FEED_KIND_SCRAPE,
		}),
	)
	require.NoError(t, err)
	feedID := created.Msg.Feed.Id
	waitForFeedImport(t, client, feedID)

	// Fetch error: the poll records the failure on the feed and returns.
	mockWebFetch.Errs[indexURL] = errors.New("boom")
	require.NoError(t, testApp.RunPollNow(context.Background()))
	feed := waitForFeedLastError(t, feedID)
	assert.Contains(t, *feed.LastError, "boom")

	// Walk error: the index now serves no plausible post links, so
	// discovery fails and the poll records that failure too.
	delete(mockWebFetch.Errs, indexURL)
	mockWebFetch.SetHTML(indexURL, `<!DOCTYPE html><html><body>
		<nav><a href="/">Home</a></nav>
	</body></html>`)
	require.NoError(t, testApp.RunPollNow(context.Background()))
	feed = waitForFeedLastError(t, feedID)
	assert.Contains(t, *feed.LastError, "no post links found on page")

	// NotModified: a 304 on the conditional GET short-circuits the poll.
	mockWebFetch.SetNotModified(indexURL)
	require.NoError(t, testApp.RunPollNow(context.Background()))
	assertFeedLastErrorCleared(t, feedID)
}

// waitForFeedLastError polls the feed list until feedID reports a non-empty
// LastError.
func waitForFeedLastError(t *testing.T, feedID string) *models.Feed {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		feeds, err := testApp.Services.Feeds.List(context.Background(), userID)
		require.NoError(t, err)
		for i := range feeds {
			if feeds[i].ID.String() == feedID &&
				feeds[i].LastError != nil && *feeds[i].LastError != "" {
				return &feeds[i]
			}
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatalf("feed %s never recorded a poll error", feedID)
	return nil
}

func assertFeedLastErrorCleared(t *testing.T, feedID string) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		feeds, err := testApp.Services.Feeds.List(context.Background(), userID)
		require.NoError(t, err)
		for i := range feeds {
			if feeds[i].ID.String() == feedID &&
				(feeds[i].LastError == nil || *feeds[i].LastError == "") {
				return
			}
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatalf("feed %s's last error never cleared", feedID)
}
