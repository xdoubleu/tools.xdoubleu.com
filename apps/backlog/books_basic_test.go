package backlog_test

import (
	"context"
	"fmt"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/xdoubleu/essentia/v4/pkg/test"
	"tools.xdoubleu.com/apps/backlog/internal/dtos"
	"tools.xdoubleu.com/apps/backlog/internal/models"
	"tools.xdoubleu.com/apps/backlog/pkg/hardcover"
)

func addTestBook(t *testing.T, title string) *models.UserBook {
	t.Helper()
	isbn := "9780140449112"
	cover := "https://example.com/cover.jpg"
	desc := "Test description."
	ext := hardcover.ExternalBook{ //nolint:exhaustruct //optional ISBN10 not needed
		Provider:    "manual",
		ProviderID:  fmt.Sprintf("test-%s", title),
		Title:       title,
		Authors:     []string{"Test Author"},
		ISBN13:      &isbn,
		CoverURL:    &cover,
		Description: &desc,
	}
	ub, err := testApp.Services.Books.AddToLibrary(
		context.Background(),
		userID,
		ext,
		models.StatusToRead,
		[]string{},
	)
	require.NoError(t, err)
	require.NotNil(t, ub)
	return ub
}

func TestBooksPage(t *testing.T) {
	tReq := test.CreateRequestTester(
		getRoutes(),
		http.MethodGet,
		"/"+testApp.GetName()+"/books",
	)
	tReq.AddCookie(&accessToken)

	rs := tReq.Do(t)
	assert.Equal(t, http.StatusOK, rs.StatusCode)
}

func TestBooksLibrary(t *testing.T) {
	tReq := test.CreateRequestTester(
		getRoutes(),
		http.MethodGet,
		"/"+testApp.GetName()+"/books/library",
	)
	tReq.AddCookie(&accessToken)

	rs := tReq.Do(t)
	assert.Equal(t, http.StatusOK, rs.StatusCode)
}

func TestBooksProgress(t *testing.T) {
	tReq := test.CreateRequestTester(
		getRoutes(),
		http.MethodGet,
		"/"+testApp.GetName()+"/books/progress",
	)
	tReq.AddCookie(&accessToken)

	rs := tReq.Do(t)
	assert.Equal(t, http.StatusOK, rs.StatusCode)
}

func TestBooksProgressWithDateRange(t *testing.T) {
	tReq := test.CreateRequestTester(
		getRoutes(),
		http.MethodGet,
		"/"+testApp.GetName()+"/books/progress?from=2024-01-01&to=2024-12-31",
	)
	tReq.AddCookie(&accessToken)

	rs := tReq.Do(t)
	assert.Equal(t, http.StatusOK, rs.StatusCode)
}

func TestBooksSearchEmpty(t *testing.T) {
	tReq := test.CreateRequestTester(
		getRoutes(),
		http.MethodGet,
		"/"+testApp.GetName()+"/books/search",
	)
	tReq.AddCookie(&accessToken)

	rs := tReq.Do(t)
	assert.Equal(t, http.StatusOK, rs.StatusCode)
}

func TestBooksSearchHardcoverFallback(t *testing.T) {
	// Query that won't match anything in the library → falls back to Hardcover mock.
	tReq := test.CreateRequestTester(
		getRoutes(),
		http.MethodGet,
		"/"+testApp.GetName()+"/books/search?q=zzz-no-match-xyz",
	)
	tReq.AddCookie(&accessToken)

	rs := tReq.Do(t)
	assert.Equal(t, http.StatusOK, rs.StatusCode)
}

func TestAddBook(t *testing.T) {
	tReq := test.CreateRequestTester(
		getRoutes(),
		http.MethodPost,
		"/"+testApp.GetName()+"/books",
	)
	tReq.SetContentType(test.FormContentType)
	tReq.SetData(dtos.AddBookDto{ //nolint:exhaustruct //optional fields
		ProviderID:  "test-provider-id-1",
		Provider:    "manual",
		Title:       "Test Book For Add",
		Author:      "Test Author",
		Status:      models.StatusToRead,
		OwnPhysical: false,
		OwnDigital:  false,
	})
	tReq.AddCookie(&accessToken)
	tReq.SetFollowRedirect(false)

	rs := tReq.Do(t)
	assert.Equal(t, http.StatusSeeOther, rs.StatusCode)
	assert.Equal(t, "/backlog/books", rs.Header.Get("Location"))
}

