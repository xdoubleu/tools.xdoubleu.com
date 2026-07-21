package reading_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"connectrpc.com/connect"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"tools.xdoubleu.com/apps/reading/internal/models"
	"tools.xdoubleu.com/apps/reading/pkg/arxiv"
	readingv1 "tools.xdoubleu.com/gen/reading/v1"
)

// uniqueBlogBase returns a per-run unique URL base. The test DB persists
// across runs, so fixed feed/item URLs would collide with rows from earlier
// runs (feeds are unique per (user, url); items dedup on source_url).
func uniqueBlogBase() string {
	return "https://blog.example.com/" + uuid.NewString()
}

// rssXML builds a minimal RSS 2.0 document with full-content items.
func rssXML(feedTitle string, items ...rssItem) string {
	body := ""
	for _, it := range items {
		content := ""
		if it.content != "" {
			content = "<content:encoded><![CDATA[" + it.content +
				"]]></content:encoded>"
		}
		body += fmt.Sprintf(
			`<item><title>%s</title><link>%s</link><guid>%s</guid>%s</item>`,
			it.title, it.link, it.guid, content,
		)
	}
	return `<?xml version="1.0"?>` +
		`<rss version="2.0" ` +
		`xmlns:content="http://purl.org/rss/1.0/modules/content/">` +
		`<channel><title>` + feedTitle + `</title>` + body + `</channel></rss>`
}

type rssItem struct {
	title   string
	link    string
	guid    string
	content string
}

const itemContent = `<p>Lorem ipsum dolor sit amet, consectetur adipiscing
elit, sed do eiusmod tempor incididunt ut labore et dolore magna aliqua.</p>`

func feedReq[T any](t *testing.T, msg *T) *connect.Request[T] {
	t.Helper()
	req := connect.NewRequest(msg)
	req.Header().Set("Cookie", accessToken.String())
	return req
}

// waitForFeedImport blocks until the feed's initial import (run in a
// detached goroutine by FeedService.Create, #430) has finished. Its last
// step, recordFetchResult, sets LastFetchedAt — so polling ListFeeds for
// that field is a reliable "import done" signal, including the case where
// nothing was ingested.
func waitForFeedImport(t *testing.T, client booksTestClient, feedID string) {
	t.Helper()
	require.Eventually(t, func() bool {
		list, err := client.ListFeeds(
			context.Background(), feedReq(t, &readingv1.ListFeedsRequest{}),
		)
		if err != nil {
			return false
		}
		for _, f := range list.Msg.Feeds {
			if f.Id == feedID {
				return f.LastFetchedAt != ""
			}
		}
		return false
	}, 5*time.Second, 20*time.Millisecond, "feed import did not complete in time")
}

// TestCreateFeed_ReturnsBeforeImportCompletes proves #430: subscribing
// returns as soon as the feed is validated and stored, without waiting for
// the (potentially slow) item import — previously CreateFeed blocked until
// every item was ingested, which could exceed the server's write timeout and
// report a false failure even though the feed was added correctly.
func TestCreateFeed_ReturnsBeforeImportCompletes(t *testing.T) {
	base := uniqueBlogBase()
	feedURL := base + "/feed-async.xml"
	itemURL := base + "/async-post"
	mockWebFetch.SetBody(feedURL, "application/rss+xml", []byte(rssXML(
		"Async Blog",
		// No embedded content: ingestItemContent fetches the linked page,
		// which is where we gate the import below.
		rssItem{"Async Post", itemURL, "a1", ""},
	)))
	mockWebFetch.SetHTML(itemURL, "<html><body><p>content</p></body></html>")

	gate := make(chan struct{})
	if mockWebFetch.Gates == nil {
		mockWebFetch.Gates = map[string]chan struct{}{}
	}
	mockWebFetch.Gates[itemURL] = gate
	t.Cleanup(func() { delete(mockWebFetch.Gates, itemURL) })

	client := newBooksTestClient(t)
	start := time.Now()
	resp, err := client.CreateFeed(
		context.Background(),
		feedReq(t, &readingv1.CreateFeedRequest{Url: feedURL, KoboSync: false}),
	)
	require.NoError(t, err)
	assert.Less(t, time.Since(start), 2*time.Second,
		"CreateFeed must return before the item import finishes")

	// The import is still blocked on the gate: nothing has landed yet.
	_, err = testApp.Repositories.Books.GetBookBySourceURL(
		context.Background(), itemURL,
	)
	assert.Error(t, err)

	close(gate)
	waitForFeedImport(t, client, resp.Msg.Feed.Id)

	book, err := testApp.Repositories.Books.GetBookBySourceURL(
		context.Background(), itemURL,
	)
	require.NoError(t, err)
	assert.Equal(t, "Async Post", book.Title)
}

