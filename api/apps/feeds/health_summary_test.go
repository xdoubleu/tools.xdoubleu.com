package feeds_test

import (
	"context"
	"testing"

	"connectrpc.com/connect"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	feedsv1 "tools.xdoubleu.com/gen/feeds/v1"
)

func TestListUnhealthyReturnsFeedsFailingToPoll(t *testing.T) {
	base := uniqueBlogBase()
	feedURL := base + "/rss.xml"
	mockWebFetch.SetBody(feedURL, "application/rss+xml", []byte(rssXML(
		"Health Blog", rssItem{"Post", base + "/h1", "h1", itemContent},
	)))

	client := newFeedsClient(t)
	created, err := client.CreateFeed(
		context.Background(),
		connect.NewRequest(&feedsv1.CreateFeedRequest{Url: feedURL}),
	)
	require.NoError(t, err)
	waitForFeedImport(t, client, created.Msg.Feed.Id)

	mockWebFetch.Errs[feedURL] = assert.AnError
	_, err = client.RefreshFeed(
		context.Background(),
		connect.NewRequest(&feedsv1.RefreshFeedRequest{FeedId: created.Msg.Feed.Id}),
	)
	require.Error(t, err)

	unhealthy, err := testApp.ListUnhealthy(context.Background())
	require.NoError(t, err)

	var found bool
	for _, feed := range unhealthy {
		if feed.URL == feedURL {
			found = true
			assert.Equal(t, 1, feed.ConsecutiveFailures)
			assert.Contains(t, feed.LastError, assert.AnError.Error())
		}
	}
	assert.True(t, found, "expected %s to be reported unhealthy", feedURL)
}
