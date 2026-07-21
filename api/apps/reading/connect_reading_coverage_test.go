package reading_test

import (
	"context"
	"testing"

	"connectrpc.com/connect"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/xdoubleu/essentia/v4/pkg/logging"

	"tools.xdoubleu.com/apps/reading/internal/models"
	"tools.xdoubleu.com/apps/reading/pkg/arxiv"
	readingv1 "tools.xdoubleu.com/gen/reading/v1"
)

// --- SetBookCategory (connect_catalog.go) ---

func TestSetBookCategory_NonAdmin_PermissionDenied(t *testing.T) {
	client := newBooksTestClient(t)
	req := connect.NewRequest(&readingv1.SetBookCategoryRequest{
		BookId:   "00000000-0000-0000-0000-000000000001",
		Category: models.CategoryPaper,
	})
	req.Header().Set("Cookie", accessToken.String())

	_, err := client.SetBookCategory(context.Background(), req)
	require.Error(t, err)
	assert.Equal(t, connect.CodePermissionDenied, connect.CodeOf(err))
}

func TestSetBookCategory_InvalidCategory_InvalidArgument(t *testing.T) {
	ub := addTestBookNoISBN(t, "SetCategoryInvalidBook")
	client := newAdminBooksTestClient(t)
	req := connect.NewRequest(&readingv1.SetBookCategoryRequest{
		BookId:   ub.BookID.String(),
		Category: "not-a-category",
	})
	req.Header().Set("Cookie", accessToken.String())

	_, err := client.SetBookCategory(context.Background(), req)
	require.Error(t, err)
	assert.Equal(t, connect.CodeInvalidArgument, connect.CodeOf(err))
}

func TestSetBookCategory_Success(t *testing.T) {
	ub := addTestBookNoISBN(t, "SetCategorySuccessBook")
	client := newAdminBooksTestClient(t)
	req := connect.NewRequest(&readingv1.SetBookCategoryRequest{
		BookId:   ub.BookID.String(),
		Category: models.CategoryPaper,
	})
	req.Header().Set("Cookie", accessToken.String())

	_, err := client.SetBookCategory(context.Background(), req)
	require.NoError(t, err)

	book, err := testApp.Repositories.Books.GetBookByID(
		context.Background(), ub.BookID,
	)
	require.NoError(t, err)
	assert.Equal(t, models.CategoryPaper, book.Category)
}

// --- Feed connect error paths (connect_feeds.go) ---

func TestRefreshFeed_InvalidID(t *testing.T) {
	client := newBooksTestClient(t)
	_, err := client.RefreshFeed(
		context.Background(),
		feedReq(t, &readingv1.RefreshFeedRequest{FeedId: "not-a-uuid"}),
	)
	require.Error(t, err)
	assert.Equal(t, connect.CodeInvalidArgument, connect.CodeOf(err))
}

func TestRefreshFeed_UnknownFeed_NotFound(t *testing.T) {
	client := newBooksTestClient(t)
	_, err := client.RefreshFeed(
		context.Background(),
		feedReq(t, &readingv1.RefreshFeedRequest{FeedId: uuid.NewString()}),
	)
	require.Error(t, err)
	assert.Equal(t, connect.CodeNotFound, connect.CodeOf(err))
}

// --- Feed service branches (feeds.go) ---

func TestRefreshFeed_NotModified_IngestsNothing(t *testing.T) {
	base := uniqueBlogBase()
	feedURL := base + "/feed-nm.xml"
	mockWebFetch.SetBody(feedURL, "application/rss+xml", []byte(rssXML(
		"NM Blog",
		rssItem{"Post", base + "/nm1", "nm1", itemContent},
	)))

	client := newBooksTestClient(t)
	created, err := client.CreateFeed(
		context.Background(),
		feedReq(t, &readingv1.CreateFeedRequest{Url: feedURL, KoboSync: false}),
	)
	require.NoError(t, err)

	// A conditional GET now short-circuits with 304.
	mockWebFetch.SetNotModified(feedURL)
	refreshed, err := client.RefreshFeed(
		context.Background(),
		feedReq(t, &readingv1.RefreshFeedRequest{FeedId: created.Msg.Feed.Id}),
	)
	require.NoError(t, err)
	assert.Equal(t, int32(0), refreshed.Msg.Ingested)
}

