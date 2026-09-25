package feeds_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"connectrpc.com/connect"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"

	"tools.xdoubleu.com/apps/feeds/pkg/webfetch"
	feedsv1 "tools.xdoubleu.com/gen/feeds/v1"
	"tools.xdoubleu.com/gen/feeds/v1/feedsv1connect"
	"tools.xdoubleu.com/internal/pagination"
)

func newFeedsClient(t *testing.T) feedsv1connect.FeedServiceClient {
	t.Helper()
	ts := httptest.NewServer(getRoutes())
	t.Cleanup(ts.Close)
	return feedsv1connect.NewFeedServiceClient(http.DefaultClient, ts.URL)
}

func uniqueBlogBase() string {
	return "https://blog-" + uuid.NewString() + ".example.com"
}

const itemContent = "<p>Lorem ipsum article body.</p>"

// articlePageHTML builds a minimal readability-extractable HTML page.
func articlePageHTML(title string) string {
	return `<!DOCTYPE html><html><head><title>` + title + `</title></head><body>` +
		`<article><h1>` + title + `</h1><p>Lorem ipsum dolor sit amet, ` +
		`consectetur adipiscing elit. Sed do eiusmod tempor incididunt ut ` +
		`labore et dolore magna aliqua. Ut enim ad minim veniam, quis ` +
		`nostrud exercitation ullamco laboris nisi ut aliquip ex ea commodo ` +
		`consequat.</p></article></body></html>`
}

// rssItem is one <item> in a hand-built RSS feed.
type rssItem struct {
	title, link, guid, content string
}

func rssXML(feedTitle string, items ...rssItem) string {
	body := `<?xml version="1.0"?><rss version="2.0" ` +
		`xmlns:content="http://purl.org/rss/1.0/modules/content/">` +
		`<channel><title>` + feedTitle + `</title>`
	for _, it := range items {
		body += `<item><title>` + it.title + `</title><link>` + it.link +
			`</link><guid>` + it.guid + `</guid>`
		if it.content != "" {
			body += `<content:encoded><![CDATA[` + it.content + `]]></content:encoded>`
		}
		body += `</item>`
	}
	body += `</channel></rss>`
	return body
}

// countSeenRows counts feeds.items rows for feedID, including dedup markers.
func countSeenRows(t *testing.T, feedID string) int {
	t.Helper()
	var count int
	err := testDB.QueryRow(
		context.Background(),
		"SELECT count(*) FROM feeds.items WHERE feed_id = $1",
		feedID,
	).Scan(&count)
	require.NoError(t, err)
	return count
}

// fetchItemContent fetches an item's body via GetFeedItem (lists omit it).
func fetchItemContent(
	t *testing.T,
	client feedsv1connect.FeedServiceClient,
	itemID string,
) string {
	t.Helper()
	resp, err := client.GetFeedItem(
		context.Background(),
		connect.NewRequest(&feedsv1.GetFeedItemRequest{ItemId: itemID}),
	)
	require.NoError(t, err)
	return resp.Msg.Item.ContentHtml
}

// waitForFeedImport waits until the feed's background import lands an item.
func waitForFeedImport(
	t *testing.T,
	client feedsv1connect.FeedServiceClient,
	feedID string,
) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		resp, err := client.ListFeedItems(
			context.Background(), connect.NewRequest(&feedsv1.ListFeedItemsRequest{}),
		)
		require.NoError(t, err)
		for _, item := range resp.Msg.Items {
			if item.FeedId == feedID {
				return
			}
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatalf("feed %s never imported any items", feedID)
}

// waitForFeedPollHealth waits until feedID's Etag is set. The import writes
// items before the fetch result, so waitForFeedImport alone is too early for
// poll-health assertions.
func waitForFeedPollHealth(
	t *testing.T,
	client feedsv1connect.FeedServiceClient,
	feedID string,
) *feedsv1.Feed {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		resp, err := client.ListFeeds(
			context.Background(), connect.NewRequest(&feedsv1.ListFeedsRequest{}),
		)
		require.NoError(t, err)
		if found := findFeed(resp.Msg.Feeds, feedID); found != nil && found.Etag != "" {
			return found
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatalf("feed %s never recorded its initial fetch result", feedID)
	return nil
}

func TestListFeeds_Empty(t *testing.T) {
	client := newFeedsClient(t)
	resp, err := client.ListFeeds(
		context.Background(), connect.NewRequest(&feedsv1.ListFeedsRequest{}),
	)
	require.NoError(t, err)
	assert.NotNil(t, resp.Msg)
}

func TestCreateFeed_RSS_Success(t *testing.T) {
	base := uniqueBlogBase()
	feedURL := base + "/feed.xml"
	mockWebFetch.SetBody(feedURL, "application/rss+xml", []byte(rssXML(
		"My Blog", rssItem{"Post One", base + "/one", "guid-1", itemContent},
	)))

	client := newFeedsClient(t)
	resp, err := client.CreateFeed(
		context.Background(),
		connect.NewRequest(&feedsv1.CreateFeedRequest{Url: feedURL}),
	)
	require.NoError(t, err)
	assert.NotEmpty(t, resp.Msg.Feed.Id)
	assert.Equal(t, "My Blog", resp.Msg.Feed.Title)
	assert.Equal(t, "rss", resp.Msg.Feed.SourceType)

	waitForFeedImport(t, client, resp.Msg.Feed.Id)

	items, err := client.ListFeedItems(
		context.Background(), connect.NewRequest(&feedsv1.ListFeedItemsRequest{}),
	)
	require.NoError(t, err)
	found := false
	for _, item := range items.Msg.Items {
		if item.SourceUrl == base+"/one" {
			found = true
			assert.Equal(t, "Post One", item.Title)
			// The list carries only the flag; the body comes from GetFeedItem.
			assert.True(t, item.HasContent)
			assert.Empty(t, item.ContentHtml)
			assert.Contains(t, fetchItemContent(t, client, item.Id), "Lorem ipsum")
		}
	}
	assert.True(t, found, "imported item should be listed")
}

func TestCreateFeed_InvalidURL(t *testing.T) {
	client := newFeedsClient(t)
	_, err := client.CreateFeed(
		context.Background(),
		connect.NewRequest(&feedsv1.CreateFeedRequest{Url: "not a url"}),
	)
	require.Error(t, err)
	assert.Equal(t, connect.CodeInvalidArgument, connect.CodeOf(err))
}

func TestCreateFeed_NotAFeed(t *testing.T) {
	url := uniqueBlogBase() + "/not-a-feed"
	mockWebFetch.SetBody(url, "text/html", []byte("<html>not xml</html>"))

	client := newFeedsClient(t)
	_, err := client.CreateFeed(
		context.Background(), connect.NewRequest(&feedsv1.CreateFeedRequest{Url: url}),
	)
	require.Error(t, err)
	assert.Equal(t, connect.CodeInvalidArgument, connect.CodeOf(err))
}

func TestCreateFeed_FetchesLinkedPageWhenNoEmbeddedContent(t *testing.T) {
	base := uniqueBlogBase()
	feedURL := base + "/feed-linked.xml"
	itemURL := base + "/linked-post"
	mockWebFetch.SetBody(feedURL, "application/rss+xml", []byte(rssXML(
		"Linked Blog", rssItem{"Linked Post", itemURL, "lp1", ""},
	)))
	mockWebFetch.SetHTML(itemURL, articlePageHTML("Linked Post Body"))

	client := newFeedsClient(t)
	created, err := client.CreateFeed(
		context.Background(),
		connect.NewRequest(&feedsv1.CreateFeedRequest{Url: feedURL}),
	)
	require.NoError(t, err)
	waitForFeedImport(t, client, created.Msg.Feed.Id)

	items, err := client.ListFeedItems(
		context.Background(), connect.NewRequest(&feedsv1.ListFeedItemsRequest{}),
	)
	require.NoError(t, err)
	var found *feedsv1.Item
	for _, item := range items.Msg.Items {
		if item.SourceUrl == itemURL {
			found = item
		}
	}
	require.NotNil(t, found)
	assert.True(t, found.HasContent)
	assert.Contains(t, fetchItemContent(t, client, found.Id), "Lorem ipsum")
}

func TestCreateFeed_CapsItemsPerPoll(t *testing.T) {
	base := uniqueBlogBase()
	feedURL := base + "/feed-cap.xml"

	items := make([]rssItem, 0, 25)
	for i := 0; i < 25; i++ {
		id := uuid.NewString()
		items = append(items, rssItem{"Post " + id, base + "/" + id, id, itemContent})
	}
	mockWebFetch.SetBody(
		feedURL,
		"application/rss+xml",
		[]byte(rssXML("Cap Blog", items...)),
	)

	client := newFeedsClient(t)
	created, err := client.CreateFeed(
		context.Background(),
		connect.NewRequest(&feedsv1.CreateFeedRequest{Url: feedURL}),
	)
	require.NoError(t, err)

	// Capped guids are still marked seen, so wait for all 25 rows.
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) && countSeenRows(t, created.Msg.Feed.Id) < len(items) {
		time.Sleep(20 * time.Millisecond)
	}
	require.Equal(
		t, len(items), countSeenRows(t, created.Msg.Feed.Id),
		"every guid should be marked seen",
	)

	resp, err := client.ListFeedItems(
		context.Background(), connect.NewRequest(&feedsv1.ListFeedItemsRequest{}),
	)
	require.NoError(t, err)
	withContent := 0
	for _, item := range resp.Msg.Items {
		if item.FeedId != created.Msg.Feed.Id {
			continue
		}
		withContent++
	}
	assert.Equal(
		t,
		20,
		withContent,
		"only the per-poll cap's worth should be listed",
	)
}