func TestCreateFeed_ImportsCurrentContents(t *testing.T) {
	base := uniqueBlogBase()
	feedURL := base + "/feed-import.xml"
	mockWebFetch.SetBody(feedURL, "application/rss+xml", []byte(rssXML(
		"Import Blog",
		rssItem{"Post One", base + "/one", "g1", itemContent},
		rssItem{"Post Two", base + "/two", "g2", itemContent},
	)))

	client := newBooksTestClient(t)
	resp, err := client.CreateFeed(
		context.Background(),
		feedReq(t, &readingv1.CreateFeedRequest{Url: feedURL, KoboSync: false}),
	)
	require.NoError(t, err)
	assert.Equal(t, "Import Blog", resp.Msg.Feed.Title)
	waitForFeedImport(t, client, resp.Msg.Feed.Id)

	// Both posts landed as rss items with stored EPUBs, status to-read.
	book, err := testApp.Repositories.Books.GetBookBySourceURL(
		context.Background(), base+"/one",
	)
	require.NoError(t, err)
	assert.Equal(t, models.CategoryRSS, book.Category)
	ub, err := testApp.Repositories.Books.GetUserBook(
		context.Background(), userID, book.ID,
	)
	require.NoError(t, err)
	assert.Equal(t, models.StatusToRead, ub.Status)
	assert.NotContains(t, ub.Tags, models.TagKoboSync)
}

func TestCreateFeed_DuplicateAndInvalid(t *testing.T) {
	feedURL := uniqueBlogBase() + "/feed-dup.xml"
	mockWebFetch.SetBody(
		feedURL, "application/rss+xml", []byte(rssXML("Dup Blog")),
	)

	client := newBooksTestClient(t)
	_, err := client.CreateFeed(
		context.Background(),
		feedReq(t, &readingv1.CreateFeedRequest{Url: feedURL, KoboSync: false}),
	)
	require.NoError(t, err)

	_, err = client.CreateFeed(
		context.Background(),
		feedReq(t, &readingv1.CreateFeedRequest{Url: feedURL, KoboSync: false}),
	)
	require.Error(t, err)
	assert.Equal(t, connect.CodeAlreadyExists, connect.CodeOf(err))

	notAFeed := uniqueBlogBase() + "/not-a-feed"
	mockWebFetch.SetHTML(notAFeed, "<html><body>not xml</body></html>")
	_, err = client.CreateFeed(
		context.Background(),
		feedReq(t, &readingv1.CreateFeedRequest{Url: notAFeed, KoboSync: false}),
	)
	require.Error(t, err)
	assert.Equal(t, connect.CodeInvalidArgument, connect.CodeOf(err))
}

func TestRefreshFeed_IngestsOnlyNewItems(t *testing.T) {
	base := uniqueBlogBase()
	feedURL := base + "/feed-refresh.xml"
	mockWebFetch.SetBody(feedURL, "application/rss+xml", []byte(rssXML(
		"Refresh Blog",
		rssItem{"First", base + "/r1", "r1", itemContent},
	)))

	client := newBooksTestClient(t)
	created, err := client.CreateFeed(
		context.Background(),
		feedReq(t, &readingv1.CreateFeedRequest{Url: feedURL, KoboSync: false}),
	)
	require.NoError(t, err)
	feedID := created.Msg.Feed.Id
	waitForFeedImport(t, client, feedID)

	// A new post appears in the feed.
	mockWebFetch.SetBody(feedURL, "application/rss+xml", []byte(rssXML(
		"Refresh Blog",
		rssItem{"First", base + "/r1", "r1", itemContent},
		rssItem{"Second", base + "/r2", "r2", itemContent},
	)))

	refreshed, err := client.RefreshFeed(
		context.Background(),
		feedReq(t, &readingv1.RefreshFeedRequest{FeedId: feedID}),
	)
	require.NoError(t, err)
	assert.Equal(t, int32(1), refreshed.Msg.Ingested)

	// Refreshing again ingests nothing — everything is seen.
	again, err := client.RefreshFeed(
		context.Background(),
		feedReq(t, &readingv1.RefreshFeedRequest{FeedId: feedID}),
	)
	require.NoError(t, err)
	assert.Equal(t, int32(0), again.Msg.Ingested)
}

