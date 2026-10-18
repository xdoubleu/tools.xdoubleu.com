package feeds_test

import (
	"context"
	"net/url"
	"testing"

	"connectrpc.com/connect/v2"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	feedsv1 "tools.xdoubleu.com/gen/feeds/v1"
	"tools.xdoubleu.com/gen/feeds/v1/feedsv1connect"
)

// arxivQueryURL is a unique, already-canonical arXiv API query URL.
func arxivQueryURL() string {
	q := url.Values{}
	q.Set("search_query", "cat:cs.AI AND abs:"+uuid.NewString())
	q.Set("sortBy", "submittedDate")
	q.Set("sortOrder", "descending")
	q.Set("max_results", "20")
	return "https://export.arxiv.org/api/query?" + q.Encode()
}

// arxivAtomEntry follows the arXiv API's documented Atom entry shape.
func arxivAtomEntry(id, title, summary string) string {
	return `<entry><id>http://arxiv.org/abs/` + id + `</id>` +
		`<published>2026-10-12T17:59:59Z</published>` +
		`<title>` + title + `</title><summary>` + summary + `</summary>` +
		`<author><name>Ada Lovelace</name></author>` +
		`<author><name>Alan Turing</name></author>` +
		`<link href="http://arxiv.org/abs/` + id + `" rel="alternate" ` +
		`type="text/html"/>` +
		`<link title="pdf" href="http://arxiv.org/pdf/` + id + `" ` +
		`rel="related" type="application/pdf"/>` +
		`<arxiv:primary_category xmlns:arxiv="http://arxiv.org/schemas/atom" ` +
		`term="cs.AI" scheme="http://arxiv.org/schemas/atom"/>` +
		`<category term="cs.AI" scheme="http://arxiv.org/schemas/atom"/>` +
		`<category term="cs.LG" scheme="http://arxiv.org/schemas/atom"/>` +
		`</entry>`
}

func arxivAtomFeed(entries ...string) string {
	body := `<?xml version="1.0" encoding="UTF-8"?>` +
		`<feed xmlns="http://www.w3.org/2005/Atom">` +
		`<title type="html">ArXiv Query: search_query=cat:cs.AI</title>` +
		`<id>http://arxiv.org/api/query</id>`
	for _, e := range entries {
		body += e
	}
	return body + `</feed>`
}

// arxivRSSItem follows rss.arxiv.org's documented item shape.
func arxivRSSItem(versionedID, announceType string) string {
	return `<item><title>Paper ` + versionedID + `</title>` +
		`<link>https://arxiv.org/abs/` + versionedID + `</link>` +
		`<description>arXiv:` + versionedID + ` Announce Type: ` + announceType +
		` Abstract: An abstract.</description>` +
		`<guid isPermaLink="false">oai:arXiv.org:` + versionedID + `</guid>` +
		`<category>cs.AI</category>` +
		`<arxiv:announce_type>` + announceType + `</arxiv:announce_type>` +
		`<dc:creator>Ada Lovelace, Alan Turing</dc:creator></item>`
}

func arxivRSSFeed(items ...string) string {
	body := `<?xml version="1.0" encoding="UTF-8"?>` +
		`<rss version="2.0" xmlns:arxiv="http://arxiv.org/schemas/atom" ` +
		`xmlns:dc="http://purl.org/dc/elements/1.1/"><channel>` +
		`<title>cs.AI updates on arXiv.org</title>`
	for _, it := range items {
		body += it
	}
	return body + `</channel></rss>`
}

// createArxivFeed subscribes and deletes the feed on cleanup: RunPollNow
// polls every stored feed, spacing arXiv ones arxivMinRequestGap apart.
func createArxivFeed(
	t *testing.T,
	client feedsv1connect.FeedServiceClient,
	req *feedsv1.CreateFeedRequest,
) *feedsv1.Feed {
	t.Helper()
	resp, err := client.CreateFeed(context.Background(), req)
	require.NoError(t, err)
	t.Cleanup(func() {
		_, _ = client.DeleteFeed(
			context.Background(),
			&feedsv1.DeleteFeedRequest{FeedId: resp.Feed.Id},
		)
	})
	return resp.Feed
}

func feedItems(
	t *testing.T,
	client feedsv1connect.FeedServiceClient,
	feedID string,
) []*feedsv1.Item {
	t.Helper()
	resp, err := client.ListFeedItems(
		context.Background(),
		&feedsv1.ListFeedItemsRequest{FeedId: &feedID},
	)
	require.NoError(t, err)
	return resp.Items
}

