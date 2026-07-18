package reading_test

import (
	"context"
	"testing"

	"connectrpc.com/connect"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	readingv1 "tools.xdoubleu.com/gen/reading/v1"
)

func getSourceStats(
	t *testing.T,
	client booksTestClient,
) *readingv1.GetSourceStatsResponse {
	t.Helper()
	req := connect.NewRequest(&readingv1.GetSourceStatsRequest{})
	req.Header().Set("Cookie", accessToken.String())
	resp, err := client.GetSourceStats(context.Background(), req)
	require.NoError(t, err)
	return resp.Msg
}

func statFor(
	t *testing.T,
	msg *readingv1.GetSourceStatsResponse,
	source string,
) *readingv1.SourceStat {
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
	msg *readingv1.GetSourceStatsResponse,
	sources ...string,
) *readingv1.SourceComboStat {
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
	msg *readingv1.GetSourceStatsResponse,
	sources ...string,
) *readingv1.SourceComboStat {
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
) *readingv1.ListBooksInExactSourcesResponse {
	t.Helper()
	req := connect.NewRequest(
		&readingv1.ListBooksInExactSourcesRequest{Sources: sources},
	)
	req.Header().Set("Cookie", accessToken.String())
	resp, err := client.ListBooksInExactSources(context.Background(), req)
	require.NoError(t, err)
	return resp.Msg
}

func TestGetSourceStats_NonAdmin_PermissionDenied(t *testing.T) {
	client := newBooksTestClient(t)
	req := connect.NewRequest(&readingv1.GetSourceStatsRequest{})
	req.Header().Set("Cookie", accessToken.String())

	_, err := client.GetSourceStats(context.Background(), req)
	require.Error(t, err)
	var connErr *connect.Error
	require.ErrorAs(t, err, &connErr)
	assert.Equal(t, connect.CodePermissionDenied, connErr.Code())
}

// TestGetSourceStats_Admin_CountsUnique verifies unique-source counting: a
// book scanned with only Hardcover populated (UniCat is configured but
// confirmed-absent — see newAdminBooksTestClientWithMockSources) is found
// there and nowhere else, so it bumps Hardcover's unique count.
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

	// Scan via adminApp's own service — it has UniCat (confirmed-absent) and
	// Hardcover (populated) configured, unlike the shared testApp, so this
	// book's Hardcover-unique status actually resolves instead of staying
	// unknown.
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
	// also resolve to hardcover-unique in the same pass — the specific
	// book's own membership is verified precisely by ListBooksInExactSources
	// (TestListBooksInExactSources_Admin_ReturnsOnlyUniqueBook).
	assert.GreaterOrEqual(
		t,
		statFor(t, after, "hardcover").UniqueCount,
		statFor(t, before, "hardcover").UniqueCount+1,
	)
	require.Len(t, after.Sources, 2)
}

// TestGetSourceStats_Admin_CountsOverlap verifies overlap counting: a book
// found by both UniCat and Hardcover (newAdminBooksTestClientWithTwoSources
// wires both to always match) bumps the combined-overlap count.
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
		overlapFor(t, after, "unicat", "hardcover").Count,
		overlapFor(t, before, "unicat", "hardcover").Count+1,
	)
	// With only two sources there is exactly one overlap combo: both.
	require.Len(t, after.Overlaps, 1)
}

// TestGetSourceStats_Admin_CountsMissed verifies per-source missed counting: a
// book found only by Hardcover is a confirmed miss (IS FALSE, not NULL) for
// UniCat, bumping UniCat's missed count while Hardcover's stays put.
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
		statFor(t, after, "unicat").MissedCount,
		statFor(t, before, "unicat").MissedCount+1,
	)
	assert.Equal(
		t,
		statFor(t, before, "hardcover").MissedCount,
		statFor(t, after, "hardcover").MissedCount,
		"the mock Hardcover client always finds, so its missed count must not move",
	)
}

// TestGetSourceStats_MissedOverlaps_Structural verifies the handler's
// missed_overlaps wiring: with only two configured sources there is exactly
// one combo (both), independent of what's currently seeded in the shared
// test DB.
func TestGetSourceStats_MissedOverlaps_Structural(t *testing.T) {
	client := newAdminBooksTestClient(t)
	msg := getSourceStats(t, client)

	require.Len(t, msg.MissedOverlaps, 1)
	combo := missedOverlapFor(t, msg, "unicat", "hardcover")
	assert.GreaterOrEqual(t, combo.Count, int32(0))
}

func TestListBooksInExactSources_NonAdmin_PermissionDenied(t *testing.T) {
	client := newBooksTestClient(t)
	req := connect.NewRequest(
		&readingv1.ListBooksInExactSourcesRequest{Sources: []string{"hardcover"}},
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
		&readingv1.ListBooksInExactSourcesRequest{Sources: []string{"bogus"}},
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
	req := connect.NewRequest(&readingv1.ListBooksInExactSourcesRequest{Sources: nil})
	req.Header().Set("Cookie", accessToken.String())

	_, err := client.ListBooksInExactSources(context.Background(), req)
	require.Error(t, err)
	var connErr *connect.Error
	require.ErrorAs(t, err, &connErr)
	assert.Equal(t, connect.CodeInvalidArgument, connErr.Code())
}

// TestListBooksInExactSources_Admin_ReturnsOnlyUniqueBook verifies a book
// found only by Hardcover (the mock source setup) shows up in the Hardcover
// unique list after a scan, and not in UniCat's list.
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

	hcMsg := listBooksInExactSources(t, client, "hardcover")
	found := false
	for _, b := range hcMsg.Books {
		if b.Title == title {
			found = true
		}
	}
	assert.True(t, found, "book unique to hardcover must appear in its unique list")

	ucMsg := listBooksInExactSources(t, client, "unicat")
	for _, b := range ucMsg.Books {
		assert.NotEqual(
			t,
			title,
			b.Title,
			"book unique to hardcover must not appear under unicat",
		)
	}
}

// TestListBooksInExactSources_Admin_ReturnsOverlapBook verifies a book found
// by both UniCat+Hardcover appears when querying that pair, and not when
// querying Hardcover alone (which now means HC-only, not UC+HC).
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

	pairMsg := listBooksInExactSources(t, client, "unicat", "hardcover")
	found := false
	for _, b := range pairMsg.Books {
		if b.Title == title {
			found = true
		}
	}
	assert.True(
		t,
		found,
		"book found by exactly UC+HC must appear in that combo's list",
	)

	hcOnlyMsg := listBooksInExactSources(t, client, "hardcover")
	for _, b := range hcOnlyMsg.Books {
		assert.NotEqual(t, title, b.Title,
			"a book also found by UniCat must not appear in the Hardcover-only list")
	}
}
