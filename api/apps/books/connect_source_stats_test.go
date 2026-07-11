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

// TestGetSourceStats_Admin_CountsApplied verifies applied provenance shows up
// in the stats: applying a source to a book bumps that source's applied count.
// Counts are compared before/after because the test DB is shared.
func TestGetSourceStats_Admin_CountsApplied(t *testing.T) {
	client := newAdminBooksTestClientWithMockSources(t)
	before := getSourceStats(t, client)

	id := uuid.New()
	ub := addTestBookWithISBN(t, "SourceStatsApplyBook", isbnFromUUID(id))
	afterAdd := getSourceStats(t, client)
	assert.Equal(t, before.TotalBooks+1, afterAdd.TotalBooks)
	assert.Equal(t, before.NeverScanned+1, afterAdd.NeverScanned,
		"a freshly added book has never been scanned")

	applyReq := connect.NewRequest(&booksv1.ApplyBookSourceRequest{

		BookId: ub.BookID.String(),
		Source: "openlibrary",
	})
	applyReq.Header().Set("Cookie", accessToken.String())
	_, err := client.ApplyBookSource(context.Background(), applyReq)
	require.NoError(t, err)

	after := getSourceStats(t, client)
	assert.Equal(
		t,
		statFor(t, before, "openlibrary").AppliedCount+1,
		statFor(t, after, "openlibrary").AppliedCount,
	)
	require.Len(t, after.Sources, 3)
}
