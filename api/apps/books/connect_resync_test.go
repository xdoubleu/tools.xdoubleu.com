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

// TestConnectResyncOpenLibrary_Success verifies that an authenticated user can
// trigger the resync endpoint and get a 200 response.
func TestConnectResyncOpenLibrary_Success(t *testing.T) {
	client := newAdminBooksTestClient(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	req := connect.NewRequest(&booksv1.ResyncOpenLibraryRequest{})
	req.Header().Set("Cookie", accessToken.String())

	resp, err := client.ResyncOpenLibrary(ctx, req)
	require.NoError(t, err)
	assert.NotNil(t, resp)
}

// TestResyncAllFromOpenLibrary_Service exercises the service layer end-to-end
// against the real DB. It seeds a book that already has a cover_url and
// verifies that resync does not touch its R2 cover cache — resync is
// additive-only and must never clobber data that already exists.
//
// The cache-bust path (no existing cover → OL provides one → cache busted) is
// covered exhaustively by the unit tests in
// internal/services/book_resync_test.go, which avoid the AddToLibrary
// enrichment that always fills in a cover via the mock OL client.
func TestResyncAllFromOpenLibrary_Service(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	uid := uuid.New().String()[:8]
	book := addTestBookWithISBN(t, "ResyncTest-"+uid, "9780000099001")

	// Pre-seed a cached cover and a missing marker so we can verify neither is
	// deleted when the book already has a cover_url.
	coverKey := "books/" + book.BookID.String() + "/cover.jpg"
	missingKey := "books/" + book.BookID.String() + "/cover.missing"
	require.NoError(
		t,
		fakeStore.Put(ctx, coverKey, bytes.NewReader([]byte("img")), 3, "image/jpeg"),
	)
	require.NoError(
		t,
		fakeStore.Put(
			ctx,
			missingKey,
			bytes.NewReader([]byte{}),
			0,
			"application/octet-stream",
		),
	)

	exists, err := fakeStore.Exists(ctx, coverKey)
	require.NoError(t, err)
	require.True(t, exists, "cover should be in store before resync")

	n, resyncErr := testApp.Services.Books.ResyncAllFromOpenLibrary(
		ctx,
		testApp.Logger,
		nil,
	)
	require.NoError(t, resyncErr)
	assert.GreaterOrEqual(t, n, 0, "resync should complete without error")

	// Cover already existed — cache must NOT be disturbed.
	exists, err = fakeStore.Exists(ctx, coverKey)
	require.NoError(t, err)
	assert.True(
		t, exists,
		"cover.jpg cache must be preserved when the book already has a cover URL",
	)

	missing, err := fakeStore.Exists(ctx, missingKey)
	require.NoError(t, err)
	assert.True(
		t, missing,
		"cover.missing marker must be preserved when the book already has a cover URL",
	)
}