// blogIndexHTML builds an index page with one post-like link.
func blogIndexHTML(postURL, postTitle string) string {
	return `<!DOCTYPE html><html><head><title>Scraped Blog</title></head><body>` +
		`<nav><a href="/">Home</a></nav>` +
		`<article><a href="` + postURL + `">` + postTitle + `</a></article>` +
		`</body></html>`
}

func TestCreateFeed_Scrape_Success(t *testing.T) {
	base := uniqueBlogBase()
	indexURL := base + "/blog"
	postURL := base + "/posts/announcing-something-new"
	mockWebFetch.SetHTML(
		indexURL, blogIndexHTML(postURL, "Announcing something brand new today"),
	)
	mockWebFetch.SetHTML(postURL, articlePageHTML("Announcing Something New"))

	client := newFeedsClient(t)
	resp, err := client.CreateFeed(
		context.Background(),
		connect.NewRequest(&feedsv1.CreateFeedRequest{
			Url:  indexURL,
			Kind: feedsv1.FeedKind_FEED_KIND_SCRAPE,
		}),
	)
	require.NoError(t, err)
	assert.NotEmpty(t, resp.Msg.Feed.Id)
	assert.Equal(t, "Scraped Blog", resp.Msg.Feed.Title)
	assert.Equal(t, "scrape", resp.Msg.Feed.SourceType)

	waitForFeedImport(t, client, resp.Msg.Feed.Id)

	items, err := client.ListFeedItems(
		context.Background(), connect.NewRequest(&feedsv1.ListFeedItemsRequest{}),
	)
	require.NoError(t, err)
	found := false
	for _, item := range items.Msg.Items {
		if item.SourceUrl == postURL {
			found = true
			assert.True(t, item.HasContent)
			assert.Contains(t, fetchItemContent(t, client, item.Id), "Lorem ipsum")
		}
	}
	assert.True(t, found, "discovered post should be imported")
}

func TestCreateFeed_Scrape_NoPostsFound(t *testing.T) {
	indexURL := uniqueBlogBase() + "/blog"
	mockWebFetch.SetHTML(
		indexURL, `<html><body><a href="/about">About</a></body></html>`,
	)

	client := newFeedsClient(t)
	_, err := client.CreateFeed(
		context.Background(),
		connect.NewRequest(&feedsv1.CreateFeedRequest{
			Url:  indexURL,
			Kind: feedsv1.FeedKind_FEED_KIND_SCRAPE,
		}),
	)
	require.Error(t, err)
	assert.Equal(t, connect.CodeInvalidArgument, connect.CodeOf(err))
}

func TestCreateFeed_Scrape_InvalidURL(t *testing.T) {
	client := newFeedsClient(t)
	_, err := client.CreateFeed(
		context.Background(),
		connect.NewRequest(&feedsv1.CreateFeedRequest{
			Url:  "not a url",
			Kind: feedsv1.FeedKind_FEED_KIND_SCRAPE,
		}),
	)
	require.Error(t, err)
	assert.Equal(t, connect.CodeInvalidArgument, connect.CodeOf(err))
}

func TestCreateFeed_Scrape_IndexFetchFails(t *testing.T) {
	// Never registered, so the initial fetch 404s.
	indexURL := uniqueBlogBase() + "/blog-never-registered"

	client := newFeedsClient(t)
	_, err := client.CreateFeed(
		context.Background(),
		connect.NewRequest(&feedsv1.CreateFeedRequest{
			Url:  indexURL,
			Kind: feedsv1.FeedKind_FEED_KIND_SCRAPE,
		}),
	)
	require.Error(t, err)
	assert.Equal(t, connect.CodeInvalidArgument, connect.CodeOf(err))
}