// TestCreateFeed_KoboSync_EndToEnd proves the plan's core claim: an item
// ingested from a kobo-sync feed ends up in ListKoboSyncBooks with a ready
// KEPUB, without any change to the Kobo sync layer.
func TestCreateFeed_KoboSync_EndToEnd(t *testing.T) {
	base := uniqueBlogBase()
	feedURL := base + "/feed-kobo.xml"
	itemURL := base + "/kobo-post"
	mockWebFetch.SetBody(feedURL, "application/rss+xml", []byte(rssXML(
		"Kobo Blog",
		rssItem{"Kobo Post", itemURL, "k1", itemContent},
	)))

	client := newBooksTestClient(t)
	resp, err := client.CreateFeed(
		context.Background(),
		feedReq(t, &readingv1.CreateFeedRequest{Url: feedURL, KoboSync: true}),
	)
	require.NoError(t, err)
	waitForFeedImport(t, client, resp.Msg.Feed.Id)

	book, err := testApp.Repositories.Books.GetBookBySourceURL(
		context.Background(), itemURL,
	)
	require.NoError(t, err)

	ub, err := testApp.Repositories.Books.GetUserBook(
		context.Background(), userID, book.ID,
	)
	require.NoError(t, err)
	assert.Contains(t, ub.Tags, models.TagKoboSync)

	syncBooks, err := testApp.Repositories.Books.ListKoboSyncBooks(
		context.Background(), userID,
	)
	require.NoError(t, err)
	found := false
	for _, sb := range syncBooks {
		if sb.BookID == book.ID {
			found = true
			assert.Equal(t, models.FileFormatKEPUB, sb.Format)
		}
	}
	assert.True(t, found, "feed item must appear in the Kobo sync library")
}

func TestUpdateListDeleteFeed(t *testing.T) {
	feedURL := uniqueBlogBase() + "/feed-crud.xml"
	mockWebFetch.SetBody(
		feedURL, "application/rss+xml", []byte(rssXML("CRUD Blog")),
	)

	client := newBooksTestClient(t)
	created, err := client.CreateFeed(
		context.Background(),
		feedReq(t, &readingv1.CreateFeedRequest{Url: feedURL, KoboSync: false}),
	)
	require.NoError(t, err)
	feedID := created.Msg.Feed.Id
	waitForFeedImport(t, client, feedID)

	_, err = client.UpdateFeed(
		context.Background(),
		feedReq(t, &readingv1.UpdateFeedRequest{
			FeedId:   feedID,
			Title:    "Renamed Blog",
			KoboSync: true,
		}),
	)
	require.NoError(t, err)

	list, err := client.ListFeeds(
		context.Background(), feedReq(t, &readingv1.ListFeedsRequest{}),
	)
	require.NoError(t, err)
	var got *readingv1.Feed
	for _, f := range list.Msg.Feeds {
		if f.Id == feedID {
			got = f
		}
	}
	require.NotNil(t, got)
	assert.Equal(t, "Renamed Blog", got.Title)
	assert.True(t, got.KoboSync)
	assert.NotEmpty(t, got.LastFetchedAt)
	assert.Empty(t, got.LastError)

	_, err = client.DeleteFeed(
		context.Background(),
		feedReq(t, &readingv1.DeleteFeedRequest{FeedId: feedID}),
	)
	require.NoError(t, err)

	_, err = client.RefreshFeed(
		context.Background(),
		feedReq(t, &readingv1.RefreshFeedRequest{FeedId: feedID}),
	)
	require.Error(t, err)
	assert.Equal(t, connect.CodeNotFound, connect.CodeOf(err))
}

