package books_test

import (
	"bytes"
	"context"
	"testing"

	"connectrpc.com/connect/v2"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	booksv1 "tools.xdoubleu.com/gen/books/v1"
	"tools.xdoubleu.com/internal/testhelper"
)

// TestConnectStartResync_Success checks an admin can start a scan.
func TestConnectStartResync_Success(t *testing.T) {
	client := newAdminBooksTestClient(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	req := &booksv1.StartResyncRequest{}
	ctx, info := connect.NewClientContext(ctx)
	info.RequestHeader().Set("Cookie", accessToken.String())

	resp, err := client.StartResync(ctx, req)
	require.NoError(t, err)
	assert.NotNil(t, resp)
}

// TestConnectStartResync_Force checks the force flag round-trips; the job
// tests check it's honored.
func TestConnectStartResync_Force(t *testing.T) {
	client := newAdminBooksTestClient(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	req := &booksv1.StartResyncRequest{Force: true}
	ctx, info := connect.NewClientContext(ctx)
	info.RequestHeader().Set("Cookie", accessToken.String())

	resp, err := client.StartResync(ctx, req)
	require.NoError(t, err)
	assert.NotNil(t, resp)
}

// TestBuildResyncProposals_Service: adding a book already enriches it from
// the mock, so the test blanks its description to create a gap worth
// flagging. The scan itself never writes to the book.
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

	n, err := testApp.TestSeeds().Services.Books.BuildResyncProposals(
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

	scanned, err := testApp.TestSeeds().Repositories.Books.GetBookByID(ctx, book.BookID)
	require.NoError(t, err)
	require.NotNil(t, scanned.HardcoverFound)
	assert.True(t, *scanned.HardcoverFound, "the mock provider always finds")
	assert.Nil(t, scanned.UniCatFound, "unconfigured provider stays NULL")
	assert.NotNil(t, scanned.LastResyncAt)

	proposals, err := testApp.TestSeeds().Services.Books.ListResyncProposals(ctx)
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

	err = testApp.TestSeeds().Services.Books.ApplyResyncChoice(
		ctx,
		testApp.Logger,
		book.BookID,
		"",
	)
	require.NoError(t, err)

	exists, err = fakeStore.Exists(ctx, coverKey)
	require.NoError(t, err)
	assert.True(t, exists, "dismissing a proposal must not touch the cover cache")
}

// TestConnectCancelResync_NonAdmin_PermissionDenied checks the admin gate.
func TestConnectCancelResync_NonAdmin_PermissionDenied(t *testing.T) {
	client := newBooksTestClient(t)
	req := &booksv1.CancelResyncRequest{}

	_, err := client.CancelResync(
		testhelper.WithCookie(context.Background(), accessToken.String()),
		req,
	)
	require.Error(t, err)
	var connErr *connect.Error
	require.ErrorAs(t, err, &connErr)
	assert.Equal(t, connect.CodePermissionDenied, connErr.Code())
}

// TestConnectCancelResync_Admin_NoopWhenNothingRunning checks the no-op.
func TestConnectCancelResync_Admin_NoopWhenNothingRunning(t *testing.T) {
	client := newAdminBooksTestClient(t)
	req := &booksv1.CancelResyncRequest{}

	resp, err := client.CancelResync(
		testhelper.WithCookie(context.Background(), accessToken.String()),
		req,
	)
	require.NoError(t, err)
	assert.NotNil(t, resp)
}

// TestListCatalogBooks_OrdersLeastCoveredFirst: uncovered books sort before
// fully covered ones.
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
	require.NoError(t, testApp.TestSeeds().Repositories.Books.UpdateResyncScanStatus(
		ctx, fullyCovered.BookID, &trueVal, &trueVal,
	))

	books, err := testApp.TestSeeds().Repositories.Books.ListCatalogBooks(ctx)
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

// TestUpdateResyncScanStatus_NilFlagPreservesPriorValue: a nil flag never
// clobbers a known value.
func TestUpdateResyncScanStatus_NilFlagPreservesPriorValue(t *testing.T) {
	ctx := context.Background()
	book := addTestBookWithISBN(
		t, "ScanStatusPreserveTest-"+uuid.New().String()[:8], "9780000099099",
	)

	trueVal := true
	require.NoError(t, testApp.TestSeeds().Repositories.Books.UpdateResyncScanStatus(
		ctx, book.BookID, &trueVal, &trueVal,
	))

	scanned, err := testApp.TestSeeds().Repositories.Books.GetBookByID(ctx, book.BookID)
	require.NoError(t, err)
	require.NotNil(t, scanned.UniCatFound)
	assert.True(t, *scanned.UniCatFound)

	require.NoError(t, testApp.TestSeeds().Repositories.Books.UpdateResyncScanStatus(
		ctx, book.BookID, nil, nil,
	))

	rescanned, err := testApp.TestSeeds().Repositories.Books.GetBookByID(ctx, book.BookID)
	require.NoError(t, err)
	require.NotNil(t, rescanned.UniCatFound)
	assert.True(
		t,
		*rescanned.UniCatFound,
		"nil must preserve, not overwrite with NULL",
	)
	require.NotNil(t, rescanned.HardcoverFound)
	assert.True(t, *rescanned.HardcoverFound)
}