func TestCreateFeed_Scrape_Duplicate(t *testing.T) {
	base := uniqueBlogBase()
	indexURL := base + "/blog-dup"
	postURL := base + "/posts/dup-post-with-a-long-title"
	mockWebFetch.SetHTML(indexURL, blogIndexHTML(postURL, "Dup post with a long title"))
	mockWebFetch.SetHTML(postURL, articlePageHTML("Dup Post"))

	client := newFeedsClient(t)
	_, err := client.CreateFeed(
		context.Background(),
		connect.NewRequest(&feedsv1.CreateFeedRequest{
			Url:  indexURL,
			Kind: feedsv1.FeedKind_FEED_KIND_SCRAPE,
		}),
	)
	require.NoError(t, err)

	_, err = client.CreateFeed(
		context.Background(),
		connect.NewRequest(&feedsv1.CreateFeedRequest{
			Url:  indexURL,
			Kind: feedsv1.FeedKind_FEED_KIND_SCRAPE,
		}),
	)
	require.Error(t, err)
	assert.Equal(t, connect.CodeAlreadyExists, connect.CodeOf(err))
}

func TestCreateFeed_Scrape_ContentFetchFails(t *testing.T) {
	base := uniqueBlogBase()
	indexURL := base + "/blog-unreachable"
	postURL := base + "/posts/unreachable-post-with-a-long-title"
	mockWebFetch.SetHTML(
		indexURL, blogIndexHTML(postURL, "Unreachable post with a long title"),
	)
	// postURL is never registered, so the item is dropped and marked seen.

	client := newFeedsClient(t)
	created, err := client.CreateFeed(
		context.Background(),
		connect.NewRequest(&feedsv1.CreateFeedRequest{
			Url:  indexURL,
			Kind: feedsv1.FeedKind_FEED_KIND_SCRAPE,
		}),
	)
	require.NoError(t, err)

	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) && countSeenRows(t, created.Msg.Feed.Id) < 1 {
		time.Sleep(20 * time.Millisecond)
	}
	require.Equal(
		t, 1, countSeenRows(t, created.Msg.Feed.Id),
		"failed link should still be marked seen",
	)

	items, err := client.ListFeedItems(
		context.Background(), connect.NewRequest(&feedsv1.ListFeedItemsRequest{}),
	)
	require.NoError(t, err)
	for _, item := range items.Msg.Items {
		assert.NotEqual(
			t, created.Msg.Feed.Id, item.FeedId,
			"error/skip dedup marker should not be user-visible",
		)
	}
}

func TestRefreshFeed_Scrape_NotModified(t *testing.T) {
	base := uniqueBlogBase()
	indexURL := base + "/blog-nm"
	postURL := base + "/posts/nm-post-with-a-long-title"
	mockWebFetch.SetHTML(indexURL, blogIndexHTML(postURL, "NM post with a long title"))
	mockWebFetch.SetHTML(postURL, articlePageHTML("NM Post"))

	client := newFeedsClient(t)
	created, err := client.CreateFeed(
		context.Background(),
		connect.NewRequest(&feedsv1.CreateFeedRequest{
			Url:  indexURL,
			Kind: feedsv1.FeedKind_FEED_KIND_SCRAPE,
		}),
	)
	require.NoError(t, err)
	waitForFeedImport(t, client, created.Msg.Feed.Id)

	mockWebFetch.SetNotModified(indexURL)
	refreshed, err := client.RefreshFeed(
		context.Background(),
		connect.NewRequest(&feedsv1.RefreshFeedRequest{FeedId: created.Msg.Feed.Id}),
	)
	require.NoError(t, err)
	assert.Equal(t, int32(0), refreshed.Msg.Ingested)
}

func TestRefreshFeed_Scrape_DiscoverFails(t *testing.T) {
	base := uniqueBlogBase()
	indexURL := base + "/blog-goes-empty"
	postURL := base + "/posts/seed-post-that-will-vanish"
	mockWebFetch.SetHTML(indexURL, blogIndexHTML(postURL, "Seed post that will vanish"))
	mockWebFetch.SetHTML(postURL, articlePageHTML("Seed Post"))

	client := newFeedsClient(t)
	created, err := client.CreateFeed(
		context.Background(),
		connect.NewRequest(&feedsv1.CreateFeedRequest{
			Url:  indexURL,
			Kind: feedsv1.FeedKind_FEED_KIND_SCRAPE,
		}),
	)
	require.NoError(t, err)
	waitForFeedImport(t, client, created.Msg.Feed.Id)

	// With no post-like links left, RefreshFeed must surface the error.
	mockWebFetch.SetHTML(
		indexURL, `<html><body><a href="/about">About</a></body></html>`,
	)
	_, err = client.RefreshFeed(
		context.Background(),
		connect.NewRequest(&feedsv1.RefreshFeedRequest{FeedId: created.Msg.Feed.Id}),
	)
	require.Error(t, err)
	assert.Equal(t, connect.CodeInvalidArgument, connect.CodeOf(err))
}

// blogIndexHTMLTwoLinks builds an index page with two post-like links.
func blogIndexHTMLTwoLinks(url1, title1, url2, title2 string) string {
	return `<!DOCTYPE html><html><head><title>Two Post Blog</title></head><body>` +
		`<article><a href="` + url1 + `">` + title1 + `</a></article>` +
		`<article><a href="` + url2 + `">` + title2 + `</a></article>` +
		`</body></html>`
}

func TestCreateFeed_Scrape_DedupesByCanonicalURL(t *testing.T) {
	base := uniqueBlogBase()
	indexURL := base + "/blog-utm"
	postURL := base + "/posts/tracked-post-with-a-long-title"
	// Two hrefs differing only in utm_ params must be ingested once.
	mockWebFetch.SetHTML(indexURL, blogIndexHTMLTwoLinks(
		postURL+"?utm_source=twitter", "Tracked post with a long title (twitter)",
		postURL+"?utm_source=newsletter", "Tracked post with a long title (newsletter)",
	))
	mockWebFetch.SetHTML(postURL, articlePageHTML("Tracked Post"))

	client := newFeedsClient(t)
	created, err := client.CreateFeed(
		context.Background(),
		connect.NewRequest(&feedsv1.CreateFeedRequest{
			Url:  indexURL,
			Kind: feedsv1.FeedKind_FEED_KIND_SCRAPE,
		}),
	)
	require.NoError(t, err)
	waitForFeedImport(t, client, created.Msg.Feed.Id)
	// Give the import time to wrongly ingest the duplicate, if it would.
	time.Sleep(50 * time.Millisecond)

	items, err := client.ListFeedItems(
		context.Background(), connect.NewRequest(&feedsv1.ListFeedItemsRequest{}),
	)
	require.NoError(t, err)
	count := 0
	for _, item := range items.Msg.Items {
		if item.FeedId == created.Msg.Feed.Id {
			count++
		}
	}
	assert.Equal(t, 1, count, "the two utm-tagged hrefs should dedupe to one item")
}