func TestCreateFeed_CapsItemsPerPoll(t *testing.T) {
	base := uniqueBlogBase()
	feedURL := base + "/feed-cap.xml"

	// Build a feed with more items than the per-poll cap.
	items := make([]rssItem, 0, 25)
	for i := 0; i < 25; i++ {
		id := uuid.NewString()
		items = append(
			items, rssItem{"Post " + id, base + "/" + id, id, itemContent},
		)
	}
	mockWebFetch.SetBody(
		feedURL, "application/rss+xml", []byte(rssXML("Cap Blog", items...)),
	)

	client := newBooksTestClient(t)
	created, err := client.CreateFeed(
		context.Background(),
		feedReq(t, &readingv1.CreateFeedRequest{Url: feedURL, KoboSync: false}),
	)
	require.NoError(t, err)
	waitForFeedImport(t, client, created.Msg.Feed.Id)

	ingestedCount := 0
	for _, it := range items {
		if _, bookErr := testApp.Repositories.Books.GetBookBySourceURL(
			context.Background(), it.link,
		); bookErr == nil {
			ingestedCount++
		}
	}
	assert.Equal(t, 20, ingestedCount)
}

func TestCreateFeed_FetchesLinkedPageWhenNoContent(t *testing.T) {
	base := uniqueBlogBase()
	feedURL := base + "/feed-linked.xml"
	itemURL := base + "/linked-post"
	// Item has no embedded <content:encoded>; its link resolves to a page.
	mockWebFetch.SetBody(feedURL, "application/rss+xml", []byte(rssXML(
		"Linked Blog",
		rssItem{"Linked Post", itemURL, "lp1", ""},
	)))
	mockWebFetch.SetHTML(itemURL, articlePageHTML("Linked Post Body"))

	client := newBooksTestClient(t)
	created, err := client.CreateFeed(
		context.Background(),
		feedReq(t, &readingv1.CreateFeedRequest{Url: feedURL, KoboSync: false}),
	)
	require.NoError(t, err)
	waitForFeedImport(t, client, created.Msg.Feed.Id)

	book, err := testApp.Repositories.Books.GetBookBySourceURL(
		context.Background(), itemURL,
	)
	require.NoError(t, err)
	// Content came from the linked page, so a real EPUB was built and stored.
	status, err := testApp.Services.Books.GetKEPUBStatus(
		context.Background(), userID, book.ID,
	)
	require.NoError(t, err)
	assert.True(t, status.HasEPUB)
}

func TestPollAll_PollsEveryFeed(t *testing.T) {
	base := uniqueBlogBase()
	feedURL := base + "/feed-pollall.xml"
	mockWebFetch.SetBody(feedURL, "application/rss+xml", []byte(rssXML(
		"PollAll Blog",
		rssItem{"Seed", base + "/seed", "seed", itemContent},
	)))

	client := newBooksTestClient(t)
	created, err := client.CreateFeed(
		context.Background(),
		feedReq(t, &readingv1.CreateFeedRequest{Url: feedURL, KoboSync: false}),
	)
	require.NoError(t, err)
	waitForFeedImport(t, client, created.Msg.Feed.Id)

	// A new post appears; PollAll should pick it up across every user's feeds.
	mockWebFetch.SetBody(feedURL, "application/rss+xml", []byte(rssXML(
		"PollAll Blog",
		rssItem{"Seed", base + "/seed", "seed", itemContent},
		rssItem{"Fresh", base + "/fresh", "fresh", itemContent},
	)))

	var lastProcessed, lastTotal int
	err = testApp.Services.Feeds.PollAll(
		context.Background(),
		logging.NewNopLogger(),
		func(processed, total int) { lastProcessed, lastTotal = processed, total },
	)
	require.NoError(t, err)
	assert.GreaterOrEqual(t, lastTotal, 1)
	assert.Equal(t, lastTotal, lastProcessed)

	// The fresh post is now in the catalog.
	_, err = testApp.Repositories.Books.GetBookBySourceURL(
		context.Background(), base+"/fresh",
	)
	require.NoError(t, err)
}

