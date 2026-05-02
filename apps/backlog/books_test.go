package backlog_test

import (
	"bytes"
	"context"
	"fmt"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/xdoubleu/essentia/v4/pkg/test"
	"tools.xdoubleu.com/apps/backlog/internal/dtos"
	"tools.xdoubleu.com/apps/backlog/internal/models"
	"tools.xdoubleu.com/apps/backlog/pkg/hardcover"
)

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

func TestToggleTag(t *testing.T) {
	ub := addTestBook(t, "ToggleTagBook")

	tReq := test.CreateRequestTester(
		getRoutes(),
		http.MethodPost,
		"/"+testApp.GetName()+"/books/"+ub.BookID.String()+"/tags",
	)
	tReq.SetContentType(test.FormContentType)
	tReq.SetData(dtos.ToggleTagDto{Tag: "classics"})
	tReq.AddCookie(&accessToken)
	tReq.SetFollowRedirect(false)

	rs := tReq.Do(t)
	assert.Equal(t, http.StatusSeeOther, rs.StatusCode)
}

func TestToggleTag_InvalidID(t *testing.T) {
	tReq := test.CreateRequestTester(
		getRoutes(),
		http.MethodPost,
		"/"+testApp.GetName()+"/books/not-a-uuid/tags",
	)
	tReq.SetContentType(test.FormContentType)
	tReq.SetData(dtos.ToggleTagDto{Tag: "classics"})
	tReq.AddCookie(&accessToken)

	rs := tReq.Do(t)
	assert.Equal(t, http.StatusNotFound, rs.StatusCode)
}

func TestToggleTag_EmptyTag(t *testing.T) {
	ub := addTestBook(t, "EmptyTagBook")

	tReq := test.CreateRequestTester(
		getRoutes(),
		http.MethodPost,
		"/"+testApp.GetName()+"/books/"+ub.BookID.String()+"/tags",
	)
	tReq.SetContentType(test.FormContentType)
	tReq.SetData(dtos.ToggleTagDto{Tag: ""})
	tReq.AddCookie(&accessToken)

	rs := tReq.Do(t)
	assert.Equal(t, http.StatusBadRequest, rs.StatusCode)
}

// goodreadsCSVForImport is a minimal Goodreads CSV for import testing.
//
//nolint:lll // CSV rows are inherently long
const goodreadsCSVForImport = `Book Id,Title,Author,ISBN,ISBN13,My Rating,Exclusive Shelf,Bookshelves with positions,Date Read
99001,Import Test Book,Import Author,"=""0140449116""","=""9780140449112""",4,read,"read (#1)",2023/05/20
`

func TestBooksProgress_CarryForwardAfterWindow(t *testing.T) {
	// Use an isolated userID so accumulated progress entries from other tests
	// (which write to the shared userID on varying calendar dates) don't become
	// the most-recent baseline and override the carry-forward value being tested.
	const isolatedUserID = "test-carry-forward-isolation"

	// Simulate a progress record from 2 years ago (outside the default 1-year window).
	twoYearsAgo := time.Now().AddDate(-2, 0, 0).Format(models.ProgressDateFormat)
	err := testApp.Services.Progress.Save(
		context.Background(),
		models.BooksTypeID,
		isolatedUserID,
		[]string{twoYearsAgo},
		[]string{"7"},
	)
	require.NoError(t, err)

	// Query a window that starts yesterday and ends today — no records fall
	// inside it, but "7" from 2 years ago should carry forward.
	yesterday := time.Now().AddDate(0, 0, -1)
	today := time.Now()
	labels, values, err := testApp.Services.Progress.GetByTypeIDAndDates(
		context.Background(),
		models.BooksTypeID,
		isolatedUserID,
		yesterday,
		today,
	)
	require.NoError(t, err)
	assert.NotEmpty(
		t,
		labels,
		"window with no records must still return labels via carry-forward",
	)
	assert.NotEmpty(
		t,
		values,
		"window with no records must still return values via carry-forward",
	)
	if len(values) > 0 {
		v, parseErr := strconv.Atoi(values[0])
		require.NoError(t, parseErr)
		assert.GreaterOrEqual(
			t,
			v,
			7,
			"first value must carry forward the cumulative count from before the window",
		)
	}
}