func TestCreateFeed_Scrape_CapsItemsPerPoll(t *testing.T) {
	base := uniqueBlogBase()
	indexURL := base + "/blog-cap"

	html := `<!DOCTYPE html><html><head><title>Cap Blog</title></head><body>`
	postURLs := make([]string, 0, 25)
	for i := 0; i < 25; i++ {
		id := uuid.NewString()
		postURL := base + "/posts/" + id
		postURLs = append(postURLs, postURL)
		html += `<article><a href="` + postURL + `">Cap post number with a long title ` +
			id + `</a></article>`
		mockWebFetch.SetHTML(postURL, articlePageHTML("Cap Post "+id))
	}
	html += `</body></html>`
	mockWebFetch.SetHTML(indexURL, html)

	client := newFeedsClient(t)
	created, err := client.CreateFeed(
		context.Background(),
		connect.NewRequest(&feedsv1.CreateFeedRequest{
			Url:  indexURL,
			Kind: feedsv1.FeedKind_FEED_KIND_SCRAPE,
		}),
	)
	require.NoError(t, err)

	// The import ingests maxItemsPerPoll and leaves the overflow unseen.
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) &&
		countSeenRows(t, created.Msg.Feed.Id) < 20 {
		time.Sleep(20 * time.Millisecond)
	}
	require.Equal(t, 20, countSeenRows(t, created.Msg.Feed.Id),
		"the per-poll cap's worth should be seen; overflow stays pending")

	resp, err := client.ListFeedItems(
		context.Background(), connect.NewRequest(&feedsv1.ListFeedItemsRequest{}),
	)
	require.NoError(t, err)
	withContent := 0
	for _, item := range resp.Msg.Items {
		if item.FeedId != created.Msg.Feed.Id {
			continue
		}
		withContent++
	}
	assert.Equal(
		t,
		20,
		withContent,
		"only the per-poll cap's worth should be listed",
	)

	// The next poll backfills the leftover five.
	require.NoError(t, testApp.RunPollNow(context.Background()))
	require.Eventually(t, func() bool {
		return countSeenRows(t, created.Msg.Feed.Id) == len(postURLs)
	}, 5*time.Second, 20*time.Millisecond,
		"the next poll must pick up the pending overflow — never mark it seen")

	resp, err = client.ListFeedItems(
		context.Background(), connect.NewRequest(&feedsv1.ListFeedItemsRequest{}),
	)
	require.NoError(t, err)
	withContent = 0
	for _, item := range resp.Msg.Items {
		if item.FeedId != created.Msg.Feed.Id {
			continue
		}
		withContent++
	}
	assert.Equal(t, len(postURLs), withContent,
		"the second poll must backfill what the first poll's cap deferred")
}

func TestCreateFeed_Scrape_TitleUsesAnchorText(t *testing.T) {
	base := uniqueBlogBase()
	indexURL := base + "/blog-no-title"
	postURL := base + "/posts/untitled-post-with-a-long-title"
	mockWebFetch.SetHTML(
		indexURL, blogIndexHTML(postURL, "Untitled post with a long anchor title"),
	)
	// The page has no <title>/<h1>, proving the anchor text wins over the
	// URL fallback.
	mockWebFetch.SetHTML(postURL, `<!DOCTYPE html><html><body><article><p>`+
		`Lorem ipsum dolor sit amet, consectetur adipiscing elit. Sed do `+
		`eiusmod tempor incididunt ut labore et dolore magna aliqua. Ut `+
		`enim ad minim veniam, quis nostrud exercitation ullamco laboris `+
		`nisi ut aliquip ex ea commodo consequat.</p></article></body></html>`)

	client := newFeedsClient(t)
	created, err := client.CreateFeed(
		context.Background(),
		connect.NewRequest(&feedsv1.CreateFeedRequest{
			Url:  indexURL,
			Kind: feedsv1.FeedKind_FEED_KIND_SCRAPE,
		}),
	)
	require.NoError(t, err)
	waitForFeedImport(t, client, created.Msg.Feed.Id)

	items, err := client.ListFeedItems(
		context.Background(), connect.NewRequest(&feedsv1.ListFeedItemsRequest{}),
	)
	require.NoError(t, err)
	var found *feedsv1.Item
	for _, item := range items.Msg.Items {
		if item.SourceUrl == postURL {
			found = item
		}
	}
	require.NotNil(t, found)
	assert.Equal(t, "Untitled post with a long anchor title", found.Title)
}

func TestRefreshFeed_Scrape_NewPost_Ingests(t *testing.T) {
	base := uniqueBlogBase()
	indexURL := base + "/blog-refresh"
	seedURL := base + "/posts/seed-post-with-a-long-title"
	mockWebFetch.SetHTML(
		indexURL,
		blogIndexHTML(seedURL, "Seed post with a long title"),
	)
	mockWebFetch.SetHTML(seedURL, articlePageHTML("Seed Post"))

	client := newFeedsClient(t)
	created, err := client.CreateFeed(
		context.Background(),
		connect.NewRequest(&feedsv1.CreateFeedRequest{
			Url:  indexURL,
			Kind: feedsv1.FeedKind_FEED_KIND_SCRAPE,
		}),
	)
	require.NoError(t, err)
	waitForFeedImport(t, client, created.Msg.Feed.Id)

	newURL := base + "/posts/new-post-with-a-long-title-too"
	mockWebFetch.SetHTML(
		indexURL, blogIndexHTML(newURL, "New post with a long title too"),
	)
	mockWebFetch.SetHTML(newURL, articlePageHTML("New Post"))

	refreshed, err := client.RefreshFeed(
		context.Background(),
		connect.NewRequest(&feedsv1.RefreshFeedRequest{FeedId: created.Msg.Feed.Id}),
	)
	require.NoError(t, err)
	assert.Equal(t, int32(1), refreshed.Msg.Ingested)
}

func TestRefreshFeed_InvalidID(t *testing.T) {
	client := newFeedsClient(t)
	_, err := client.RefreshFeed(
		context.Background(),
		connect.NewRequest(&feedsv1.RefreshFeedRequest{FeedId: "not-a-uuid"}),
	)
	require.Error(t, err)
	assert.Equal(t, connect.CodeInvalidArgument, connect.CodeOf(err))
}

func TestRefreshFeed_UnknownFeed_NotFound(t *testing.T) {
	client := newFeedsClient(t)
	_, err := client.RefreshFeed(
		context.Background(),
		connect.NewRequest(&feedsv1.RefreshFeedRequest{FeedId: uuid.NewString()}),
	)
	require.Error(t, err)
	assert.Equal(t, connect.CodeNotFound, connect.CodeOf(err))
}

