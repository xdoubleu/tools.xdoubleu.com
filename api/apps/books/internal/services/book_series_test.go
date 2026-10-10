//nolint:testpackage // testing unexported service helpers
package services

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"tools.xdoubleu.com/apps/books/internal/models"
	"tools.xdoubleu.com/apps/books/internal/repositories"
	"tools.xdoubleu.com/apps/books/pkg/hardcover"
	"tools.xdoubleu.com/apps/books/pkg/objectstore"
	"tools.xdoubleu.com/internal/logging"
)

func fptr(f float64) *float64 { return &f }

func iptr(i int) *int { return &i }

func seriesAt(name string, pos *float64) *models.BookSeries {
	return &models.BookSeries{Name: name, Position: pos, Total: nil}
}

func hcDiscworldRef() *hardcover.SeriesRef {
	return &hardcover.SeriesRef{Name: "Discworld", Position: fptr(4), Total: iptr(41)}
}

func TestBuildResyncProposals_FillsEmptySeries(t *testing.T) {
	id := uuid.New()
	isbn := "9780552131063"
	//nolint:exhaustruct // partial
	book := models.Book{ID: id, Title: "Mort", ISBN13: &isbn}
	repo := &fakeBooksResync{books: []models.Book{book}} //nolint:exhaustruct // partial
	hc := &fakeHCClient{                                 //nolint:exhaustruct // partial
		byISBN: &hardcover.ExternalBook{ //nolint:exhaustruct // partial
			Title: "Mort", ISBN13: &isbn, Series: hcDiscworldRef(),
		},
	}
	svc := &BookService{ //nolint:exhaustruct // partial
		logger:       logging.NewNopLogger(),
		resyncSource: repo,
		hardcover:    hc,
		objectStore:  objectstore.NewFake(),
	}

	_, err := svc.BuildResyncProposals(
		context.Background(), logging.NewNopLogger(), nil, true,
	)
	require.NoError(t, err)
	require.Contains(t, repo.seriesSet, id)
	got := repo.seriesSet[id]
	assert.Equal(t, "Discworld", got.Name)
	assert.InDelta(t, 4.0, *got.Position, 0)
	assert.Equal(t, 41, *got.Total)
}

func TestBuildResyncProposals_KeepsExistingSeries(t *testing.T) {
	id := uuid.New()
	isbn := "9780552131063"
	book := models.Book{ //nolint:exhaustruct // partial
		ID: id, Title: "Mort", ISBN13: &isbn,
		Series: seriesAt("Death", fptr(1)),
	}
	repo := &fakeBooksResync{books: []models.Book{book}} //nolint:exhaustruct // partial
	hc := &fakeHCClient{                                 //nolint:exhaustruct // partial
		byISBN: &hardcover.ExternalBook{ //nolint:exhaustruct // partial
			Title: "Mort", Series: hcDiscworldRef(),
		},
	}
	svc := &BookService{ //nolint:exhaustruct // partial
		logger:       logging.NewNopLogger(),
		resyncSource: repo,
		hardcover:    hc,
		objectStore:  objectstore.NewFake(),
	}

	_, err := svc.BuildResyncProposals(
		context.Background(), logging.NewNopLogger(), nil, true,
	)
	require.NoError(t, err)
	assert.Empty(t, repo.seriesSet, "a hand-set series must not be overwritten")
}

func TestApplyResyncChoice_WritesSeriesOnlyWhenSourceHasOne(t *testing.T) {
	for _, tc := range []struct {
		name     string
		proposal SourceProposal
		wantSet  bool
	}{
		{
			name: "with series",
			proposal: SourceProposal{ //nolint:exhaustruct // partial
				Source: "hardcover", Title: "Mort",
				SeriesName: "Discworld", SeriesPosition: fptr(4),
			},
			wantSet: true,
		},
		{
			name: "without series",
			proposal: SourceProposal{ //nolint:exhaustruct // partial
				Source: "hardcover", Title: "Mort",
			},
			wantSet: false,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			bookID := uuid.New()
			raw, err := json.Marshal([]SourceProposal{tc.proposal})
			require.NoError(t, err)
			repo := &fakeBooksResync{ //nolint:exhaustruct // zero values fine
				proposalRows: map[uuid.UUID]repositories.ResyncProposalRow{
					bookID: {
						//nolint:exhaustruct // partial
						Book:          models.Book{ID: bookID, Title: "Mort"},
						ProposalsJSON: raw,
					},
				},
			}
			svc := &BookService{ //nolint:exhaustruct // partial
				resyncSource: repo,
				objectStore:  objectstore.NewFake(),
			}

			err = svc.ApplyResyncChoice(
				context.Background(), logging.NewNopLogger(), bookID, "hardcover",
			)
			require.NoError(t, err)
			_, set := repo.seriesSet[bookID]
			assert.Equal(t, tc.wantSet, set)
		})
	}
}