func TestCreateFeed_ArxivQuery_BuildsPaperItems(t *testing.T) {
	feedURL := arxivQueryURL()
	mockWebFetch.SetBody(feedURL, "application/atom+xml", []byte(arxivAtomFeed(
		arxivAtomEntry(
			"2610.01234v2",
			"Agents  that\n  plan",
			"  We study <agents> that\n  plan & act.  ",
		),
	)))

	client := newFeedsClient(t)
	feed := createArxivFeed(t, client,
		&feedsv1.CreateFeedRequest{Url: feedURL, Title: "arXiv: agents · cs.AI"},
	)
	assert.Equal(t, "arXiv: agents · cs.AI", feed.Title)
	waitForFeedImport(t, client, feed.Id)

	items := feedItems(t, client, feed.Id)
	require.Len(t, items, 1)
	item := items[0]
	assert.Equal(t, "Agents that plan", item.Title)
	assert.Equal(t, "https://arxiv.org/abs/2610.01234", item.SourceUrl)
	assert.Equal(t, []string{"cs.AI", "cs.LG"}, item.Categories)
	assert.Equal(t, "2026-10-12T17:59:59Z", item.PublishedAt)

	content := fetchItemContent(t, client, item.Id)
	assert.Contains(t, content, "<em>Ada Lovelace, Alan Turing</em>")
	assert.Contains(t, content, "We study &lt;agents&gt; that plan &amp; act.")
	assert.Contains(t, content, `href="https://arxiv.org/html/2610.01234"`)
	assert.Contains(t, content, `href="https://arxiv.org/pdf/2610.01234"`)
}

func TestCreateFeed_ArxivQuery_NoTitleKeepsFeedTitle(t *testing.T) {
	feedURL := arxivQueryURL()
	mockWebFetch.SetBody(feedURL, "application/atom+xml", []byte(arxivAtomFeed(
		arxivAtomEntry("2610.05555v1", "T", "S"),
	)))

	client := newFeedsClient(t)
	feed := createArxivFeed(t, client,
		&feedsv1.CreateFeedRequest{Url: feedURL, Title: "  "},
	)
	assert.Equal(t, "ArXiv Query: search_query=cat:cs.AI", feed.Title)
}

func TestCreateFeed_ArxivQuery_APIErrorRejected(t *testing.T) {
	feedURL := arxivQueryURL()
	mockWebFetch.SetBody(feedURL, "application/atom+xml", []byte(arxivAtomFeed(
		`<entry><id>http://arxiv.org/api/errors#malformed_query</id>`+
			`<title>Error</title><summary>malformed query</summary></entry>`,
	)))

	client := newFeedsClient(t)
	_, err := client.CreateFeed(
		context.Background(),
		&feedsv1.CreateFeedRequest{Url: feedURL},
	)
	require.Error(t, err)
	assert.Equal(t, connect.CodeInvalidArgument, connect.CodeOf(err))
	assert.Contains(t, err.Error(), "malformed query")
}

func TestRefreshFeed_ArxivRSS_SkipsReplacementsAndRevisions(t *testing.T) {
	feedURL := "https://rss.arxiv.org/rss/cs.AI?t=" + uuid.NewString()
	mockWebFetch.SetBody(feedURL, "application/rss+xml", []byte(arxivRSSFeed(
		arxivRSSItem("2610.00001v1", "new"),
		arxivRSSItem("2610.00002v3", "replace"),
		arxivRSSItem("2610.00003v2", "replace-cross"),
	)))

	client := newFeedsClient(t)
	created := createArxivFeed(t, client, &feedsv1.CreateFeedRequest{Url: feedURL})
	waitForFeedImport(t, client, created.Id)

	items := feedItems(t, client, created.Id)
	require.Len(t, items, 1)
	assert.Equal(t, "https://arxiv.org/abs/2610.00001", items[0].SourceUrl)
	content := fetchItemContent(t, client, items[0].Id)
	assert.Contains(t, content, "<p>An abstract.</p>")
	assert.NotContains(t, content, "Announce Type")

	// A later version of a seen paper, and a cross-listing, arrive.
	mockWebFetch.SetBody(feedURL, "application/rss+xml", []byte(arxivRSSFeed(
		arxivRSSItem("2610.00001v2", "new"),
		arxivRSSItem("2610.00004v1", "cross"),
	)))
	refreshed, err := client.RefreshFeed(
		context.Background(),
		&feedsv1.RefreshFeedRequest{FeedId: created.Id},
	)
	require.NoError(t, err)
	assert.Equal(t, int32(1), refreshed.Ingested)
	assert.Len(t, feedItems(t, client, created.Id), 2)
}

func TestRunPollNow_ArxivFeed_IngestsNewPaper(t *testing.T) {
	feedURL := "https://rss.arxiv.org/rss/cs.LG?t=" + uuid.NewString()
	mockWebFetch.SetBody(feedURL, "application/rss+xml", []byte(arxivRSSFeed(
		arxivRSSItem("2610.00011v1", "new"),
	)))

	client := newFeedsClient(t)
	created := createArxivFeed(t, client, &feedsv1.CreateFeedRequest{Url: feedURL})
	waitForFeedImport(t, client, created.Id)

	mockWebFetch.SetBody(feedURL, "application/rss+xml", []byte(arxivRSSFeed(
		arxivRSSItem("2610.00011v1", "new"),
		arxivRSSItem("2610.00012v1", "new"),
	)))
	require.NoError(t, testApp.RunPollNow(context.Background()))
	assert.Len(t, feedItems(t, client, created.Id), 2)
}