// TestDeleteFeed_RemovesUntouchedItems proves #405: deleting a feed removes the
// library items it ingested, except any the user read or favourited.
func TestDeleteFeed_RemovesUntouchedItems(t *testing.T) {
	base := uniqueBlogBase()
	feedURL := base + "/feed-cascade.xml"
	untouched := base + "/untouched"
	readItem := base + "/read"
	favItem := base + "/fav"
	mockWebFetch.SetBody(feedURL, "application/rss+xml", []byte(rssXML(
		"Cascade Blog",
		rssItem{"Untouched", untouched, "c1", itemContent},
		rssItem{"Read", readItem, "c2", itemContent},
		rssItem{"Fav", favItem, "c3", itemContent},
	)))

	client := newBooksTestClient(t)
	created, err := client.CreateFeed(
		context.Background(),
		feedReq(t, &readingv1.CreateFeedRequest{Url: feedURL, KoboSync: false}),
	)
	require.NoError(t, err)
	waitForFeedImport(t, client, created.Msg.Feed.Id)

	bookID := func(sourceURL string) uuid.UUID {
		book, bookErr := testApp.Repositories.Books.GetBookBySourceURL(
			context.Background(), sourceURL,
		)
		require.NoError(t, bookErr)
		return book.ID
	}
	untouchedID := bookID(untouched)
	readID := bookID(readItem)
	favID := bookID(favItem)

	// Engage with two of the three items: mark one read, favourite the other.
	_, err = client.UpdateBookStatus(context.Background(), feedReq(t,
		&readingv1.UpdateBookStatusRequest{
			BookId: readID.String(), Status: models.StatusRead,
		}))
	require.NoError(t, err)
	_, err = client.UpdateBookStatus(context.Background(), feedReq(t,
		&readingv1.UpdateBookStatusRequest{
			BookId: favID.String(), Status: models.StatusToRead, Favourite: true,
		}))
	require.NoError(t, err)

	_, err = client.DeleteFeed(
		context.Background(),
		feedReq(t, &readingv1.DeleteFeedRequest{FeedId: created.Msg.Feed.Id}),
	)
	require.NoError(t, err)

	// The untouched item is gone; the read and favourited items survive.
	_, err = testApp.Repositories.Books.GetUserBook(
		context.Background(), userID, untouchedID,
	)
	require.Error(t, err)

	readUB, err := testApp.Repositories.Books.GetUserBook(
		context.Background(), userID, readID,
	)
	require.NoError(t, err)
	assert.Equal(t, models.StatusRead, readUB.Status)

	favUB, err := testApp.Repositories.Books.GetUserBook(
		context.Background(), userID, favID,
	)
	require.NoError(t, err)
	assert.Contains(t, favUB.Tags, models.TagFavourite)
}

// TestCreateFeed_ArxivItemsBecomePapers proves #406: a feed item linking to an
// arXiv paper is ingested as a "paper" with a PDF, not a readability "rss"
// article.
func TestCreateFeed_ArxivItemsBecomePapers(t *testing.T) {
	id := uniqueArxivID()
	registerMockPaper(id, "A Feed-Ingested Paper", "Grace Hopper")

	base := uniqueBlogBase()
	feedURL := base + "/arxiv-feed.xml"
	mockWebFetch.SetBody(feedURL, "application/rss+xml", []byte(rssXML(
		"arXiv Feed",
		rssItem{"A Feed-Ingested Paper", arxiv.AbsURL(id), arxiv.AbsURL(id), ""},
	)))

	client := newBooksTestClient(t)
	resp, err := client.CreateFeed(
		context.Background(),
		feedReq(t, &readingv1.CreateFeedRequest{Url: feedURL, KoboSync: false}),
	)
	require.NoError(t, err)
	waitForFeedImport(t, client, resp.Msg.Feed.Id)

	book, err := testApp.Repositories.Books.GetBookBySourceURL(
		context.Background(), arxiv.AbsURL(id),
	)
	require.NoError(t, err)
	assert.Equal(t, models.CategoryPaper, book.Category)
	assert.Equal(t, "A Feed-Ingested Paper", book.Title)

	statusResult, err := testApp.Services.Books.GetKEPUBStatus(
		context.Background(), userID, book.ID,
	)
	require.NoError(t, err)
	assert.True(t, statusResult.HasPDF)
}

