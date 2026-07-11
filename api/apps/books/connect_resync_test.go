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

// TestBuildResyncProposals_Service exercises the service layer end-to-end
// against the real DB. The test app's mock Open Library client always
// returns "The Odyssey" by Homer, so a book seeded with a different title
// must come back flagged. BuildResyncProposals never writes to a book or the
// cover cache — that only happens through ApplyResyncChoice.
func TestBuildResyncProposals_Service(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	id := uuid.New()
	book := addTestBookWithISBN(
		t,
		"ResyncTest-"+id.String()[:8],
		isbnFromUUID(id),
	)

	coverKey := "books/" + book.BookID.String() + "/cover.jpg"
	require.NoError(
		t,
		fakeStore.Put(ctx, coverKey, bytes.NewReader([]byte("img")), 3, "image/jpeg"),
	)

	n, err := testApp.Services.Books.BuildResyncProposals(ctx, testApp.Logger, nil)
	require.NoError(t, err)
	assert.Positive(t, n, "the mock provider always disagrees on title")

	exists, err := fakeStore.Exists(ctx, coverKey)
	require.NoError(t, err)
	assert.True(t, exists, "a scan must never touch the cover cache — it only reads")

	// The scan must persist per-source found flags and bump last_resync_at.
	scanned, err := testApp.Repositories.Books.GetBookByID(ctx, book.BookID)
	require.NoError(t, err)
	require.NotNil(t, scanned.OpenLibraryFound)
	assert.True(t, *scanned.OpenLibraryFound, "the mock provider always finds")
	assert.Nil(t, scanned.GoogleBooksFound, "unconfigured provider stays NULL")
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
	require.NotNil(t, rescanned.GoogleBooksFound)
	assert.True(t, *rescanned.GoogleBooksFound)
	require.NotNil(t, rescanned.UniCatFound)
	assert.True(t, *rescanned.UniCatFound)
}
