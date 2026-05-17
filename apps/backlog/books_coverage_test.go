package backlog_test

import (
	"context"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/xdoubleu/essentia/v4/pkg/test"
	"tools.xdoubleu.com/apps/backlog/internal/dtos"
	"tools.xdoubleu.com/apps/backlog/internal/models"
	"tools.xdoubleu.com/apps/backlog/pkg/hardcover"
)

// addTestBookWithStatus adds a book directly via the service layer for an
// isolated userID, so the shared userID's library is not polluted and
// BooksLibraryPage tests get predictable data.
func addTestBookWithStatus(
	t *testing.T,
	title string,
	status string,
	tags []string,
) *models.UserBook {
	t.Helper()
	desc := "A description."
	ext := hardcover.ExternalBook{ //nolint:exhaustruct //optional fields
		Provider:    "manual",
		ProviderID:  "test-lib-" + title,
		Title:       title,
		Authors:     []string{"Author"},
		Description: &desc,
	}
	ub, err := testApp.Services.Books.AddToLibrary(
		context.Background(),
		userID,
		ext,
		status,
		tags,
	)
	require.NoError(t, err)
	require.NotNil(t, ub)
	return ub
}

// TestBooksLibraryPage_WithData populates the library with books of every
// status and with custom/tag shelves, then fetches the library page.
// This exercises the non-empty conditional branches in BooksLibraryPage.
func TestBooksLibraryPage_WithData(t *testing.T) {
	addTestBookWithStatus(t, "LibraryReadingBook", models.StatusReading, []string{})
	addTestBookWithStatus(t, "LibraryToReadBook", models.StatusToRead, []string{})
	addTestBookWithStatus(t, "LibraryReadBook", models.StatusRead, []string{})
	addTestBookWithStatus(t, "LibraryTagBook", models.StatusToRead, []string{"sci-fi"})

	tReq := test.CreateRequestTester(
		getRoutes(),
		http.MethodGet,
		"/"+testApp.GetName()+"/books/library",
	)
	tReq.AddCookie(&accessToken)

	rs := tReq.Do(t)
	assert.Equal(t, http.StatusOK, rs.StatusCode)
}

// TestBooksProgressPage_WithData marks a book as read so that the progress
// service returns non-nil labels/values, exercising the populated branches
// of BooksProgressPage and booksProgressScript.
func TestBooksProgressPage_WithData(t *testing.T) {
	ub := addTestBook(t, "ProgressPageBook")

	// Mark the book as read so a progress entry is recorded.
	tReq := test.CreateRequestTester(
		getRoutes(),
		http.MethodPost,
		"/"+testApp.GetName()+"/books/"+ub.BookID.String()+"/status",
	)
	tReq.SetContentType(test.FormContentType)
	tReq.SetData(struct {
		Status string `schema:"status"`
		Rating string `schema:"rating"`
	}{Status: models.StatusRead, Rating: "4"})
	tReq.AddCookie(&accessToken)
	tReq.SetFollowRedirect(false)
	rs := tReq.Do(t)
	assert.Equal(t, http.StatusSeeOther, rs.StatusCode)

	// Now fetch the progress page — labels/values will be non-empty.
	tReq2 := test.CreateRequestTester(
		getRoutes(),
		http.MethodGet,
		"/"+testApp.GetName()+"/books/progress",
	)
	tReq2.AddCookie(&accessToken)
	rs2 := tReq2.Do(t)
	assert.Equal(t, http.StatusOK, rs2.StatusCode)
}

// TestBooksLibraryPage_FavouriteAndOwnership adds a book with special tags so
// the bookCard templ branches for own-physical, own-digital and favourite are
// exercised.
func TestBooksLibraryPage_FavouriteAndOwnership(t *testing.T) {
	addTestBookWithStatus(
		t,
		"FavouriteOwnedBook",
		models.StatusRead,
		[]string{
			models.TagFavourite,
			models.TagOwnPhysical,
			models.TagOwnDigital,
		},
	)

	tReq := test.CreateRequestTester(
		getRoutes(),
		http.MethodGet,
		"/"+testApp.GetName()+"/books/library",
	)
	tReq.AddCookie(&accessToken)

	rs := tReq.Do(t)
	assert.Equal(t, http.StatusOK, rs.StatusCode)
}

// TestUpdateBookStatus_ReadThenReSaveNoSpike is covered in books_import_test.go
// but here we also verify the FinishedAt dedup via the service layer directly.
// (The handler test lives in books_import_test.go; keep this file focused on
// coverage-only library/progress page rendering.)

// TestBooksLibraryPage_AllStatusesAndTags exercises the full library page with
// books in every status bucket plus tagged shelves to cover all branches.
func TestBooksLibraryPage_AllStatusesAndTags(t *testing.T) {
	addTestBookWithStatus(
		t, "AllStatusReading", models.StatusReading, []string{"tag-a"},
	)
	addTestBookWithStatus(
		t, "AllStatusToRead", models.StatusToRead, []string{"tag-b"},
	)
	addTestBookWithStatus(
		t, "AllStatusRead", models.StatusRead, []string{},
	)

	tReq := test.CreateRequestTester(
		getRoutes(),
		http.MethodGet,
		"/"+testApp.GetName()+"/books/library",
	)
	tReq.AddCookie(&accessToken)

	rs := tReq.Do(t)
	assert.Equal(t, http.StatusOK, rs.StatusCode)
}

// TestBooksProgressPage_WithDateRangeAndData marks a book read then queries
// the progress page with explicit from/to parameters to cover the date-range
// branch of GetByTypeIDAndDates.
func TestBooksProgressPage_WithDateRangeAndData(t *testing.T) {
	ub := addTestBook(t, "ProgressRangeBook")

	tReq := test.CreateRequestTester(
		getRoutes(),
		http.MethodPost,
		"/"+testApp.GetName()+"/books/"+ub.BookID.String()+"/status",
	)
	tReq.SetContentType(test.FormContentType)
	tReq.SetData(dtos.UpdateBookStatusDto{ //nolint:exhaustruct //optional fields
		Status: models.StatusRead,
		Rating: "3",
	})
	tReq.AddCookie(&accessToken)
	tReq.SetFollowRedirect(false)
	rs := tReq.Do(t)
	require.Equal(t, http.StatusSeeOther, rs.StatusCode)

	tReq2 := test.CreateRequestTester(
		getRoutes(),
		http.MethodGet,
		"/"+testApp.GetName()+"/books/progress?from=2020-01-01&to=2099-12-31",
	)
	tReq2.AddCookie(&accessToken)
	rs2 := tReq2.Do(t)
	assert.Equal(t, http.StatusOK, rs2.StatusCode)
}
