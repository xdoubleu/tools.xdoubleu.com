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

func overlapFor(
	t *testing.T,
	msg *booksv1.GetSourceStatsResponse,
	sources ...string,
) *booksv1.SourceComboStat {
	t.Helper()
	for _, o := range msg.Overlaps {
		if equalSourceSets(o.Sources, sources) {
			return o
		}
	}
	t.Fatalf("overlap combo %v missing from stats", sources)
	return nil
}

func missedOverlapFor(
	t *testing.T,
	msg *booksv1.GetSourceStatsResponse,
	sources ...string,
) *booksv1.SourceComboStat {
	t.Helper()
	for _, o := range msg.MissedOverlaps {
		if equalSourceSets(o.Sources, sources) {
			return o
		}
	}
	t.Fatalf("missed overlap combo %v missing from stats", sources)
	return nil
}

func equalSourceSets(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	set := make(map[string]bool, len(a))
	for _, s := range a {
		set[s] = true
	}
	for _, s := range b {
		if !set[s] {
			return false
		}
	}
	return true
}

func listBooksInExactSources(
	t *testing.T,
	client booksTestClient,
	sources ...string,
) *booksv1.ListBooksInExactSourcesResponse {
	t.Helper()
	req := connect.NewRequest(&booksv1.ListBooksInExactSourcesRequest{Sources: sources})
	req.Header().Set("Cookie", accessToken.String())
	resp, err := client.ListBooksInExactSources(context.Background(), req)
	require.NoError(t, err)
	return resp.Msg
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
	client, adminApp := newAdminBooksTestClientWithMockSources(t)
	before := getSourceStats(t, client)

	id := uuid.New()
	addTestBookWithISBN(t, "SourceStatsUniqueBook", isbnFromUUID(id))
	afterAdd := getSourceStats(t, client)
	assert.Equal(t, before.TotalBooks+1, afterAdd.TotalBooks)
	assert.Equal(t, before.NeverScanned+1, afterAdd.NeverScanned,
		"a freshly added book has never been scanned")

	// Scan via adminApp's own service — it has GB/UniCat configured
	// (confirmed-absent mocks), unlike the shared testApp (OL-only), so this
	// book's OL-unique status actually resolves instead of staying unknown.
	_, err := adminApp.Services.Books.BuildResyncProposals(
		context.Background(),
		adminApp.Logger,
		nil,
		false,
	)
	require.NoError(t, err)

	after := getSourceStats(t, client)
	// >= rather than ==: this is a full-catalog scan, and the shared test DB
	// may carry other never-fully-resolved books left by earlier tests that
	// also resolve to openlibrary-unique in the same pass — the specific
	// book's own membership is verified precisely by ListBooksInExactSources
	// (TestListBooksInExactSources_Admin_ReturnsOnlyUniqueBook).
	assert.GreaterOrEqual(
		t,
		statFor(t, after, "openlibrary").UniqueCount,
		statFor(t, before, "openlibrary").UniqueCount+1,
	)
	require.Len(t, after.Sources, 3)
}

// TestGetSourceStats_Admin_CountsOverlap verifies pairwise overlap counting:
// a book found by both OpenLibrary and Google Books (and not UniCat, which
// stays unconfigured/NULL) bumps the OpenLibrary+GoogleBooks overlap count,
// not the all-three count.
func TestGetSourceStats_Admin_CountsOverlap(t *testing.T) {
	client, adminApp := newAdminBooksTestClientWithTwoSources(t)
	before := getSourceStats(t, client)

	id := uuid.New()
	addTestBookWithISBN(t, "SourceStatsOverlapBook", isbnFromUUID(id))

	_, err := adminApp.Services.Books.BuildResyncProposals(
		context.Background(),
		adminApp.Logger,
		nil,
		false,
	)
	require.NoError(t, err)

	after := getSourceStats(t, client)
	// >= rather than ==: see TestGetSourceStats_Admin_CountsUnique — other
	// leftover books in the shared test DB may also resolve into this combo
	// in the same full-catalog scan. The specific book's membership is
	// verified precisely by ListBooksInExactSources
	// (TestListBooksInExactSources_Admin_ReturnsOverlapBook).
	assert.GreaterOrEqual(
		t,
		overlapFor(t, after, "openlibrary", "googlebooks").Count,
		overlapFor(t, before, "openlibrary", "googlebooks").Count+1,
	)
	assert.Equal(
		t,
		overlapFor(t, before, "openlibrary", "googlebooks", "unicat").Count,
		overlapFor(t, after, "openlibrary", "googlebooks", "unicat").Count,
		"the mock UniCat client never confirms a match, so all-three must stay 0",
	)
	require.Len(t, after.Overlaps, 4)
}

// TestGetSourceStats_Admin_CountsMissed verifies per-source missed counting: a
// book found only by OpenLibrary is a confirmed miss (IS FALSE, not NULL) for
// the two other configured sources, bumping their missed counts.
func TestGetSourceStats_Admin_CountsMissed(t *testing.T) {
	client, adminApp := newAdminBooksTestClientWithMockSources(t)
	before := getSourceStats(t, client)

	id := uuid.New()
	addTestBookWithISBN(t, "SourceStatsMissedBook", isbnFromUUID(id))

	_, err := adminApp.Services.Books.BuildResyncProposals(
		context.Background(),
		adminApp.Logger,
		nil,
		false,
	)
	require.NoError(t, err)

	after := getSourceStats(t, client)
	assert.GreaterOrEqual(
		t,
		statFor(t, after, "googlebooks").MissedCount,
		statFor(t, before, "googlebooks").MissedCount+1,
	)
	assert.GreaterOrEqual(
		t,
		statFor(t, after, "unicat").MissedCount,
		statFor(t, before, "unicat").MissedCount+1,
	)
	assert.Equal(
		t,
		statFor(t, before, "openlibrary").MissedCount,
		statFor(t, after, "openlibrary").MissedCount,
		"the mock OpenLibrary client always finds, so its missed count must not move",
	)
}