func TestUpdateBookStatus_ReadThenReSaveNoSpike(t *testing.T) {
	ub := addTestBook(t, "ReSaveNoSpikeBook")

	markRead := func(notes string) {
		tReq := test.CreateRequestTester(
			getRoutes(),
			http.MethodPost,
			"/"+testApp.GetName()+"/books/"+ub.BookID.String()+"/status",
		)
		tReq.SetContentType(test.FormContentType)
		tReq.SetData(dtos.UpdateBookStatusDto{ //nolint:exhaustruct //optional fields
			Status: models.StatusRead,
			Notes:  notes,
		})
		tReq.AddCookie(&accessToken)
		tReq.SetFollowRedirect(false)
		rs := tReq.Do(t)
		assert.Equal(t, http.StatusSeeOther, rs.StatusCode)
	}

	markRead("First save.")
	markRead("Second save — just updating notes, should not add a timestamp.")

	got, err := testApp.Services.Books.GetUserBook(
		context.Background(),
		userID,
		ub.BookID,
	)
	require.NoError(t, err)
	assert.Len(
		t,
		got.FinishedAt,
		1,
		"re-saving a read book must not append extra FinishedAt timestamps",
	)
}

func TestImportBooks(t *testing.T) {
	ts := httptest.NewServer(getRoutes())
	defer ts.Close()

	var body bytes.Buffer
	mw := multipart.NewWriter(&body)

	fw, err := mw.CreateFormFile("goodreads_csv", "goodreads.csv")
	require.NoError(t, err)
	_, err = fw.Write([]byte(goodreadsCSVForImport))
	require.NoError(t, err)
	require.NoError(t, mw.Close())

	req, err := http.NewRequestWithContext(
		context.Background(),
		http.MethodPost,
		ts.URL+"/"+testApp.GetName()+"/books/import",
		&body,
	)
	require.NoError(t, err)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	req.AddCookie(&accessToken)

	client := ts.Client()
	client.CheckRedirect = func(_ *http.Request, _ []*http.Request) error {
		return http.ErrUseLastResponse
	}

	rs, err := client.Do(req)
	require.NoError(t, err)
	defer rs.Body.Close()

	assert.Equal(t, http.StatusSeeOther, rs.StatusCode)
	assert.True(
		t,
		strings.HasPrefix(rs.Header.Get("Location"), "/backlog/books?imported="),
	)
}

func TestImportBooks_DeadlineExceeded(t *testing.T) {
	// Wrap routes with a middleware that pre-cancels the request context,
	// simulating an expired HTTP server deadline during a large CSV import.
	// Before the fix, DB batch operations would fail with "context canceled".
	// After the fix, importBooksHandler uses context.WithoutCancel so the
	// import still completes.
	routes := getRoutes()
	wrapped := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithCancel(r.Context())
		cancel()
		routes.ServeHTTP(w, r.WithContext(ctx))
	})

	ts := httptest.NewServer(wrapped)
	defer ts.Close()

	var body bytes.Buffer
	mw := multipart.NewWriter(&body)
	fw, err := mw.CreateFormFile("goodreads_csv", "goodreads.csv")
	require.NoError(t, err)
	_, err = fw.Write([]byte(goodreadsCSVForImport))
	require.NoError(t, err)
	require.NoError(t, mw.Close())

	req, err := http.NewRequestWithContext(
		context.Background(),
		http.MethodPost,
		ts.URL+"/"+testApp.GetName()+"/books/import",
		&body,
	)
	require.NoError(t, err)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	req.AddCookie(&accessToken)

	client := ts.Client()
	client.CheckRedirect = func(_ *http.Request, _ []*http.Request) error {
		return http.ErrUseLastResponse
	}

	rs, err := client.Do(req)
	require.NoError(t, err)
	defer rs.Body.Close()

	assert.Equal(t, http.StatusSeeOther, rs.StatusCode)
}

func TestImportBooks_MissingFile(t *testing.T) {
	ts := httptest.NewServer(getRoutes())
	defer ts.Close()

	var body bytes.Buffer
	mw := multipart.NewWriter(&body)
	// Write a wrong field name to trigger "missing file".
	fw, err := mw.CreateFormFile("wrong_field", "file.csv")
	require.NoError(t, err)
	_, err = fw.Write([]byte("data"))
	require.NoError(t, err)
	require.NoError(t, mw.Close())

	req, err := http.NewRequestWithContext(
		context.Background(),
		http.MethodPost,
		ts.URL+"/"+testApp.GetName()+"/books/import",
		&body,
	)
	require.NoError(t, err)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	req.AddCookie(&accessToken)

	rs, err := ts.Client().Do(req)
	require.NoError(t, err)
	defer rs.Body.Close()

	assert.Equal(t, http.StatusBadRequest, rs.StatusCode)
}
