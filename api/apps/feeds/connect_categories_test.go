package feeds_test

import (
	"context"
	"slices"
	"testing"
	"time"

	"connectrpc.com/connect"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	feedsv1 "tools.xdoubleu.com/gen/feeds/v1"
	"tools.xdoubleu.com/gen/feeds/v1/feedsv1connect"
)

// categorizedRSSItem is an RSS <item> with raw <category> values; no
// embedded content, so ingest fetches the linked page.
type categorizedRSSItem struct {
	link       string
	categories []string
}

func categorizedRSSXML(items ...categorizedRSSItem) string {
	body := `<?xml version="1.0"?><rss version="2.0"><channel><title>Cat Blog</title>`
	for _, it := range items {
		body += `<item><title>Post ` + it.link + `</title><link>` + it.link +
			`</link><guid>` + it.link + `</guid>`
		for _, c := range it.categories {
			body += `<category>` + c + `</category>`
		}
		body += `</item>`
	}
	return body + `</channel></rss>`
}

// feedItemCategories maps source URL to categories for feedID's items.
func feedItemCategories(
	t *testing.T,
	client feedsv1connect.FeedServiceClient,
	feedID string,
) map[string][]string {
	t.Helper()
	resp, err := client.ListFeedItems(
		context.Background(),
		connect.NewRequest(&feedsv1.ListFeedItemsRequest{FeedId: &feedID}),
	)
	require.NoError(t, err)
	out := map[string][]string{}
	for _, item := range resp.Msg.Items {
		out[item.SourceUrl] = item.Categories
	}
	return out
}

// waitForItemCount waits until the background import stored n items.
func waitForItemCount(
	t *testing.T,
	client feedsv1connect.FeedServiceClient,
	feedID string,
	n int,
) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if len(feedItemCategories(t, client, feedID)) == n {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatalf("feed %s never imported %d items", feedID, n)
}

func countFetches(url string) int {
	return len(slices.DeleteFunc(slices.Clone(mockWebFetch.Calls), func(c string) bool {
		return c != url
	}))
}

func TestCreateFeed_RSS_StoresCategories(t *testing.T) {
	base := uniqueBlogBase()
	feedURL := base + "/feed-cats.xml"
	postURL := base + "/posts/categorized"
	mockWebFetch.SetHTML(postURL, articlePageHTML("Categorized"))
	mockWebFetch.SetBody(feedURL, "application/rss+xml", []byte(categorizedRSSXML(
		categorizedRSSItem{postURL, []string{" Go ", "", "Databases", "go"}},
	)))

	client := newFeedsClient(t)
	created, err := client.CreateFeed(
		context.Background(),
		connect.NewRequest(&feedsv1.CreateFeedRequest{Url: feedURL}),
	)
	require.NoError(t, err)
	waitForFeedImport(t, client, created.Msg.Feed.Id)

	assert.Equal(t, []string{"Go", "Databases"},
		feedItemCategories(t, client, created.Msg.Feed.Id)[postURL])
}

func TestRefreshFeed_RSS_BackfillsMissingCategories(t *testing.T) {
	base := uniqueBlogBase()
	feedURL := base + "/feed-backfill.xml"
	bareURL := base + "/posts/bare"
	taggedURL := base + "/posts/tagged"
	mockWebFetch.SetHTML(bareURL, articlePageHTML("Bare"))
	mockWebFetch.SetHTML(taggedURL, articlePageHTML("Tagged"))
	mockWebFetch.SetBody(feedURL, "application/rss+xml", []byte(categorizedRSSXML(
		categorizedRSSItem{bareURL, nil},
		categorizedRSSItem{taggedURL, []string{"Keep"}},
	)))

	client := newFeedsClient(t)
	created, err := client.CreateFeed(
		context.Background(),
		connect.NewRequest(&feedsv1.CreateFeedRequest{Url: feedURL}),
	)
	require.NoError(t, err)
	feedID := created.Msg.Feed.Id
	waitForItemCount(t, client, feedID, 2)
	require.Empty(t, feedItemCategories(t, client, feedID)[bareURL])

	mockWebFetch.SetBody(feedURL, "application/rss+xml", []byte(categorizedRSSXML(
		categorizedRSSItem{bareURL, []string{"News"}},
		categorizedRSSItem{taggedURL, []string{"Other"}},
	)))
	fetchesBefore := countFetches(bareURL) + countFetches(taggedURL)

	refreshed, err := client.RefreshFeed(
		context.Background(),
		connect.NewRequest(&feedsv1.RefreshFeedRequest{FeedId: feedID}),
	)
	require.NoError(t, err)
	assert.Equal(t, int32(0), refreshed.Msg.Ingested)

	categories := feedItemCategories(t, client, feedID)
	assert.Equal(t, []string{"News"}, categories[bareURL])
	assert.Equal(t, []string{"Keep"}, categories[taggedURL])
	assert.Equal(t, fetchesBefore, countFetches(bareURL)+countFetches(taggedURL),
		"backfill must not refetch content")
}

func TestRefreshFeed_Scrape_StoresAndBackfillsCategories(t *testing.T) {
	base := uniqueBlogBase()
	indexURL := base + "/blog-cats"
	oldURL := base + "/posts/older-post-with-a-long-title"
	newURL := base + "/posts/newer-post-with-a-long-title"
	mockWebFetch.SetHTML(
		indexURL,
		blogIndexHTML(oldURL, "Older post with a long title"),
	)
	mockWebFetch.SetHTML(oldURL, articlePageHTML("Older"))
	mockWebFetch.SetHTML(newURL, articlePageHTML("Newer"))

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
	waitForItemCount(t, client, feedID, 1)
	require.Empty(t, feedItemCategories(t, client, feedID)[oldURL])

	mockWebFetch.SetHTML(indexURL, `<!DOCTYPE html><html><body><ul>`+
		`<li><article><h2><a href="`+newURL+`">Newer post with a long title</a></h2>`+
		`<div fs-list-field="category">Enterprise AI</div></article></li>`+
		`<li><article><h2><a href="`+oldURL+`">Older post with a long title</a></h2>`+
		`<div fs-list-field="category">Product announcements</div></article></li>`+
		`</ul></body></html>`)
	oldFetchesBefore := countFetches(oldURL)

	refreshed, err := client.RefreshFeed(
		context.Background(),
		connect.NewRequest(&feedsv1.RefreshFeedRequest{FeedId: feedID}),
	)
	require.NoError(t, err)
	assert.Equal(t, int32(1), refreshed.Msg.Ingested)

	categories := feedItemCategories(t, client, feedID)
	assert.Equal(t, []string{"Enterprise AI"}, categories[newURL])
	assert.Equal(t, []string{"Product announcements"}, categories[oldURL])
	assert.Equal(t, oldFetchesBefore, countFetches(oldURL),
		"backfill must not refetch content")
}