// TestGetSourceStats_MissedOverlapsMirrorUniqueCounts verifies the handler's
// missed_overlaps wiring: missed by exactly {A,B} (both confirmed miss) is
// the same book set as found-only-by-the-third-source, so it must always
// equal that source's Unique count — a structural invariant, independent of
// what's currently seeded in the shared test DB.
func TestGetSourceStats_MissedOverlapsMirrorUniqueCounts(t *testing.T) {
	client := newAdminBooksTestClient(t)
	msg := getSourceStats(t, client)

	require.Len(t, msg.MissedOverlaps, 4)
	assert.Equal(
		t,
		statFor(t, msg, "unicat").UniqueCount,
		missedOverlapFor(t, msg, "openlibrary", "googlebooks").Count,
	)
	assert.Equal(
		t,
		statFor(t, msg, "googlebooks").UniqueCount,
		missedOverlapFor(t, msg, "openlibrary", "unicat").Count,
	)
	assert.Equal(
		t,
		statFor(t, msg, "openlibrary").UniqueCount,
		missedOverlapFor(t, msg, "googlebooks", "unicat").Count,
	)
}

func TestListBooksInExactSources_NonAdmin_PermissionDenied(t *testing.T) {
	client := newBooksTestClient(t)
	req := connect.NewRequest(
		&booksv1.ListBooksInExactSourcesRequest{Sources: []string{"openlibrary"}},
	)
	req.Header().Set("Cookie", accessToken.String())

	_, err := client.ListBooksInExactSources(context.Background(), req)
	require.Error(t, err)
	var connErr *connect.Error
	require.ErrorAs(t, err, &connErr)
	assert.Equal(t, connect.CodePermissionDenied, connErr.Code())
}

func TestListBooksInExactSources_UnknownSource_InvalidArgument(t *testing.T) {
	client, _ := newAdminBooksTestClientWithMockSources(t)
	req := connect.NewRequest(
		&booksv1.ListBooksInExactSourcesRequest{Sources: []string{"bogus"}},
	)
	req.Header().Set("Cookie", accessToken.String())

	_, err := client.ListBooksInExactSources(context.Background(), req)
	require.Error(t, err)
	var connErr *connect.Error
	require.ErrorAs(t, err, &connErr)
	assert.Equal(t, connect.CodeInvalidArgument, connErr.Code())
}

func TestListBooksInExactSources_EmptySources_InvalidArgument(t *testing.T) {
	client, _ := newAdminBooksTestClientWithMockSources(t)
	req := connect.NewRequest(&booksv1.ListBooksInExactSourcesRequest{Sources: nil})
	req.Header().Set("Cookie", accessToken.String())

	_, err := client.ListBooksInExactSources(context.Background(), req)
	require.Error(t, err)
	var connErr *connect.Error
	require.ErrorAs(t, err, &connErr)
	assert.Equal(t, connect.CodeInvalidArgument, connErr.Code())
}

// TestListBooksInExactSources_Admin_ReturnsOnlyUniqueBook verifies a book
// found only by OpenLibrary (the mock source setup) shows up in the
// OpenLibrary unique list after a scan, and not in another source's list.
func TestListBooksInExactSources_Admin_ReturnsOnlyUniqueBook(t *testing.T) {
	client, adminApp := newAdminBooksTestClientWithMockSources(t)

	id := uuid.New()
	title := "SourceUniqueBooksTestBook"
	addTestBookWithISBN(t, title, isbnFromUUID(id))

	_, err := adminApp.Services.Books.BuildResyncProposals(
		context.Background(),
		adminApp.Logger,
		nil,
		false,
	)
	require.NoError(t, err)

	olMsg := listBooksInExactSources(t, client, "openlibrary")
	found := false
	for _, b := range olMsg.Books {
		if b.Title == title {
			found = true
		}
	}
	assert.True(t, found, "book unique to openlibrary must appear in its unique list")

	ucMsg := listBooksInExactSources(t, client, "unicat")
	for _, b := range ucMsg.Books {
		assert.NotEqual(
			t,
			title,
			b.Title,
			"book unique to openlibrary must not appear under unicat",
		)
	}
}

// TestListBooksInExactSources_Admin_ReturnsOverlapBook verifies a book found
// by exactly OpenLibrary+GoogleBooks appears when querying that pair, and
// not when querying OpenLibrary alone (which now means OL-only, not OL+GB).
func TestListBooksInExactSources_Admin_ReturnsOverlapBook(t *testing.T) {
	client, adminApp := newAdminBooksTestClientWithTwoSources(t)

	id := uuid.New()
	title := "SourceOverlapBooksTestBook"
	addTestBookWithISBN(t, title, isbnFromUUID(id))

	_, err := adminApp.Services.Books.BuildResyncProposals(
		context.Background(),
		adminApp.Logger,
		nil,
		false,
	)
	require.NoError(t, err)

	pairMsg := listBooksInExactSources(t, client, "openlibrary", "googlebooks")
	found := false
	for _, b := range pairMsg.Books {
		if b.Title == title {
			found = true
		}
	}
	assert.True(
		t,
		found,
		"book found by exactly OL+GB must appear in that combo's list",
	)

	olOnlyMsg := listBooksInExactSources(t, client, "openlibrary")
	for _, b := range olOnlyMsg.Books {
		assert.NotEqual(t, title, b.Title,
			"a book also found by GB must not appear in the OL-only list")
	}
}