// TestCreateFeed_ArxivFromGUID covers the GUID fallback in arxivIDFromItem:
// the item's <link> is a normal URL but its <guid> is an arXiv id.
func TestCreateFeed_ArxivFromGUID(t *testing.T) {
	id := uniqueArxivID()
	registerMockPaper(id, "Paper From GUID", "Alan Turing")

	base := uniqueBlogBase()
	feedURL := base + "/arxiv-guid.xml"
	mockWebFetch.SetBody(feedURL, "application/rss+xml", []byte(rssXML(
		"arXiv GUID Feed",
		rssItem{"Paper From GUID", base + "/landing", arxiv.AbsURL(id), ""},
	)))

	client := newBooksTestClient(t)
	resp, err := client.CreateFeed(
		context.Background(),
		feedReq(t, &readingv1.CreateFeedRequest{Url: feedURL, KoboSync: false}),
	)
	require.NoError(t, err)
	waitForFeedImport(t, client, resp.Msg.Feed.Id)

	book, err := testApp.Repositories.Books.GetBookBySourceURL(
		context.Background(), arxiv.AbsURL(id),
	)
	require.NoError(t, err)
	assert.Equal(t, models.CategoryPaper, book.Category)
}

// TestCreateFeed_UnknownArxivItem_IsSkipped covers the error path of
// IngestArxivByID: an arXiv item the API can't resolve is marked seen and not
// ingested, without failing the whole poll.
func TestCreateFeed_UnknownArxivItem_IsSkipped(t *testing.T) {
	id := uniqueArxivID() // deliberately NOT registered in the arXiv mock

	base := uniqueBlogBase()
	feedURL := base + "/arxiv-missing.xml"
	mockWebFetch.SetBody(feedURL, "application/rss+xml", []byte(rssXML(
		"arXiv Missing Feed",
		rssItem{"Missing Paper", arxiv.AbsURL(id), arxiv.AbsURL(id), ""},
	)))

	client := newBooksTestClient(t)
	resp, err := client.CreateFeed(
		context.Background(),
		feedReq(t, &readingv1.CreateFeedRequest{Url: feedURL, KoboSync: false}),
	)
	require.NoError(t, err)
	waitForFeedImport(t, client, resp.Msg.Feed.Id)

	_, err = testApp.Repositories.Books.GetBookBySourceURL(
		context.Background(), arxiv.AbsURL(id),
	)
	require.Error(t, err)
}

func TestFeedItemWithoutContent_TracksMetadataOnly(t *testing.T) {
	base := uniqueBlogBase()
	feedURL := base + "/feed-nocontent.xml"
	deadLink := base + "/dead-post"
	// Item has no embedded content and its link 404s in the mock.
	mockWebFetch.SetBody(feedURL, "application/rss+xml", []byte(rssXML(
		"NoContent Blog",
		rssItem{"Dead Post", deadLink, "n1", ""},
	)))

	client := newBooksTestClient(t)
	resp, err := client.CreateFeed(
		context.Background(),
		feedReq(t, &readingv1.CreateFeedRequest{Url: feedURL, KoboSync: false}),
	)
	require.NoError(t, err)
	waitForFeedImport(t, client, resp.Msg.Feed.Id)

	// The item is tracked metadata-only: catalog + user_book, no file.
	book, err := testApp.Repositories.Books.GetBookBySourceURL(
		context.Background(), deadLink,
	)
	require.NoError(t, err)
	assert.Equal(t, "Dead Post", book.Title)

	statusResult, err := testApp.Services.Books.GetKEPUBStatus(
		context.Background(), userID, book.ID,
	)
	require.NoError(t, err)
	assert.False(t, statusResult.HasEPUB)
	assert.False(t, statusResult.HasPDF)
}