func TestRefreshFeed_NotModified_IngestsNothing(t *testing.T) {
	base := uniqueBlogBase()
	feedURL := base + "/feed-nm.xml"
	mockWebFetch.SetBody(feedURL, "application/rss+xml", []byte(rssXML(
		"NM Blog", rssItem{"Post", base + "/nm1", "nm1", itemContent},
	)))

	client := newFeedsClient(t)
	created, err := client.CreateFeed(
		context.Background(),
		connect.NewRequest(&feedsv1.CreateFeedRequest{Url: feedURL}),
	)
	require.NoError(t, err)

	mockWebFetch.SetNotModified(feedURL)
	refreshed, err := client.RefreshFeed(
		context.Background(),
		connect.NewRequest(&feedsv1.RefreshFeedRequest{FeedId: created.Msg.Feed.Id}),
	)
	require.NoError(t, err)
	assert.Equal(t, int32(0), refreshed.Msg.Ingested)
}

func TestRefreshFeed_NewPost_Ingests(t *testing.T) {
	base := uniqueBlogBase()
	feedURL := base + "/feed-refresh.xml"
	mockWebFetch.SetBody(feedURL, "application/rss+xml", []byte(rssXML(
		"Refresh Blog", rssItem{"Seed", base + "/seed", "seed", itemContent},
	)))

	client := newFeedsClient(t)
	created, err := client.CreateFeed(
		context.Background(),
		connect.NewRequest(&feedsv1.CreateFeedRequest{Url: feedURL}),
	)
	require.NoError(t, err)
	waitForFeedImport(t, client, created.Msg.Feed.Id)

	mockWebFetch.SetBody(feedURL, "application/rss+xml", []byte(rssXML(
		"Refresh Blog",
		rssItem{"Seed", base + "/seed", "seed", itemContent},
		rssItem{"Fresh", base + "/fresh", "fresh", itemContent},
	)))

	refreshed, err := client.RefreshFeed(
		context.Background(),
		connect.NewRequest(&feedsv1.RefreshFeedRequest{FeedId: created.Msg.Feed.Id}),
	)
	require.NoError(t, err)
	assert.Equal(t, int32(1), refreshed.Msg.Ingested)
}

func TestListFeeds_ExposesPollHealthFields(t *testing.T) {
	base := uniqueBlogBase()
	feedURL := base + "/feed-health.xml"
	//nolint:exhaustruct // NotModified false is the zero value
	mockWebFetch.Responses[feedURL] = &webfetch.Result{
		Body: []byte(rssXML(
			"Health Blog", rssItem{"Post", base + "/h1", "h1", itemContent},
		)),
		ContentType:  "application/rss+xml",
		FinalURL:     feedURL,
		ETag:         `"v1"`,
		LastModified: "Wed, 21 Oct 2015 07:28:00 GMT",
	}

	client := newFeedsClient(t)
	created, err := client.CreateFeed(
		context.Background(),
		connect.NewRequest(&feedsv1.CreateFeedRequest{Url: feedURL}),
	)
	require.NoError(t, err)
	waitForFeedImport(t, client, created.Msg.Feed.Id)

	found := waitForFeedPollHealth(t, client, created.Msg.Feed.Id)
	assert.Equal(t, `"v1"`, found.Etag)
	assert.Equal(t, "Wed, 21 Oct 2015 07:28:00 GMT", found.LastModified)
	assert.Equal(t, int32(0), found.ConsecutiveFailures)
	assert.Empty(t, found.NotifiedAt)

	mockWebFetch.Errs[feedURL] = assert.AnError
	_, err = client.RefreshFeed(
		context.Background(),
		connect.NewRequest(&feedsv1.RefreshFeedRequest{FeedId: created.Msg.Feed.Id}),
	)
	require.Error(t, err)

	list, err := client.ListFeeds(
		context.Background(), connect.NewRequest(&feedsv1.ListFeedsRequest{}),
	)
	require.NoError(t, err)
	found = findFeed(list.Msg.Feeds, created.Msg.Feed.Id)
	require.NotNil(t, found)
	assert.Contains(t, found.LastError, assert.AnError.Error())
	assert.Equal(t, int32(1), found.ConsecutiveFailures)

	_, err = testDB.Exec(
		context.Background(),
		"UPDATE feeds.feeds SET notified_at = now() WHERE id = $1",
		found.Id,
	)
	require.NoError(t, err)

	list, err = client.ListFeeds(
		context.Background(), connect.NewRequest(&feedsv1.ListFeedsRequest{}),
	)
	require.NoError(t, err)
	found = findFeed(list.Msg.Feeds, created.Msg.Feed.Id)
	require.NotNil(t, found)
	assert.NotEmpty(t, found.NotifiedAt)
}

func findFeed(feeds []*feedsv1.Feed, id string) *feedsv1.Feed {
	for _, f := range feeds {
		if f.Id == id {
			return f
		}
	}
	return nil
}

func TestUpdateFeed_Success(t *testing.T) {
	feedURL := uniqueBlogBase() + "/feed.xml"
	mockWebFetch.SetBody(feedURL, "application/rss+xml", []byte(rssXML("Old Title")))

	client := newFeedsClient(t)
	created, err := client.CreateFeed(
		context.Background(),
		connect.NewRequest(&feedsv1.CreateFeedRequest{Url: feedURL}),
	)
	require.NoError(t, err)

	_, err = client.UpdateFeed(
		context.Background(),
		connect.NewRequest(&feedsv1.UpdateFeedRequest{
			FeedId: created.Msg.Feed.Id, Title: "New Title",
		}),
	)
	require.NoError(t, err)

	list, err := client.ListFeeds(
		context.Background(), connect.NewRequest(&feedsv1.ListFeedsRequest{}),
	)
	require.NoError(t, err)
	var found *feedsv1.Feed
	for _, f := range list.Msg.Feeds {
		if f.Id == created.Msg.Feed.Id {
			found = f
		}
	}
	require.NotNil(t, found)
	assert.Equal(t, "New Title", found.Title)
}

func TestUpdateFeed_NotFound(t *testing.T) {
	client := newFeedsClient(t)
	_, err := client.UpdateFeed(
		context.Background(),
		connect.NewRequest(&feedsv1.UpdateFeedRequest{
			FeedId: uuid.NewString(), Title: "Whatever",
		}),
	)
	require.Error(t, err)
	assert.Equal(t, connect.CodeNotFound, connect.CodeOf(err))
}

func TestDeleteFeed_Success(t *testing.T) {
	feedURL := uniqueBlogBase() + "/feed.xml"
	mockWebFetch.SetBody(feedURL, "application/rss+xml", []byte(rssXML("Delete Me")))

	client := newFeedsClient(t)
	created, err := client.CreateFeed(
		context.Background(),
		connect.NewRequest(&feedsv1.CreateFeedRequest{Url: feedURL}),
	)
	require.NoError(t, err)

	_, err = client.DeleteFeed(
		context.Background(),
		connect.NewRequest(&feedsv1.DeleteFeedRequest{FeedId: created.Msg.Feed.Id}),
	)
	require.NoError(t, err)

	_, err = client.RefreshFeed(
		context.Background(),
		connect.NewRequest(&feedsv1.RefreshFeedRequest{FeedId: created.Msg.Feed.Id}),
	)
	require.Error(t, err)
	assert.Equal(t, connect.CodeNotFound, connect.CodeOf(err))
}