func TestAddBook_DefaultStatus(t *testing.T) {
	// Omitting Status → handler defaults to to-read.
	tReq := test.CreateRequestTester(
		getRoutes(),
		http.MethodPost,
		"/"+testApp.GetName()+"/books",
	)
	tReq.SetContentType(test.FormContentType)
	tReq.SetData(dtos.AddBookDto{ //nolint:exhaustruct //optional fields
		ProviderID: "test-provider-id-default-status",
		Provider:   "manual",
		Title:      "Test Book Default Status",
		Author:     "Author",
	})
	tReq.AddCookie(&accessToken)
	tReq.SetFollowRedirect(false)

	rs := tReq.Do(t)
	assert.Equal(t, http.StatusSeeOther, rs.StatusCode)
}

func TestAddBook_WithOwnershipTags(t *testing.T) {
	tReq := test.CreateRequestTester(
		getRoutes(),
		http.MethodPost,
		"/"+testApp.GetName()+"/books",
	)
	tReq.SetContentType(test.FormContentType)
	tReq.SetData(dtos.AddBookDto{ //nolint:exhaustruct //optional fields
		ProviderID:  "test-provider-id-ownership",
		Provider:    "manual",
		Title:       "Owned Book",
		Author:      "Author",
		OwnPhysical: true,
		OwnDigital:  true,
	})
	tReq.AddCookie(&accessToken)
	tReq.SetFollowRedirect(false)

	rs := tReq.Do(t)
	assert.Equal(t, http.StatusSeeOther, rs.StatusCode)
}

func TestBooksSearchLibraryHit(t *testing.T) {
	addTestBook(t, "LibrarySearchTarget")

	tReq := test.CreateRequestTester(
		getRoutes(),
		http.MethodGet,
		"/"+testApp.GetName()+"/books/search?q=LibrarySearchTarget",
	)
	tReq.AddCookie(&accessToken)

	rs := tReq.Do(t)
	assert.Equal(t, http.StatusOK, rs.StatusCode)
}

func TestUpdateBookStatus(t *testing.T) {
	ub := addTestBook(t, "StatusUpdateBook")

	tReq := test.CreateRequestTester(
		getRoutes(),
		http.MethodPost,
		"/"+testApp.GetName()+"/books/"+ub.BookID.String()+"/status",
	)
	tReq.SetContentType(test.FormContentType)
	tReq.SetData(dtos.UpdateBookStatusDto{ //nolint:exhaustruct //optional fields
		Status: models.StatusReading,
	})
	tReq.AddCookie(&accessToken)
	tReq.SetFollowRedirect(false)

	rs := tReq.Do(t)
	assert.Equal(t, http.StatusSeeOther, rs.StatusCode)
}

func TestUpdateBookStatus_MarkRead(t *testing.T) {
	ub := addTestBook(t, "MarkReadBook")

	tReq := test.CreateRequestTester(
		getRoutes(),
		http.MethodPost,
		"/"+testApp.GetName()+"/books/"+ub.BookID.String()+"/status",
	)
	tReq.SetContentType(test.FormContentType)
	tReq.SetData(
		dtos.UpdateBookStatusDto{
			Status:    models.StatusRead,
			Rating:    "5",
			Notes:     "Excellent.",
			Favourite: true,
		},
	)
	tReq.AddCookie(&accessToken)
	tReq.SetFollowRedirect(false)

	rs := tReq.Do(t)
	assert.Equal(t, http.StatusSeeOther, rs.StatusCode)
}

func TestUpdateBookStatus_InvalidRating(t *testing.T) {
	ub := addTestBook(t, "InvalidRatingBook")

	tReq := test.CreateRequestTester(
		getRoutes(),
		http.MethodPost,
		"/"+testApp.GetName()+"/books/"+ub.BookID.String()+"/status",
	)
	tReq.SetContentType(test.FormContentType)
	tReq.SetData(dtos.UpdateBookStatusDto{ //nolint:exhaustruct //optional fields
		Status: models.StatusToRead,
		Rating: "0",
	})
	tReq.AddCookie(&accessToken)
	tReq.SetFollowRedirect(false)

	rs := tReq.Do(t)
	assert.Equal(t, http.StatusSeeOther, rs.StatusCode)
}

func TestUpdateBookStatus_InvalidID(t *testing.T) {
	tReq := test.CreateRequestTester(
		getRoutes(),
		http.MethodPost,
		"/"+testApp.GetName()+"/books/not-a-uuid/status",
	)
	tReq.SetContentType(test.FormContentType)
	tReq.SetData(dtos.UpdateBookStatusDto{ //nolint:exhaustruct //optional fields
		Status: models.StatusToRead,
	})
	tReq.AddCookie(&accessToken)

	rs := tReq.Do(t)
	assert.Equal(t, http.StatusNotFound, rs.StatusCode)
}
