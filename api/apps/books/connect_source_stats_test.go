package books_test

import (
	"context"
	"testing"

	"connectrpc.com/connect"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	booksv1 "tools.xdoubleu.com/gen/books/v1"
)

func getSourceStats(
	t *testing.T,
	client booksTestClient,
) *booksv1.GetSourceStatsResponse {
	t.Helper()
	req := connect.NewRequest(&booksv1.GetSourceStatsRequest{})
	req.Header().Set("Cookie", accessToken.String())
	resp, err := client.GetSourceStats(context.Background(), req)
	require.NoError(t, err)
	return resp.Msg
}

func statFor(
	t *testing.T,
	msg *booksv1.GetSourceStatsResponse,
	source string,
) *booksv1.SourceStat {
	t.Helper()
	for _, s := range msg.Sources {
		if s.Source == source {
			return s
		}
	}
	t.Fatalf("source %q missing from stats", source)
	return nil
}

func TestGetSourceStats_NonAdmin_PermissionDenied(t *testing.T) {
	client := newBooksTestClient(t)
	req := connect.NewRequest(&booksv1.GetSourceStatsRequest{})
	req.Header().Set("Cookie", accessToken.String())

	_, err := client.GetSourceStats(context.Background(), req)
	require.Error(t, err)
	var connErr *connect.Error
	require.ErrorAs(t, err, &connErr)
	assert.Equal(t, connect.CodePermissionDenied, connErr.Code())
}

// TestGetSourceStats_Admin_CountsUnique verifies unique-source counting: a
// book scanned with only OpenLibrary configured is found there and nowhere
// else, so it bumps OpenLibrary's unique count.
// Counts are compared before/after because the test DB is shared.
func TestGetSourceStats_Admin_CountsUnique(t *testing.T) {
	client := newAdminBooksTestClientWithMockSources(t)
	before := getSourceStats(t, client)

	id := uuid.New()
	addTestBookWithISBN(t, "SourceStatsUniqueBook", isbnFromUUID(id))
	afterAdd := getSourceStats(t, client)
	assert.Equal(t, before.TotalBooks+1, afterAdd.TotalBooks)
	assert.Equal(t, before.NeverScanned+1, afterAdd.NeverScanned,
		"a freshly added book has never been scanned")

	_, err := testApp.Services.Books.BuildResyncProposals(
		context.Background(),
		testApp.Logger,
		nil,
	)
	require.NoError(t, err)

	after := getSourceStats(t, client)
	assert.Equal(
		t,
		statFor(t, before, "openlibrary").UniqueCount+1,
		statFor(t, after, "openlibrary").UniqueCount,
	)
	require.Len(t, after.Sources, 3)
}

func TestListSourceUniqueBooks_NonAdmin_PermissionDenied(t *testing.T) {
	client := newBooksTestClient(t)
	req := connect.NewRequest(
		&booksv1.ListSourceUniqueBooksRequest{Source: "openlibrary"},
	)
	req.Header().Set("Cookie", accessToken.String())

	_, err := client.ListSourceUniqueBooks(context.Background(), req)
	require.Error(t, err)
	var connErr *connect.Error
	require.ErrorAs(t, err, &connErr)
	assert.Equal(t, connect.CodePermissionDenied, connErr.Code())
}

func TestListSourceUniqueBooks_UnknownSource_InvalidArgument(t *testing.T) {
	client := newAdminBooksTestClientWithMockSources(t)
	req := connect.NewRequest(&booksv1.ListSourceUniqueBooksRequest{Source: "bogus"})
	req.Header().Set("Cookie", accessToken.String())

	_, err := client.ListSourceUniqueBooks(context.Background(), req)
	require.Error(t, err)
	var connErr *connect.Error
	require.ErrorAs(t, err, &connErr)
	assert.Equal(t, connect.CodeInvalidArgument, connErr.Code())
}

// TestListSourceUniqueBooks_Admin_ReturnsOnlyUniqueBook verifies a book found
// only by OpenLibrary (the mock source setup) shows up in the OpenLibrary
// unique list after a scan, and not in another source's list.
func TestListSourceUniqueBooks_Admin_ReturnsOnlyUniqueBook(t *testing.T) {
	client := newAdminBooksTestClientWithMockSources(t)

	id := uuid.New()
	title := "SourceUniqueBooksTestBook"
	addTestBookWithISBN(t, title, isbnFromUUID(id))

	_, err := testApp.Services.Books.BuildResyncProposals(
		context.Background(),
		testApp.Logger,
		nil,
	)
	require.NoError(t, err)

	olReq := connect.NewRequest(
		&booksv1.ListSourceUniqueBooksRequest{Source: "openlibrary"},
	)
	olReq.Header().Set("Cookie", accessToken.String())
	olResp, err := client.ListSourceUniqueBooks(context.Background(), olReq)
	require.NoError(t, err)
	found := false
	for _, b := range olResp.Msg.Books {
		if b.Title == title {
			found = true
		}
	}
	assert.True(t, found, "book unique to openlibrary must appear in its unique list")

	ucReq := connect.NewRequest(&booksv1.ListSourceUniqueBooksRequest{Source: "unicat"})
	ucReq.Header().Set("Cookie", accessToken.String())
	ucResp, err := client.ListSourceUniqueBooks(context.Background(), ucReq)
	require.NoError(t, err)
	for _, b := range ucResp.Msg.Books {
		assert.NotEqual(
			t,
			title,
			b.Title,
			"book unique to openlibrary must not appear under unicat",
		)
	}
}