func TestDeleteFeed_NotFound(t *testing.T) {
	client := newFeedsClient(t)
	_, err := client.DeleteFeed(
		context.Background(),
		connect.NewRequest(&feedsv1.DeleteFeedRequest{FeedId: uuid.NewString()}),
	)
	require.Error(t, err)
	assert.Equal(t, connect.CodeNotFound, connect.CodeOf(err))
}

func TestCreateFeed_Email_Success(t *testing.T) {
	client := newFeedsClient(t)
	resp, err := client.CreateFeed(
		context.Background(),
		connect.NewRequest(&feedsv1.CreateFeedRequest{
			Kind: feedsv1.FeedKind_FEED_KIND_EMAIL, Title: "My Newsletter",
		}),
	)
	require.NoError(t, err)
	assert.Equal(t, "email", resp.Msg.Feed.SourceType)
	assert.Equal(t, "My Newsletter", resp.Msg.Feed.Title)
	assert.Contains(t, resp.Msg.Feed.InboundAddress, "@mail.example.com")
}

func TestCreateFeed_Email_URLMustBeEmpty(t *testing.T) {
	client := newFeedsClient(t)
	_, err := client.CreateFeed(
		context.Background(),
		connect.NewRequest(&feedsv1.CreateFeedRequest{
			Kind: feedsv1.FeedKind_FEED_KIND_EMAIL,
			Url:  "https://example.com/feed.xml",
		}),
	)
	require.Error(t, err)
	assert.Equal(t, connect.CodeInvalidArgument, connect.CodeOf(err))
}

// createItem imports one feed with a single item and returns its item ID.
func createItem(t *testing.T, client feedsv1connect.FeedServiceClient) string {
	t.Helper()
	base := uniqueBlogBase()
	feedURL := base + "/feed.xml"
	mockWebFetch.SetBody(feedURL, "application/rss+xml", []byte(rssXML(
		"Item Blog",
		rssItem{"Post", base + "/one", "guid-" + uuid.NewString(), itemContent},
	)))

	created, err := client.CreateFeed(
		context.Background(),
		connect.NewRequest(&feedsv1.CreateFeedRequest{Url: feedURL}),
	)
	require.NoError(t, err)
	waitForFeedImport(t, client, created.Msg.Feed.Id)

	items, err := client.ListFeedItems(
		context.Background(), connect.NewRequest(&feedsv1.ListFeedItemsRequest{}),
	)
	require.NoError(t, err)
	for _, item := range items.Msg.Items {
		if item.FeedId == created.Msg.Feed.Id {
			return item.Id
		}
	}
	t.Fatal("created item not found")
	return ""
}

// createItemAndFeed is createItem plus the feed ID.
func createItemAndFeed(
	t *testing.T, client feedsv1connect.FeedServiceClient,
) (string, string) {
	t.Helper()
	base := uniqueBlogBase()
	feedURL := base + "/feed.xml"
	mockWebFetch.SetBody(feedURL, "application/rss+xml", []byte(rssXML(
		"Item Blog",
		rssItem{"Post", base + "/one", "guid-" + uuid.NewString(), itemContent},
	)))

	created, err := client.CreateFeed(
		context.Background(),
		connect.NewRequest(&feedsv1.CreateFeedRequest{Url: feedURL}),
	)
	require.NoError(t, err)
	waitForFeedImport(t, client, created.Msg.Feed.Id)

	items, err := client.ListFeedItems(
		context.Background(), connect.NewRequest(&feedsv1.ListFeedItemsRequest{}),
	)
	require.NoError(t, err)
	for _, item := range items.Msg.Items {
		if item.FeedId == created.Msg.Feed.Id {
			return item.Id, created.Msg.Feed.Id
		}
	}
	t.Fatal("created item not found")
	return "", ""
}

func TestUpdateItem_MarkRead(t *testing.T) {
	client := newFeedsClient(t)
	itemID := createItem(t, client)

	read := true
	resp, err := client.UpdateItem(
		context.Background(),
		connect.NewRequest(&feedsv1.UpdateItemRequest{ItemId: itemID, Read: &read}),
	)
	require.NoError(t, err)
	assert.NotEmpty(t, resp.Msg.Item.ReadAt)
}

func TestUpdateItem_MarkUnread(t *testing.T) {
	client := newFeedsClient(t)
	itemID := createItem(t, client)

	read := true
	_, err := client.UpdateItem(
		context.Background(),
		connect.NewRequest(&feedsv1.UpdateItemRequest{ItemId: itemID, Read: &read}),
	)
	require.NoError(t, err)

	unread := false
	resp, err := client.UpdateItem(
		context.Background(),
		connect.NewRequest(&feedsv1.UpdateItemRequest{ItemId: itemID, Read: &unread}),
	)
	require.NoError(t, err)
	assert.Empty(t, resp.Msg.Item.ReadAt)
}

func TestUpdateItem_Dismiss(t *testing.T) {
	client := newFeedsClient(t)
	itemID := createItem(t, client)

	dismissed := true
	resp, err := client.UpdateItem(
		context.Background(),
		connect.NewRequest(&feedsv1.UpdateItemRequest{
			ItemId: itemID, Dismissed: &dismissed,
		}),
	)
	require.NoError(t, err)
	assert.True(t, resp.Msg.Item.Dismissed)
	assert.False(t, resp.Msg.Item.Bookmarked, "unset fields stay unchanged")
}

func TestUpdateItem_Bookmarked(t *testing.T) {
	client := newFeedsClient(t)
	itemID := createItem(t, client)

	bookmarked := true
	resp, err := client.UpdateItem(
		context.Background(),
		connect.NewRequest(&feedsv1.UpdateItemRequest{
			ItemId: itemID, Bookmarked: &bookmarked,
		}),
	)
	require.NoError(t, err)
	assert.True(t, resp.Msg.Item.Bookmarked)
}

func TestListFeedItems_ExcludesDismissed(t *testing.T) {
	client := newFeedsClient(t)
	itemID := createItem(t, client)

	dismissed := true
	_, err := client.UpdateItem(
		context.Background(),
		connect.NewRequest(
			&feedsv1.UpdateItemRequest{ItemId: itemID, Dismissed: &dismissed},
		),
	)
	require.NoError(t, err)

	resp, err := client.ListFeedItems(
		context.Background(), connect.NewRequest(&feedsv1.ListFeedItemsRequest{}),
	)
	require.NoError(t, err)
	for _, item := range resp.Msg.Items {
		assert.NotEqual(t, itemID, item.Id, "dismissed item must not be listed")
	}
}