func TestComputeDifferences_Series(t *testing.T) {
	book := models.Book{ //nolint:exhaustruct // partial
		Title:  "Mort",
		Series: seriesAt("Discworld", fptr(4)),
	}
	same := SourceProposal{ //nolint:exhaustruct // partial
		Title: "Mort", SeriesName: "Discworld", SeriesPosition: fptr(4),
	}
	moved := same
	moved.SeriesPosition = fptr(5)
	unordered := same
	unordered.SeriesPosition = nil

	assert.NotContains(t, computeDifferences(book, same), "series")
	assert.Contains(t, computeDifferences(book, moved), "series")
	assert.Contains(t, computeDifferences(book, unordered), "series")
	assert.Contains(t, computeDifferences(models.Book{Title: "Mort"}, same), "series") //nolint:exhaustruct,lll // partial
}

func TestLibraryProposal_CarriesSeries(t *testing.T) {
	p := libraryProposal(models.Book{ //nolint:exhaustruct // partial
		Title:  "Mort",
		Series: &models.BookSeries{Name: "Discworld", Position: fptr(4), Total: iptr(41)},
	})
	assert.Equal(t, "Discworld", p.SeriesName)
	assert.InDelta(t, 4.0, *p.SeriesPosition, 0)
	assert.Equal(t, 41, *p.SeriesTotal)
}

func ownedInSeries(title string, pos *float64) models.UserBook {
	return models.UserBook{ //nolint:exhaustruct // partial
		ID: uuid.New(),
		Book: &models.Book{ //nolint:exhaustruct // partial
			Title:  title,
			Series: seriesAt("Discworld", pos),
		},
	}
}

func TestMergeSeriesEntries(t *testing.T) {
	owned := []models.UserBook{
		ownedInSeries("Mort", fptr(4)),
		ownedInSeries("The Colour of Magic", nil),
		ownedInSeries("Unlisted Short Story", nil),
	}
	hc := &hardcover.Series{
		Name:  "Discworld",
		Total: iptr(41),
		Books: []hardcover.SeriesBook{
			{Position: 1, Book: hardcover.ExternalBook{Title: "The Colour of Magic"}}, //nolint:exhaustruct,lll // partial
			{Position: 2, Book: hardcover.ExternalBook{Title: "The Light Fantastic"}}, //nolint:exhaustruct,lll // partial
			{Position: 4, Book: hardcover.ExternalBook{Title: "Mort (Discworld #4)"}}, //nolint:exhaustruct,lll // partial
		},
	}

	entries := mergeSeriesEntries("Discworld", owned, hc)
	require.Len(t, entries, 4)

	assert.Equal(t, owned[1].ID, entries[0].UserBook.ID, "title match")
	assert.InDelta(t, 1.0, *entries[0].Position, 0, "Hardcover fills a missing position")

	require.NotNil(t, entries[1].External)
	assert.Nil(t, entries[1].UserBook)
	assert.Equal(t, "The Light Fantastic", entries[1].External.Title)
	assert.Equal(t, "Discworld", entries[1].External.SeriesName)
	assert.InDelta(t, 2.0, *entries[1].External.SeriesPosition, 0)

	assert.Equal(t, owned[0].ID, entries[2].UserBook.ID, "position match")

	assert.Equal(t, owned[2].ID, entries[3].UserBook.ID, "unmatched, unpositioned last")
	assert.Nil(t, entries[3].Position)
}

