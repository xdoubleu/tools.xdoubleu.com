package feeds_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"connectrpc.com/connect"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	feedsv1 "tools.xdoubleu.com/gen/feeds/v1"
	"tools.xdoubleu.com/gen/feeds/v1/feedsv1connect"
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

// articlePageHTML builds a minimal but readability-extractable HTML page —
// enough text/structure for go-readability to find an article body.
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

// waitForFeedImport polls ListFeedItems until the feed's background import
// (kicked off by CreateFeed) has landed at least one item, or fails the test
// after a timeout.
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

// ── ListFeeds / CreateFeed (RSS) ────────────────────────────────────────────

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
			assert.Contains(t, item.ContentHtml, "Lorem ipsum")
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
	assert.Contains(t, found.ContentHtml, "Lorem ipsum")
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

	// Every guid is marked seen even when capped (as an error-only row with
	// no content), so wait for all 25 to land before asserting the cap.
	var resp *connect.Response[feedsv1.ListFeedItemsResponse]
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		var listErr error
		resp, listErr = client.ListFeedItems(
			context.Background(), connect.NewRequest(&feedsv1.ListFeedItemsRequest{}),
		)
		require.NoError(t, listErr)
		total := 0
		for _, item := range resp.Msg.Items {
			if item.FeedId == created.Msg.Feed.Id {
				total++
			}
		}
		if total >= len(items) {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}

	withContent, seen := 0, 0
	for _, item := range resp.Msg.Items {
		if item.FeedId != created.Msg.Feed.Id {
			continue
		}
		seen++
		if item.ContentHtml != "" {
			withContent++
		}
	}
	require.Equal(t, len(items), seen, "every guid should be marked seen")
	assert.Equal(
		t,
		20,
		withContent,
		"only the per-poll cap's worth should have content",
	)
}

// ── RefreshFeed ──────────────────────────────────────────────────────────

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

// ── UpdateFeed / DeleteFeed ─────────────────────────────────────────────────

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

// ── Email feeds ─────────────────────────────────────────────────────────────

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

// ── UpdateItem ───────────────────────────────────────────────────────────────

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
	assert.False(t, resp.Msg.Item.Favourite, "unset fields stay unchanged")
}

func TestUpdateItem_Favourite(t *testing.T) {
	client := newFeedsClient(t)
	itemID := createItem(t, client)

	favourite := true
	resp, err := client.UpdateItem(
		context.Background(),
		connect.NewRequest(&feedsv1.UpdateItemRequest{
			ItemId: itemID, Favourite: &favourite,
		}),
	)
	require.NoError(t, err)
	assert.True(t, resp.Msg.Item.Favourite)
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