func TestListFeedItems_UnreadOnly(t *testing.T) {
	client := newFeedsClient(t)
	itemID := createItem(t, client)

	read := true
	_, err := client.UpdateItem(
		context.Background(),
		connect.NewRequest(&feedsv1.UpdateItemRequest{ItemId: itemID, Read: &read}),
	)
	require.NoError(t, err)

	unreadOnly := true
	resp, err := client.ListFeedItems(
		context.Background(),
		connect.NewRequest(&feedsv1.ListFeedItemsRequest{UnreadOnly: &unreadOnly}),
	)
	require.NoError(t, err)
	for _, item := range resp.Msg.Items {
		assert.NotEqual(
			t,
			itemID,
			item.Id,
			"read item must not be listed when unread_only",
		)
	}

	resp, err = client.ListFeedItems(
		context.Background(), connect.NewRequest(&feedsv1.ListFeedItemsRequest{}),
	)
	require.NoError(t, err)
	assert.Contains(
		t,
		itemIDs(resp.Msg.Items),
		itemID,
		"unset unread_only returns read items too",
	)
}

func TestListFeedItems_BookmarkedOnly(t *testing.T) {
	client := newFeedsClient(t)
	itemID := createItem(t, client)

	bookmarkedOnly := true
	resp, err := client.ListFeedItems(
		context.Background(),
		connect.NewRequest(
			&feedsv1.ListFeedItemsRequest{BookmarkedOnly: &bookmarkedOnly},
		),
	)
	require.NoError(t, err)
	assert.NotContains(
		t,
		itemIDs(resp.Msg.Items),
		itemID,
		"unbookmarked item must not be listed when bookmarked_only",
	)

	bookmarked := true
	_, err = client.UpdateItem(
		context.Background(),
		connect.NewRequest(&feedsv1.UpdateItemRequest{
			ItemId: itemID, Bookmarked: &bookmarked,
		}),
	)
	require.NoError(t, err)

	resp, err = client.ListFeedItems(
		context.Background(),
		connect.NewRequest(
			&feedsv1.ListFeedItemsRequest{BookmarkedOnly: &bookmarkedOnly},
		),
	)
	require.NoError(t, err)
	assert.Contains(
		t,
		itemIDs(resp.Msg.Items),
		itemID,
		"bookmarked item is listed when bookmarked_only",
	)
}

// The test DB is shared across the package, so counts are relative.
func TestListFeedItems_Pagination(t *testing.T) {
	client := newFeedsClient(t)
	createItem(t, client)
	createItem(t, client)
	createItem(t, client)

	all, err := client.ListFeedItems(
		context.Background(),
		connect.NewRequest(&feedsv1.ListFeedItemsRequest{Limit: pagination.MaxLimit}),
	)
	require.NoError(t, err)
	total := len(all.Msg.Items)
	require.GreaterOrEqual(t, total, 3)

	resp, err := client.ListFeedItems(
		context.Background(),
		connect.NewRequest(&feedsv1.ListFeedItemsRequest{Limit: int32(total - 1)}),
	)
	require.NoError(t, err)
	assert.Len(t, resp.Msg.Items, total-1)
	assert.True(t, resp.Msg.HasMore)

	resp, err = client.ListFeedItems(
		context.Background(),
		connect.NewRequest(&feedsv1.ListFeedItemsRequest{Limit: int32(total)}),
	)
	require.NoError(t, err)
	assert.Len(t, resp.Msg.Items, total)
	assert.False(t, resp.Msg.HasMore)
}

// TestListFeedItems_FeedIDFilter: feed_id restricts results to one feed.
func TestListFeedItems_FeedIDFilter(t *testing.T) {
	client := newFeedsClient(t)
	itemA, feedA := createItemAndFeed(t, client)
	itemB, feedB := createItemAndFeed(t, client)
	require.NotEqual(t, feedA, feedB)

	resp, err := client.ListFeedItems(
		context.Background(),
		connect.NewRequest(&feedsv1.ListFeedItemsRequest{
			Limit: pagination.MaxLimit, FeedId: &feedA,
		}),
	)
	require.NoError(t, err)
	assert.Contains(t, itemIDs(resp.Msg.Items), itemA)
	assert.NotContains(t, itemIDs(resp.Msg.Items), itemB)
	for _, item := range resp.Msg.Items {
		assert.Equal(t, feedA, item.FeedId)
	}
}

func itemIDs(items []*feedsv1.Item) []string {
	out := make([]string, len(items))
	for i, item := range items {
		out[i] = item.Id
	}
	return out
}

func TestUpdateItem_InvalidID(t *testing.T) {
	client := newFeedsClient(t)
	read := true
	_, err := client.UpdateItem(
		context.Background(),
		connect.NewRequest(&feedsv1.UpdateItemRequest{
			ItemId: "not-a-uuid", Read: &read,
		}),
	)
	require.Error(t, err)
	assert.Equal(t, connect.CodeInvalidArgument, connect.CodeOf(err))
}

func TestUpdateItem_NotFound(t *testing.T) {
	client := newFeedsClient(t)
	read := true
	_, err := client.UpdateItem(
		context.Background(),
		connect.NewRequest(&feedsv1.UpdateItemRequest{
			ItemId: uuid.NewString(), Read: &read,
		}),
	)
	require.Error(t, err)
	assert.Equal(t, connect.CodeNotFound, connect.CodeOf(err))
}

// TestGetFeedItem_NotFound: an item deleted after listing maps to
// CodeNotFound.
func TestGetFeedItem_NotFound(t *testing.T) {
	client := newFeedsClient(t)
	_, err := client.GetFeedItem(
		context.Background(),
		connect.NewRequest(&feedsv1.GetFeedItemRequest{ItemId: uuid.NewString()}),
	)
	require.Error(t, err)
	assert.Equal(t, connect.CodeNotFound, connect.CodeOf(err))
}

// TestUpdateItem_ReadProgressClampsAndMonotonic: clamped to [0,100], never
// lowered.
func TestUpdateItem_ReadProgressClampsAndMonotonic(t *testing.T) {
	client := newFeedsClient(t)
	itemID := createItem(t, client)

	over := int32(150)
	resp, err := client.UpdateItem(
		context.Background(),
		connect.NewRequest(&feedsv1.UpdateItemRequest{
			ItemId: itemID, ReadProgressPct: &over,
		}),
	)
	require.NoError(t, err)
	assert.Equal(t, int32(100), resp.Msg.Item.ReadProgressPct)

	under := int32(-10)
	resp, err = client.UpdateItem(
		context.Background(),
		connect.NewRequest(&feedsv1.UpdateItemRequest{
			ItemId: itemID, ReadProgressPct: &under,
		}),
	)
	require.NoError(t, err)
	// Clamped to 0, but GREATEST against the existing 100 keeps it at 100.
	assert.Equal(t, int32(100), resp.Msg.Item.ReadProgressPct)
}