// --- Ingest branches (ingest.go) ---

func TestAddBookByURL_ArxivPDFNotAPDF(t *testing.T) {
	id := uniqueArxivID()
	registerMockPaper(id, "Bad PDF Paper", "Ada Lovelace")
	// Override: the "PDF" download actually returns an HTML error page.
	mockWebFetch.SetBody(
		arxiv.PDFURL(id), "application/pdf", []byte("<html>nope</html>"),
	)

	_, err := addByURL(t, arxiv.AbsURL(id), "")
	require.Error(t, err)
	assert.Equal(t, connect.CodeInvalidArgument, connect.CodeOf(err))
}

func TestAddBookByURL_RebuildsMissingPaperPDF(t *testing.T) {
	id := uniqueArxivID()
	registerMockPaper(id, "Rebuild Paper", "Ada Lovelace")

	first, err := addByURL(t, arxiv.AbsURL(id), "")
	require.NoError(t, err)
	bookID := mustUUID(t, first.UserBook.BookId)

	// Drop the stored PDF so the re-add must re-download it via the paper path.
	_, err = testApp.Repositories.BookFiles.DeleteByUserBook(
		context.Background(), userID, bookID,
	)
	require.NoError(t, err)

	again, err := addByURL(t, arxiv.AbsURL(id), "")
	require.NoError(t, err)
	assert.True(t, again.AlreadyInLibrary)

	status, err := testApp.Services.Books.GetKEPUBStatus(
		context.Background(), userID, bookID,
	)
	require.NoError(t, err)
	assert.True(t, status.HasPDF, "missing paper PDF should be rebuilt on re-add")
}

// TestCreateFeed_ItemWithAuthor covers the byline path: an RSS item with an
// <author> element becomes the article's author.
func TestCreateFeed_ItemWithAuthor(t *testing.T) {
	base := uniqueBlogBase()
	feedURL := base + "/feed-author.xml"
	itemURL := base + "/authored-post"
	rss := `<?xml version="1.0"?><rss version="2.0" ` +
		`xmlns:content="http://purl.org/rss/1.0/modules/content/">` +
		`<channel><title>Author Blog</title><item>` +
		`<title>Authored Post</title><link>` + itemURL + `</link>` +
		`<guid>ap1</guid><author>jane@example.com (Jane Writer)</author>` +
		`<content:encoded><![CDATA[` + itemContent + `]]></content:encoded>` +
		`</item></channel></rss>`
	mockWebFetch.SetBody(feedURL, "application/rss+xml", []byte(rss))

	client := newBooksTestClient(t)
	created, err := client.CreateFeed(
		context.Background(),
		feedReq(t, &readingv1.CreateFeedRequest{Url: feedURL, KoboSync: false}),
	)
	require.NoError(t, err)
	waitForFeedImport(t, client, created.Msg.Feed.Id)

	book, err := testApp.Repositories.Books.GetBookBySourceURL(
		context.Background(), itemURL,
	)
	require.NoError(t, err)
	require.NotEmpty(t, book.Authors)
	assert.Contains(t, book.Authors[0], "Jane Writer")
}

func TestAddBookByURL_RebuildsMissingFile(t *testing.T) {
	url := "https://blog.example.com/posts/" + uuid.NewString() + "/rebuild-me"
	mockWebFetch.SetHTML(url, articlePageHTML("Rebuild Me"))

	first, err := addByURL(t, url, "")
	require.NoError(t, err)
	bookID := mustUUID(t, first.UserBook.BookId)

	// Drop the stored EPUB so the re-add must rebuild it.
	_, err = testApp.Repositories.BookFiles.DeleteByUserBook(
		context.Background(), userID, bookID,
	)
	require.NoError(t, err)

	again, err := addByURL(t, url, "")
	require.NoError(t, err)
	assert.True(t, again.AlreadyInLibrary)

	status, err := testApp.Services.Books.GetKEPUBStatus(
		context.Background(), userID, bookID,
	)
	require.NoError(t, err)
	assert.True(t, status.HasEPUB, "missing file should be rebuilt on re-add")
}
