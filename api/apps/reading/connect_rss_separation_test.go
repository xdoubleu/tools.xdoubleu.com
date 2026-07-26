package reading_test

import (
	"context"
	"testing"
	"time"

	"connectrpc.com/connect"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"tools.xdoubleu.com/apps/reading/internal/models"
	readingv1 "tools.xdoubleu.com/gen/reading/v1"
)

// addTestItem seeds a catalog item of the given category and status directly
// (bypassing the ingest pipeline) so tests can exercise category-aware
// bucketing and counting. Read items get a finished_at date.
func addTestItem(t *testing.T, category, status, title string) *models.Book {
	t.Helper()
	src := "https://example.com/" + category + "/" + uuid.NewString()
	book, err := testApp.Repositories.Books.UpsertBookBySourceURL(
		context.Background(),
		//nolint:exhaustruct // catalog metadata only; rest is DB-owned
		models.Book{
			Title:     title,
			Authors:   []string{"Feed Author"},
			Category:  category,
			SourceURL: &src,
		},
	)
	require.NoError(t, err)

	var finishedAt []time.Time
	if status == models.StatusRead {
		finishedAt = []time.Time{time.Now()}
	}
	require.NoError(t, testApp.Repositories.Books.UpsertUserBook(
		context.Background(),
		//nolint:exhaustruct // optional fields
		models.UserBook{
			UserID:         userID,
			BookID:         book.ID,
			Status:         status,
			Tags:           []string{},
			ShelfPositions: map[string]int{},
			FinishedAt:     finishedAt,
		},
	))
	return book
}

func containsBook(books []*readingv1.UserBook, bookID string) bool {
	for _, ub := range books {
		if ub.BookId == bookID {
			return true
		}
	}
	return false
}

func findUserBook(books []*readingv1.UserBook, bookID string) *readingv1.UserBook {
	for _, ub := range books {
		if ub.BookId == bookID {
			return ub
		}
	}
	return nil
}

// TestGetLibrary_SeparatesRSS proves #408: RSS items are returned in the
// dedicated `rss` field and kept out of the reading-state shelves, while
// deliberately-added papers/articles stay in the shelves with books.
func TestGetLibrary_SeparatesRSS(t *testing.T) {
	rssBook := addTestItem(t, models.CategoryRSS, models.StatusToRead, "RSS Item")
	paperBook := addTestItem(t, models.CategoryPaper, models.StatusToRead, "Paper Item")

	client := newBooksTestClient(t)
	req := connect.NewRequest(&readingv1.GetLibraryRequest{})
	req.Header().Set("Cookie", accessToken.String())
	resp, err := client.GetLibrary(context.Background(), req)
	require.NoError(t, err)
	lib := resp.Msg.Library

	// RSS item: in rss, not in wishlist.
	assert.True(t, containsBook(lib.Rss, rssBook.ID.String()))
	assert.False(t, containsBook(lib.Wishlist, rssBook.ID.String()))

	// Paper item (deliberate reading): in wishlist, not in rss.
	assert.True(t, containsBook(lib.Wishlist, paperBook.ID.String()))
	assert.False(t, containsBook(lib.Rss, paperBook.ID.String()))
}

// TestGetFinishedDates_ExcludesRSS proves #409: a read RSS item is not counted
// toward the read-progress graph, while a read book is.
func TestGetFinishedDates_ExcludesRSS(t *testing.T) {
	before, err := testApp.Repositories.Books.GetFinishedDates(
		context.Background(), userID,
	)
	require.NoError(t, err)

	// A read book counts.
	addTestItem(t, models.CategoryBook, models.StatusRead, "Read Book Item")
	afterBook, err := testApp.Repositories.Books.GetFinishedDates(
		context.Background(), userID,
	)
	require.NoError(t, err)
	assert.Len(t, afterBook, len(before)+1)

	// A read RSS item does not.
	addTestItem(t, models.CategoryRSS, models.StatusRead, "Read RSS Item")
	afterRSS, err := testApp.Repositories.Books.GetFinishedDates(
		context.Background(), userID,
	)
	require.NoError(t, err)
	assert.Len(t, afterRSS, len(afterBook))
}

// TestGetLibrary_HasContentReflectsExtractedContent proves #592: the feed
// reader needs to tell apart RSS items with in-app content from ones where
// extraction failed, without a per-item content fetch.
func TestGetLibrary_HasContentReflectsExtractedContent(t *testing.T) {
	withContent := addTestItem(
		t,
		models.CategoryRSS,
		models.StatusToRead,
		"Has Content Item",
	)
	require.NoError(t, testApp.Repositories.Books.SetBookContentHTML(
		context.Background(), withContent.ID, "<p>body</p>",
	))
	withoutContent := addTestItem(
		t,
		models.CategoryRSS,
		models.StatusToRead,
		"No Content Item",
	)

	client := newBooksTestClient(t)
	req := connect.NewRequest(&readingv1.GetLibraryRequest{})
	req.Header().Set("Cookie", accessToken.String())
	resp, err := client.GetLibrary(context.Background(), req)
	require.NoError(t, err)
	lib := resp.Msg.Library

	withContentUB := findUserBook(lib.Rss, withContent.ID.String())
	require.NotNil(t, withContentUB)
	assert.True(t, withContentUB.Book.HasContent)

	withoutContentUB := findUserBook(lib.Rss, withoutContent.ID.String())
	require.NotNil(t, withoutContentUB)
	assert.False(t, withoutContentUB.Book.HasContent)
}
