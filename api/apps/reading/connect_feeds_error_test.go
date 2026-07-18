package reading_test

import (
	"context"
	"errors"
	"testing"

	"connectrpc.com/connect"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	readingv1 "tools.xdoubleu.com/gen/reading/v1"
)

// --- connect_feeds.go error mapping ---

func TestUpdateFeed_InvalidID(t *testing.T) {
	client := newBooksTestClient(t)
	_, err := client.UpdateFeed(
		context.Background(),
		feedReq(t, &readingv1.UpdateFeedRequest{
			FeedId: "not-a-uuid", Title: "x", KoboSync: false,
		}),
	)
	require.Error(t, err)
	assert.Equal(t, connect.CodeInvalidArgument, connect.CodeOf(err))
}

func TestDeleteFeed_InvalidID(t *testing.T) {
	client := newBooksTestClient(t)
	_, err := client.DeleteFeed(
		context.Background(),
		feedReq(t, &readingv1.DeleteFeedRequest{FeedId: "not-a-uuid"}),
	)
	require.Error(t, err)
	assert.Equal(t, connect.CodeInvalidArgument, connect.CodeOf(err))
}

func TestUpdateFeed_UnknownFeed_NotFound(t *testing.T) {
	client := newBooksTestClient(t)
	_, err := client.UpdateFeed(
		context.Background(),
		feedReq(t, &readingv1.UpdateFeedRequest{
			FeedId: uuid.NewString(), Title: "x", KoboSync: true,
		}),
	)
	require.Error(t, err)
	assert.Equal(t, connect.CodeNotFound, connect.CodeOf(err))
}

func TestDeleteFeed_UnknownFeed_NotFound(t *testing.T) {
	client := newBooksTestClient(t)
	_, err := client.DeleteFeed(
		context.Background(),
		feedReq(t, &readingv1.DeleteFeedRequest{FeedId: uuid.NewString()}),
	)
	require.Error(t, err)
	assert.Equal(t, connect.CodeNotFound, connect.CodeOf(err))
}

func TestCreateFeed_FetchError_InvalidArgument(t *testing.T) {
	// The feed URL itself cannot be fetched — Create wraps it as ErrInvalidFeed.
	feedURL := uniqueBlogBase() + "/unreachable-feed.xml"
	mockWebFetch.Errs[feedURL] = errors.New("boom")

	client := newBooksTestClient(t)
	_, err := client.CreateFeed(
		context.Background(),
		feedReq(t, &readingv1.CreateFeedRequest{Url: feedURL, KoboSync: false}),
	)
	require.Error(t, err)
	assert.Equal(t, connect.CodeInvalidArgument, connect.CodeOf(err))
}

// --- feeds.go poll error + last_error path ---

func TestRefreshFeed_FetchError_RecordsLastError(t *testing.T) {
	base := uniqueBlogBase()
	feedURL := base + "/feed-fail.xml"
	mockWebFetch.SetBody(feedURL, "application/rss+xml", []byte(rssXML(
		"Fail Blog",
		rssItem{"Post", base + "/fp1", "fp1", itemContent},
	)))

	client := newBooksTestClient(t)
	created, err := client.CreateFeed(
		context.Background(),
		feedReq(t, &readingv1.CreateFeedRequest{Url: feedURL, KoboSync: false}),
	)
	require.NoError(t, err)
	feedID := created.Msg.Feed.Id

	// The next poll fails to fetch the feed body.
	mockWebFetch.Errs[feedURL] = errors.New("network down")
	_, err = client.RefreshFeed(
		context.Background(),
		feedReq(t, &readingv1.RefreshFeedRequest{FeedId: feedID}),
	)
	require.Error(t, err)

	// The failure is recorded on the feed for the UI to surface.
	list, err := client.ListFeeds(
		context.Background(), feedReq(t, &readingv1.ListFeedsRequest{}),
	)
	require.NoError(t, err)
	var found *readingv1.Feed
	for _, f := range list.Msg.Feeds {
		if f.Id == feedID {
			found = f
		}
	}
	require.NotNil(t, found)
	assert.NotEmpty(t, found.LastError)

	// Recover for any later reuse of this URL.
	delete(mockWebFetch.Errs, feedURL)
}

// TestCreateFeed_ItemWithoutLink covers the per-item error path: an item with
// no link cannot be ingested, is marked seen with an error, and does not count.
func TestCreateFeed_ItemWithoutLink(t *testing.T) {
	base := uniqueBlogBase()
	feedURL := base + "/feed-nolink.xml"
	rss := `<?xml version="1.0"?><rss version="2.0">` +
		`<channel><title>No Link Blog</title>` +
		`<item><title>No Link Post</title><guid>nl1</guid>` +
		`<description>body</description></item>` +
		`</channel></rss>`
	mockWebFetch.SetBody(feedURL, "application/rss+xml", []byte(rss))

	client := newBooksTestClient(t)
	created, err := client.CreateFeed(
		context.Background(),
		feedReq(t, &readingv1.CreateFeedRequest{Url: feedURL, KoboSync: false}),
	)
	require.NoError(t, err)
	// The item has no link (gofeed leaves it empty), so it is not ingested.
	assert.Equal(t, int32(0), created.Msg.Ingested)
}
