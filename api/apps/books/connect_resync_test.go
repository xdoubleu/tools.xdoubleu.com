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

	book := addTestBookWithISBN(
		t,
		"ResyncTest-"+uuid.New().String()[:8],
		"9780000099001",
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