func TestMergeSeriesEntries_LibraryOnly(t *testing.T) {
	owned := []models.UserBook{
		ownedInSeries("B", nil),
		ownedInSeries("A", fptr(2)),
	}
	entries := mergeSeriesEntries("Discworld", owned, nil)
	require.Len(t, entries, 2)
	assert.Equal(t, owned[1].ID, entries[0].UserBook.ID)
	assert.Equal(t, owned[0].ID, entries[1].UserBook.ID)
}

func TestFetchHardcoverSeries(t *testing.T) {
	ctx := context.Background()
	logger := logging.NewNopLogger()

	unconfigured := &BookService{logger: logger} //nolint:exhaustruct // partial
	hc, ok := unconfigured.fetchHardcoverSeries(ctx, "X")
	assert.Nil(t, hc)
	assert.False(t, ok, "no client means missing volumes are unknown")

	failing := &BookService{ //nolint:exhaustruct // partial
		logger:    logger,
		hardcover: &fakeHCClient{err: errors.New("boom")}, //nolint:exhaustruct // partial
	}
	hc, ok = failing.fetchHardcoverSeries(ctx, "X")
	assert.Nil(t, hc)
	assert.False(t, ok)

	unknown := &BookService{ //nolint:exhaustruct // partial
		logger:    logger,
		hardcover: &fakeHCClient{}, //nolint:exhaustruct // series nil -> ErrNotFound
	}
	hc, ok = unknown.fetchHardcoverSeries(ctx, "X")
	assert.Nil(t, hc)
	assert.True(t, ok, "a series Hardcover doesn't know has no missing volumes")
}

func TestEnrichByISBN_FillsSeriesFromHardcover(t *testing.T) {
	hc := &fakeHCClient{ //nolint:exhaustruct // partial
		byISBN: &hardcover.ExternalBook{ //nolint:exhaustruct // partial
			Title: "Mort", Series: hcDiscworldRef(),
		},
	}
	svc := &BookService{logger: logging.NewNopLogger(), hardcover: hc} //nolint:exhaustruct,lll // partial

	got := svc.enrichByISBN(context.Background(), SourceProposal{ //nolint:exhaustruct,lll // partial
		Source: "manual", Title: "Mort", ISBN13: "9780552131063",
	})
	assert.Equal(t, "Discworld", got.SeriesName)
	assert.InDelta(t, 4.0, *got.SeriesPosition, 0)

	kept := svc.enrichByISBN(context.Background(), SourceProposal{ //nolint:exhaustruct,lll // partial
		Source: "manual", Title: "Mort", ISBN13: "9780552131063", SeriesName: "Death",
	})
	assert.Equal(t, "Death", kept.SeriesName, "a supplied series wins")
}

func TestGetExternal_HardcoverCarriesSeries(t *testing.T) {
	hc := &fakeHCClient{ //nolint:exhaustruct // partial
		byISBN: &hardcover.ExternalBook{ //nolint:exhaustruct // partial
			Title: "Mort", Series: hcDiscworldRef(),
		},
	}
	svc := &BookService{logger: logging.NewNopLogger(), hardcover: hc} //nolint:exhaustruct,lll // partial

	got, err := svc.GetExternal(context.Background(), "hardcover", "9780552131063")
	require.NoError(t, err)
	assert.Equal(t, "Discworld", got.SeriesName)
	assert.Equal(t, 41, *got.SeriesTotal)
}