// TestGetFeedStats_Basic: item count and read rate aggregate per feed.
func TestGetFeedStats_Basic(t *testing.T) {
	client := newFeedsClient(t)
	base := uniqueBlogBase()
	feedURL := base + "/feed.xml"
	mockWebFetch.SetBody(feedURL, "application/rss+xml", []byte(rssXML(
		"Stats Blog", rssItem{"Post One", base + "/one", "guid-1", itemContent},
	)))

	created, err := client.CreateFeed(
		context.Background(),
		connect.NewRequest(&feedsv1.CreateFeedRequest{Url: feedURL}),
	)
	require.NoError(t, err)
	waitForFeedImport(t, client, created.Msg.Feed.Id)

	read := true
	items, err := client.ListFeedItems(
		context.Background(), connect.NewRequest(&feedsv1.ListFeedItemsRequest{}),
	)
	require.NoError(t, err)
	var itemID string
	for _, item := range items.Msg.Items {
		if item.FeedId == created.Msg.Feed.Id {
			itemID = item.Id
		}
	}
	require.NotEmpty(t, itemID)
	_, err = client.UpdateItem(
		context.Background(),
		connect.NewRequest(&feedsv1.UpdateItemRequest{ItemId: itemID, Read: &read}),
	)
	require.NoError(t, err)

	stats, err := client.GetFeedStats(
		context.Background(), connect.NewRequest(&feedsv1.GetFeedStatsRequest{}),
	)
	require.NoError(t, err)

	var found *feedsv1.FeedStats
	for _, s := range stats.Msg.Stats {
		if s.FeedId == created.Msg.Feed.Id {
			found = s
		}
	}
	require.NotNil(t, found)
	assert.Equal(t, int32(1), found.ItemCount)
	assert.InDelta(t, 1.0, found.ReadRate, 0.001)
}

func TestRefreshFeed_EmailFeed_NoOp(t *testing.T) {
	client := newFeedsClient(t)
	created, err := client.CreateFeed(
		context.Background(),
		connect.NewRequest(&feedsv1.CreateFeedRequest{
			Kind: feedsv1.FeedKind_FEED_KIND_EMAIL, Title: "Refresh No-op",
		}),
	)
	require.NoError(t, err)

	resp, err := client.RefreshFeed(
		context.Background(),
		connect.NewRequest(&feedsv1.RefreshFeedRequest{FeedId: created.Msg.Feed.Id}),
	)
	require.NoError(t, err)
	assert.Equal(t, int32(0), resp.Msg.Ingested)
}

// listFeedWithBody creates a one-item RSS feed with a large body.
func listFeedWithBody(t *testing.T, body string) (
	feedsv1connect.FeedServiceClient, *feedsv1.Item,
) {
	t.Helper()
	base := uniqueBlogBase()
	feedURL := base + "/feed-body.xml"
	mockWebFetch.SetBody(feedURL, "application/rss+xml", []byte(rssXML(
		"Body Blog", rssItem{"Big Post", base + "/big", uuid.NewString(), body},
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
		connect.NewRequest(&feedsv1.ListFeedItemsRequest{
			FeedId: proto.String(created.Msg.Feed.Id),
		}),
	)
	require.NoError(t, err)
	require.Len(t, items.Msg.Items, 1)
	return client, items.Msg.Items[0]
}

// A list read must never carry content_html, or every page drags every
// article body out of Postgres.
func TestListFeedItems_OmitsArticleBody(t *testing.T) {
	body := "<p>" + strings.Repeat("egress ", 5000) + "</p>"
	client, item := listFeedWithBody(t, body)

	assert.Empty(t, item.ContentHtml, "list responses must not carry the body")
	assert.True(t, item.HasContent, "but must still report that a body exists")

	full, err := client.GetFeedItem(
		context.Background(),
		connect.NewRequest(&feedsv1.GetFeedItemRequest{ItemId: item.Id}),
	)
	require.NoError(t, err)
	assert.Contains(t, full.Msg.Item.ContentHtml, "egress")
	assert.True(t, full.Msg.Item.HasContent)
}

// UpdateItem fires on every scroll tick, so it must not carry the body either.
func TestUpdateItem_OmitsArticleBody(t *testing.T) {
	client, item := listFeedWithBody(t, "<p>"+strings.Repeat("scroll ", 2000)+"</p>")

	updated, err := client.UpdateItem(
		context.Background(),
		connect.NewRequest(&feedsv1.UpdateItemRequest{
			ItemId:          item.Id,
			ReadProgressPct: proto.Int32(42),
		}),
	)
	require.NoError(t, err)
	assert.Empty(t, updated.Msg.Item.ContentHtml)
	assert.True(t, updated.Msg.Item.HasContent)
	assert.Equal(t, int32(42), updated.Msg.Item.ReadProgressPct)
}

// A body-less item reports HasContent=false, distinguishing "nothing stored"
// from "not loaded yet".
func TestFeedItem_NoBodyReportsHasContentFalse(t *testing.T) {
	client, seeded := listFeedWithBody(t, "<p>seed</p>")

	// Inserted directly: ingest paths mark body-less items with ingest_error.
	var itemID string
	err := testDB.QueryRow(context.Background(), `
		INSERT INTO feeds.items (feed_id, guid, title, source_url, content_html)
		VALUES ($1, $2, 'Bodyless Post', 'https://example.com/bodyless', '')
		RETURNING id
	`, seeded.FeedId, uuid.NewString()).Scan(&itemID)
	require.NoError(t, err)

	items, err := client.ListFeedItems(
		context.Background(),
		connect.NewRequest(&feedsv1.ListFeedItemsRequest{
			FeedId: proto.String(seeded.FeedId),
		}),
	)
	require.NoError(t, err)
	var listed *feedsv1.Item
	for _, item := range items.Msg.Items {
		if item.Id == itemID {
			listed = item
		}
	}
	require.NotNil(t, listed)
	assert.False(t, listed.HasContent)

	full, err := client.GetFeedItem(
		context.Background(),
		connect.NewRequest(&feedsv1.GetFeedItemRequest{ItemId: itemID}),
	)
	require.NoError(t, err)
	assert.Empty(t, full.Msg.Item.ContentHtml)
	assert.False(t, full.Msg.Item.HasContent)
}

func TestGetFeedItem_InvalidID(t *testing.T) {
	client := newFeedsClient(t)
	_, err := client.GetFeedItem(
		context.Background(),
		connect.NewRequest(&feedsv1.GetFeedItemRequest{ItemId: "not-a-uuid"}),
	)
	require.Error(t, err)
	assert.Equal(t, connect.CodeInvalidArgument, connect.CodeOf(err))
}

func TestGetFeedItem_UnknownItem_NotFound(t *testing.T) {
	client := newFeedsClient(t)
	_, err := client.GetFeedItem(
		context.Background(),
		connect.NewRequest(&feedsv1.GetFeedItemRequest{ItemId: uuid.NewString()}),
	)
	require.Error(t, err)
	assert.Equal(t, connect.CodeNotFound, connect.CodeOf(err))
}
