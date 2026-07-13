package books_test

import (
	"bytes"
	"context"
	"testing"

	"connectrpc.com/connect"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	booksv1 "tools.xdoubleu.com/gen/books/v1"
)

// TestConnectStartResync_Success verifies that an authenticated admin can
// trigger the resync scan endpoint and get a 200 response.
func TestConnectStartResync_Success(t *testing.T) {
	client := newAdminBooksTestClient(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	req := connect.NewRequest(&booksv1.StartResyncRequest{})
	req.Header().Set("Cookie", accessToken.String())

	resp, err := client.StartResync(ctx, req)
	require.NoError(t, err)
	assert.NotNil(t, resp)
}

// TestConnectStartResync_Force verifies the force flag round-trips through
// the RPC without erroring — the job itself asserts the flag is honored (see
// TestResyncOpenLibraryJob_Run in internal/jobs).
func TestConnectStartResync_Force(t *testing.T) {
	client := newAdminBooksTestClient(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	req := connect.NewRequest(&booksv1.StartResyncRequest{Force: true})
	req.Header().Set("Cookie", accessToken.String())

	resp, err := client.StartResync(ctx, req)
	require.NoError(t, err)
	assert.NotNil(t, resp)
}

// TestBuildResyncProposals_Service exercises the service layer end-to-end
// against the real DB. AddToLibrary enriches a new book from the same mocked
// Open Library client resync later queries (see enrichByISBN), so a freshly
// seeded book already agrees with the mock on every field resync would
// otherwise supply — a title-only mismatch alone is a mere difference, not a
// gap, and must not be flagged (see encodeIfFlagged). The test blanks the
// seeded book's description directly in the DB to create a genuine gap the
// mock source can fill, so the book is surfaced. BuildResyncProposals never
// writes to a book or the cover cache — that only happens through
// ApplyResyncChoice.
func TestBuildResyncProposals_Service(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	id := uuid.New()
	book := addTestBookWithISBN(
		t,
		"ResyncTest-"+id.String()[:8],
		isbnFromUUID(id),
	)
	_, err := testDB.Exec(ctx,
		`UPDATE books.books SET description = NULL WHERE id = $1`, book.BookID)
	require.NoError(t, err)

	coverKey := "books/" + book.BookID.String() + "/cover.jpg"
	require.NoError(
		t,
		fakeStore.Put(ctx, coverKey, bytes.NewReader([]byte("img")), 3, "image/jpeg"),
	)

	n, err := testApp.Services.Books.BuildResyncProposals(
		ctx,
		testApp.Logger,
		nil,
		false,
	)
	require.NoError(t, err)
	assert.Positive(t, n, "the mock provider can fill the description gap we created")

	exists, err := fakeStore.Exists(ctx, coverKey)
	require.NoError(t, err)
	assert.True(t, exists, "a scan must never touch the cover cache — it only reads")

	// The scan must persist per-source found flags and bump last_resync_at.
	scanned, err := testApp.Repositories.Books.GetBookByID(ctx, book.BookID)
	require.NoError(t, err)
	require.NotNil(t, scanned.OpenLibraryFound)
	assert.True(t, *scanned.OpenLibraryFound, "the mock provider always finds")
	assert.Nil(t, scanned.UniCatFound, "unconfigured provider stays NULL")
	assert.NotNil(t, scanned.LastResyncAt)

	proposals, err := testApp.Services.Books.ListResyncProposals(ctx)
	require.NoError(t, err)
	var found bool
	for _, p := range proposals {
		if p.BookID == book.BookID.String() {
			found = true
			require.NotEmpty(t, p.Sources)
			assert.Contains(t, p.Sources[0].Differs, "title")
		}
	}
	assert.True(t, found, "the seeded book must be flagged as differing")

	// Dismissing the proposal (source == "") must not write anything.
	err = testApp.Services.Books.ApplyResyncChoice(ctx, testApp.Logger, book.BookID, "")
	require.NoError(t, err)

	exists, err = fakeStore.Exists(ctx, coverKey)
	require.NoError(t, err)
	assert.True(t, exists, "dismissing a proposal must not touch the cover cache")
}

// TestConnectCancelResync_NonAdmin_PermissionDenied verifies CancelResync is
// admin-gated like every other resync RPC.
func TestConnectCancelResync_NonAdmin_PermissionDenied(t *testing.T) {
	client := newBooksTestClient(t)
	req := connect.NewRequest(&booksv1.CancelResyncRequest{})
	req.Header().Set("Cookie", accessToken.String())

	_, err := client.CancelResync(context.Background(), req)
	require.Error(t, err)
	var connErr *connect.Error
	require.ErrorAs(t, err, &connErr)
	assert.Equal(t, connect.CodePermissionDenied, connErr.Code())
}

// TestConnectCancelResync_Admin_NoopWhenNothingRunning verifies calling
// CancelResync while no scan is in progress succeeds without effect — there's
// nothing to stop.
func TestConnectCancelResync_Admin_NoopWhenNothingRunning(t *testing.T) {
	client := newAdminBooksTestClient(t)
	req := connect.NewRequest(&booksv1.CancelResyncRequest{})
	req.Header().Set("Cookie", accessToken.String())

	resp, err := client.CancelResync(context.Background(), req)
	require.NoError(t, err)
	assert.NotNil(t, resp)
}

// TestListCatalogBooks_OrdersLeastCoveredFirst verifies the scan order fix:
// a book with no sources confirmed found (the quota-starved case under a
// full-catalog force resync) must sort before a book already confirmed found
// by every source, so a rate-limited or interrupted run spends its budget on
// the books that most need checking rather than the already-covered ones.
func TestListCatalogBooks_OrdersLeastCoveredFirst(t *testing.T) {
	ctx := context.Background()
	id := uuid.New()
	suffix := id.String()[:8]
	neverScanned := addTestBookWithISBN(
		t,
		"ZZZOrderTest-Never-"+suffix,
		isbnFromUUID(id),
	)

	id2 := uuid.New()
	fullyCovered := addTestBookWithISBN(
		t,
		"ZZZOrderTest-Full-"+suffix,
		isbnFromUUID(id2),
	)
	trueVal := true
	require.NoError(t, testApp.Repositories.Books.UpdateResyncScanStatus(
		ctx, fullyCovered.BookID, &trueVal, &trueVal, &trueVal,
	))

	books, err := testApp.Repositories.Books.ListCatalogBooks(ctx)
	require.NoError(t, err)

	var neverIdx, fullIdx = -1, -1
	for i, b := range books {
		switch b.ID {
		case neverScanned.BookID:
			neverIdx = i
		case fullyCovered.BookID:
			fullIdx = i
		}
	}
	require.NotEqual(t, -1, neverIdx, "never-scanned book must be in the list")
	require.NotEqual(t, -1, fullIdx, "fully-covered book must be in the list")
	assert.Less(
		t,
		neverIdx,
		fullIdx,
		"a book with fewer confirmed-found sources must scan before a fully-covered one",
	)
}

// TestUpdateResyncScanStatus_NilFlagPreservesPriorValue verifies the
// COALESCE-preserve write: a nil flag (source not resolved this pass — not
// configured, skipped, or errored) must never clobber an already-known found
// value. Regression for a throttled/errored scan silently flipping a known
// "found" source back to "not found".
func TestUpdateResyncScanStatus_NilFlagPreservesPriorValue(t *testing.T) {
	ctx := context.Background()
	book := addTestBookWithISBN(
		t, "ScanStatusPreserveTest-"+uuid.New().String()[:8], "9780000099099",
	)

	trueVal := true
	require.NoError(t, testApp.Repositories.Books.UpdateResyncScanStatus(
		ctx, book.BookID, &trueVal, &trueVal, &trueVal,
	))

	scanned, err := testApp.Repositories.Books.GetBookByID(ctx, book.BookID)
	require.NoError(t, err)
	require.NotNil(t, scanned.OpenLibraryFound)
	assert.True(t, *scanned.OpenLibraryFound)

	// A second pass with all-nil flags (every source unresolved this time)
	// must leave the previously-known true values untouched.
	require.NoError(t, testApp.Repositories.Books.UpdateResyncScanStatus(
		ctx, book.BookID, nil, nil, nil,
	))

	rescanned, err := testApp.Repositories.Books.GetBookByID(ctx, book.BookID)
	require.NoError(t, err)
	require.NotNil(t, rescanned.OpenLibraryFound)
	assert.True(
		t,
		*rescanned.OpenLibraryFound,
		"nil must preserve, not overwrite with NULL",
	)
	require.NotNil(t, rescanned.UniCatFound)
	assert.True(t, *rescanned.UniCatFound)
	require.NotNil(t, rescanned.HardcoverFound)
	assert.True(t, *rescanned.HardcoverFound)
}