func TestFillEmptySeries_OnlyFromExactISBN(t *testing.T) {
	isbn := "9780552131063"
	other := "9780000000002"
	series := SourceProposal{ //nolint:exhaustruct // partial
		Source: "hardcover", Title: "Mort", SeriesName: "Discworld",
	}
	fuzzy := series
	fuzzy.ISBN13 = other
	exact := series
	exact.ISBN13 = isbn

	for _, tc := range []struct {
		name    string
		book    models.Book
		p       SourceProposal
		wantSet bool
	}{
		{"exact ISBN", models.Book{ID: uuid.New(), ISBN13: &isbn}, exact, true},         //nolint:exhaustruct,lll // partial
		{"search match", models.Book{ID: uuid.New(), ISBN13: &isbn}, fuzzy, false},      //nolint:exhaustruct,lll // partial
		{"no library ISBN", models.Book{ID: uuid.New()}, exact, false},                  //nolint:exhaustruct,lll // partial
		{"no proposal ISBN", models.Book{ID: uuid.New(), ISBN13: &isbn}, series, false}, //nolint:exhaustruct,lll // partial
	} {
		t.Run(tc.name, func(t *testing.T) {
			repo := &fakeBooksResync{}              //nolint:exhaustruct // zero values fine
			svc := &BookService{resyncSource: repo} //nolint:exhaustruct // partial
			require.NoError(t, svc.fillEmptySeries(
				context.Background(), tc.book, []SourceProposal{tc.p},
			))
			_, set := repo.seriesSet[tc.book.ID]
			assert.Equal(t, tc.wantSet, set)
		})
	}
}

func TestMatchOwnedVolumes_TitleBeatsPosition(t *testing.T) {
	owned := []models.UserBook{
		ownedInSeries("A", nil),
		ownedInSeries("B", fptr(1)), // mistyped: B is volume 2
	}
	volumes := []hardcover.SeriesBook{
		{Position: 1, Book: hardcover.ExternalBook{Title: "A"}}, //nolint:exhaustruct,lll // partial
		{Position: 2, Book: hardcover.ExternalBook{Title: "B"}}, //nolint:exhaustruct,lll // partial
		{Position: 3, Book: hardcover.ExternalBook{Title: ""}},  //nolint:exhaustruct,lll // partial
	}
	assert.Equal(t, []int{0, 1, -1}, matchOwnedVolumes(owned, volumes))
}

func TestMatchOwnedVolumes_PositionPassSkipsMatchedAndMismatched(t *testing.T) {
	owned := []models.UserBook{
		ownedInSeries("A", nil),
		ownedInSeries("C", fptr(1)),
		ownedInSeries("D", fptr(5)),
	}
	volumes := []hardcover.SeriesBook{
		{Position: 1, Book: hardcover.ExternalBook{Title: "A"}}, //nolint:exhaustruct,lll // partial
		{Position: 2, Book: hardcover.ExternalBook{Title: "X"}}, //nolint:exhaustruct,lll // partial
	}
	assert.Equal(t, []int{0, -1}, matchOwnedVolumes(owned, volumes),
		"a title match keeps its volume; D's position 5 isn't volume 2")
}

func TestSeriesCache(t *testing.T) {
	var nilCache *seriesCache
	nilCache.put("x", nil)
	_, hit := nilCache.get("x")
	assert.False(t, hit)

	now := time.Unix(0, 0)
	c := newSeriesCache()
	c.now = func() time.Time { return now }
	s := &hardcover.Series{Name: "Discworld"} //nolint:exhaustruct // partial
	c.put("Discworld", s)
	c.put("Unknown", nil)

	got, hit := c.get("Discworld")
	assert.True(t, hit)
	assert.Same(t, s, got)
	got, hit = c.get("Unknown")
	assert.True(t, hit, "an unknown series is cached too")
	assert.Nil(t, got)

	now = now.Add(seriesCacheTTL)
	_, hit = c.get("Discworld")
	assert.True(t, hit, "still fresh at exactly the TTL")

	now = now.Add(time.Second)
	_, hit = c.get("Discworld")
	assert.False(t, hit, "expired")
}

func TestFetchHardcoverSeries_UsesCache(t *testing.T) {
	hc := &fakeHCClient{ //nolint:exhaustruct // partial
		series: &hardcover.Series{Name: "Discworld"}, //nolint:exhaustruct // partial
	}
	svc := &BookService{ //nolint:exhaustruct // partial
		logger: logging.NewNopLogger(), hardcover: hc, seriesCache: newSeriesCache(),
	}
	for range 2 {
		got, ok := svc.fetchHardcoverSeries(context.Background(), "Discworld")
		assert.True(t, ok)
		assert.Equal(t, "Discworld", got.Name)
	}
	assert.Equal(t, 1, hc.calls)

	_, ok := svc.fetchHardcoverSeries(context.Background(), "Unknown")
	assert.True(t, ok)
}
