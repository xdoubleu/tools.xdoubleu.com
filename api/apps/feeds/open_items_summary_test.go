package feeds_test

import (
	"context"
	"testing"

	"connectrpc.com/connect"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	feedsv1 "tools.xdoubleu.com/gen/feeds/v1"
)

func TestListOpenItemsReturnsUnreadCountsPerFeed(t *testing.T) {
	base := uniqueBlogBase()
	feedURL := base + "/rss.xml"
	mockWebFetch.SetBody(feedURL, "application/rss+xml", []byte(rssXML(
		"Open Items Blog",
		rssItem{"Post 1", base + "/o1", "o1", itemContent},
		rssItem{"Post 2", base + "/o2", "o2", itemContent},
	)))

	client := newFeedsClient(t)
	created, err := client.CreateFeed(
		context.Background(),
		connect.NewRequest(&feedsv1.CreateFeedRequest{Url: feedURL}),
	)
	require.NoError(t, err)
	waitForFeedImport(t, client, created.Msg.Feed.Id)

	open, err := testApp.ListOpenItems(context.Background())
	require.NoError(t, err)

	var found bool
	for _, feed := range open {
		if feed.URL == feedURL {
			found = true
			assert.Equal(t, "Open Items Blog", feed.Title)
			assert.Equal(t, 2, feed.Count)
		}
	}
	assert.True(t, found, "expected %s to be reported with open items", feedURL)
}

func TestListOpenItemsOmitsFeedOnceEverythingIsRead(t *testing.T) {
	base := uniqueBlogBase()
	feedURL := base + "/rss.xml"
	mockWebFetch.SetBody(feedURL, "application/rss+xml", []byte(rssXML(
		"All Read Blog",
		rssItem{"Post", base + "/r1", "r1", itemContent},
	)))

	client := newFeedsClient(t)
	created, err := client.CreateFeed(
		context.Background(),
		connect.NewRequest(&feedsv1.CreateFeedRequest{Url: feedURL}),
	)
	require.NoError(t, err)
	waitForFeedImport(t, client, created.Msg.Feed.Id)

	items, err := client.ListFeedItems(
		context.Background(),
		connect.NewRequest(&feedsv1.ListFeedItemsRequest{FeedId: &created.Msg.Feed.Id}),
	)
	require.NoError(t, err)
	require.Len(t, items.Msg.Items, 1)

	read := true
	_, err = client.UpdateItem(
		context.Background(),
		connect.NewRequest(&feedsv1.UpdateItemRequest{
			ItemId: items.Msg.Items[0].Id,
			Read:   &read,
		}),
	)
	require.NoError(t, err)

	open, err := testApp.ListOpenItems(context.Background())
	require.NoError(t, err)

	for _, feed := range open {
		assert.NotEqual(t, feedURL, feed.URL)
	}
}

// TestListOpenItemsHandlesFeedWithNoURL covers an email-relay feed
// (feeds.feeds.url is nullable — email feeds have none) with an unread
// item: CountUnreadByFeed must not error scanning a NULL url column.
func TestListOpenItemsHandlesFeedWithNoURL(t *testing.T) {
	ctx := context.Background()
	var feedID uuid.UUID
	err := testDB.QueryRow(ctx, `
		INSERT INTO feeds.feeds (id, user_id, url, title, source_type)
		VALUES (gen_random_uuid(), $1, NULL, 'Email Feed No URL', 'email')
		RETURNING id
	`, userID).Scan(&feedID)
	require.NoError(t, err)

	_, err = testDB.Exec(ctx, `
		INSERT INTO feeds.items (feed_id, guid, title, source_url, published_at)
		VALUES ($1, 'no-url-item', 'Email Item', '', now())
	`, feedID)
	require.NoError(t, err)

	open, err := testApp.ListOpenItems(ctx)
	require.NoError(t, err)

	var found bool
	for _, feed := range open {
		if feed.Title == "Email Feed No URL" {
			found = true
			assert.Empty(t, feed.URL)
			assert.Equal(t, 1, feed.Count)
		}
	}
	assert.True(t, found, "expected the email feed to be reported with open items")
}
