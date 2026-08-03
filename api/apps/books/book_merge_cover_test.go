package books_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"tools.xdoubleu.com/apps/books/internal/services"
)

// TestMergeBooks_CoverSource_CachesCoverEagerly verifies that resolving a
// merge's cover from another book eagerly fetches that cover into R2 (see
// BookService.applyCoverSource / cacheCoverFromURL) rather than just clearing
// the cache for a later lazy fetch.
func TestMergeBooks_CoverSource_CachesCoverEagerly(t *testing.T) {
	cleanupMergeUser(t)
	imgServer := httptest.NewServer(http.HandlerFunc(
		func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "image/jpeg")
			_, _ = w.Write([]byte("merge-cover-bytes"))
		},
	))
	defer imgServer.Close()

	winner, err := testApp.Services.Books.AddToLibrary(
		context.Background(), mergeTestUser,
		services.SourceProposal{ //nolint:exhaustruct //only required fields
			Source:  "manual",
			Title:   "MergeCoverWinner",
			Authors: []string{"Merge Author"},
			ISBN13:  "9780020202071",
		},
		"to-read", []string{},
	)
	require.NoError(t, err)

	source, err := testApp.Services.Books.AddToLibrary(
		context.Background(), mergeTestUser,
		services.SourceProposal{ //nolint:exhaustruct //only required fields
			Source:   "manual",
			Title:    "MergeCoverSource",
			Authors:  []string{"Merge Author"},
			ISBN13:   "9780020202072",
			CoverURL: imgServer.URL,
		},
		"to-read", []string{},
	)
	require.NoError(t, err)

	loser, err := testApp.Services.Books.AddToLibrary(
		context.Background(), mergeTestUser,
		services.SourceProposal{ //nolint:exhaustruct //only required fields
			Source:  "manual",
			Title:   "MergeCoverLoser",
			Authors: []string{"Merge Author"},
			ISBN13:  "9780020202075",
		},
		"to-read", []string{},
	)
	require.NoError(t, err)

	_, _, err = testApp.Services.Books.MergeBooks(
		context.Background(), mergeTestUser, winner.BookID, []uuid.UUID{loser.BookID},
		nil, &source.BookID, nil,
	)
	require.NoError(t, err)

	coverKey := "books/" + winner.BookID.String() + "/cover.jpg"
	data, cached := fakeStore.GetContent(coverKey)
	require.True(t, cached, "winner's cover should be cached in R2 after merge")
	assert.Equal(t, "merge-cover-bytes", string(data))
}

// TestMergeBooks_CoverSource_NoCover_ClearsCache verifies that resolving a
// merge's cover to a source book with no cover clears any previously cached
// cover for the winner, rather than leaving a stale image behind.
func TestMergeBooks_CoverSource_NoCover_ClearsCache(t *testing.T) {
	cleanupMergeUser(t)
	imgServer := httptest.NewServer(http.HandlerFunc(
		func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "image/jpeg")
			_, _ = w.Write([]byte("stale-cover-bytes"))
		},
	))
	defer imgServer.Close()

	winner, err := testApp.Services.Books.AddToLibrary(
		context.Background(), mergeTestUser,
		services.SourceProposal{ //nolint:exhaustruct //only required fields
			Source:   "manual",
			Title:    "MergeNoCoverWinner",
			Authors:  []string{"Merge Author"},
			ISBN13:   "9780020202073",
			CoverURL: imgServer.URL,
		},
		"to-read", []string{},
	)
	require.NoError(t, err)

	coverKey := "books/" + winner.BookID.String() + "/cover.jpg"
	_, cachedBeforeMerge := fakeStore.GetContent(coverKey)
	require.True(t, cachedBeforeMerge, "winner's cover must be cached before the merge")

	// No ISBN: enrichByISBN only runs when an ISBN is present, so this book
	// stays genuinely coverless instead of being auto-enriched with a cover
	// from the (always-populated) Hardcover mock.
	source, err := testApp.Services.Books.AddToLibrary(
		context.Background(), mergeTestUser,
		services.SourceProposal{ //nolint:exhaustruct //only required fields
			Source:  "manual",
			Title:   "MergeNoCoverSource",
			Authors: []string{"Merge Author"},
		},
		"to-read", []string{},
	)
	require.NoError(t, err)

	loser, err := testApp.Services.Books.AddToLibrary(
		context.Background(), mergeTestUser,
		services.SourceProposal{ //nolint:exhaustruct //only required fields
			Source:  "manual",
			Title:   "MergeNoCoverLoser",
			Authors: []string{"Merge Author"},
			ISBN13:  "9780020202076",
		},
		"to-read", []string{},
	)
	require.NoError(t, err)

	_, _, err = testApp.Services.Books.MergeBooks(
		context.Background(), mergeTestUser, winner.BookID, []uuid.UUID{loser.BookID},
		nil, &source.BookID, nil,
	)
	require.NoError(t, err)

	_, cachedAfterMerge := fakeStore.GetContent(coverKey)
	assert.False(
		t,
		cachedAfterMerge,
		"stale cover cache must be cleared, not left behind",
	)
}
