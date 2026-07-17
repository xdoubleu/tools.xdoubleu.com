//nolint:testpackage // testing unexported service helpers
package services

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/xdoubleu/essentia/v4/pkg/logging"

	"tools.xdoubleu.com/apps/reading/internal/models"
	"tools.xdoubleu.com/apps/reading/pkg/objectstore"
	"tools.xdoubleu.com/apps/reading/pkg/unicat"
)

// TestFetchUniCatByISBN_MissFallsBackToSearch is a regression test for a book
// present in UniCat under a title/author search but not found by ISBN: UniCat's
// 020$a index is populated from the physical item catalogued, which can miss
// editions the union catalog otherwise has under a different or no ISBN. On an
// ISBN miss, fetchUniCatByISBN must fall back to a title+author search rather
// than giving up.
func TestFetchUniCatByISBN_MissFallsBackToSearch(t *testing.T) {
	isbn := "9789463107389"
	book := models.Book{ //nolint:exhaustruct // partial
		Title: "10 franke vragen aan Frank", Authors: []string{"Frank Vandenbroucke"},
		ISBN13: &isbn,
	}
	uc := &fakeUCClient{ //nolint:exhaustruct // partial: byISBN nil -> ErrNotFound
		searchResults: []unicat.ExternalBook{
			{ //nolint:exhaustruct // partial
				Title:   "10 franke vragen aan Frank",
				Authors: []string{"Frank Vandenbroucke"},
			},
		},
	}
	svc := &BookService{ //nolint:exhaustruct // partial
		logger:      logging.NewNopLogger(),
		uniCat:      uc,
		objectStore: objectstore.NewFake(),
	}

	p, unresolved := svc.fetchUniCatByISBN(
		context.Background(), logging.NewNopLogger(), book, nil,
	)
	require.NotNil(t, p, "the search fallback must surface a proposal")
	assert.False(t, unresolved)
	assert.Equal(t, "unicat", p.Source)
	assert.Equal(t, "10 franke vragen aan Frank", p.Title)
}

// TestFetchUniCatByISBN_MissFallback_GuardsWrongTitle verifies the search
// fallback is guarded like every other title search: a result that doesn't
// match the book's title/author must not be proposed, even though the ISBN
// lookup missed.
func TestFetchUniCatByISBN_MissFallback_GuardsWrongTitle(t *testing.T) {
	isbn := "9789463107389"
	book := models.Book{ //nolint:exhaustruct // partial
		Title: "10 franke vragen aan Frank", Authors: []string{"Frank Vandenbroucke"},
		ISBN13: &isbn,
	}
	uc := &fakeUCClient{ //nolint:exhaustruct // partial
		searchResults: []unicat.ExternalBook{
			{ //nolint:exhaustruct // partial
				Title:   "An Unrelated Book",
				Authors: []string{"Someone Else"},
			},
		},
	}
	svc := &BookService{ //nolint:exhaustruct // partial
		logger:      logging.NewNopLogger(),
		uniCat:      uc,
		objectStore: objectstore.NewFake(),
	}

	p, unresolved := svc.fetchUniCatByISBN(
		context.Background(), logging.NewNopLogger(), book, nil,
	)
	assert.Nil(t, p, "a title/author mismatch must not be proposed")
	assert.False(t, unresolved, "a clean guarded miss is resolved, not unresolved")
}

// TestFetchUniCatByISBN_NoTitle_FallbackSkipsSearch verifies the fallback
// doesn't call Search for a book with no title to search by.
func TestFetchUniCatByISBN_NoTitle_FallbackSkipsSearch(t *testing.T) {
	isbn := "9789463107389"
	book := models.Book{ISBN13: &isbn} //nolint:exhaustruct // partial: no title
	uc := &fakeUCClient{}              //nolint:exhaustruct // byISBN nil -> ErrNotFound
	svc := &BookService{               //nolint:exhaustruct // partial
		logger:      logging.NewNopLogger(),
		uniCat:      uc,
		objectStore: objectstore.NewFake(),
	}

	p, unresolved := svc.fetchUniCatByISBN(
		context.Background(), logging.NewNopLogger(), book, nil,
	)
	assert.Nil(t, p)
	assert.False(t, unresolved)
}

// fakeUCClientSearchErr returns ErrNotFound from GetByISBN (so
// fetchUniCatByISBN reaches the search fallback) but errors from Search,
// unlike fakeUCClient whose single err field drives both methods identically.
type fakeUCClientSearchErr struct{}

func (fakeUCClientSearchErr) GetByISBN(
	_ context.Context,
	_ string,
) (*unicat.ExternalBook, error) {
	return nil, unicat.ErrNotFound
}

func (fakeUCClientSearchErr) Search(
	_ context.Context,
	_ string,
) ([]unicat.ExternalBook, error) {
	return nil, assert.AnError
}

// TestFetchUniCatByISBN_SearchFallback_Errors verifies a Search error surfaces
// as unresolved rather than a silent miss.
func TestFetchUniCatByISBN_SearchFallback_Errors(t *testing.T) {
	isbn := "9789463107389"
	book := models.Book{ //nolint:exhaustruct // partial
		Title: "10 franke vragen aan Frank", ISBN13: &isbn,
	}
	svc := &BookService{ //nolint:exhaustruct // partial
		logger:      logging.NewNopLogger(),
		uniCat:      fakeUCClientSearchErr{},
		objectStore: objectstore.NewFake(),
	}

	p, unresolved := svc.fetchUniCatByISBN(
		context.Background(), logging.NewNopLogger(), book, nil,
	)
	assert.Nil(t, p)
	assert.True(t, unresolved)
}

// TestFetchUniCatByISBN_GetByISBNErrors verifies a non-ErrNotFound error from
// the ISBN lookup itself surfaces as unresolved without reaching the fallback.
func TestFetchUniCatByISBN_GetByISBNErrors(t *testing.T) {
	isbn := "9789463107389"
	book := models.Book{ISBN13: &isbn}       //nolint:exhaustruct // partial
	uc := &fakeUCClient{err: assert.AnError} //nolint:exhaustruct // partial
	svc := &BookService{                     //nolint:exhaustruct // partial
		logger:      logging.NewNopLogger(),
		uniCat:      uc,
		objectStore: objectstore.NewFake(),
	}

	p, unresolved := svc.fetchUniCatByISBN(
		context.Background(), logging.NewNopLogger(), book, nil,
	)
	assert.Nil(t, p)
	assert.True(t, unresolved)
}

// TestFetchUniCatByISBN_SkipKnown verifies opts' skip-if-known cache short-
// circuits before any provider call, mirroring the other sources' gating.
func TestFetchUniCatByISBN_SkipKnown(t *testing.T) {
	isbn := "9789463107389"
	book := models.Book{ISBN13: &isbn} //nolint:exhaustruct // partial
	svc := &BookService{               //nolint:exhaustruct // partial
		logger:      logging.NewNopLogger(),
		uniCat:      &fakeUCClient{}, //nolint:exhaustruct // partial
		objectStore: objectstore.NewFake(),
	}
	opts := &scanOptions{
		known: map[string]bool{"unicat": true},
	}

	p, unresolved := svc.fetchUniCatByISBN(
		context.Background(), logging.NewNopLogger(), book, opts,
	)
	assert.Nil(t, p)
	assert.True(t, unresolved)
}

// TestFetchByISBN_UniCatUnresolved_PropagatesToOutput verifies fetchByISBN's
// UniCat dispatch surfaces an unresolved source (skip-known here) rather than
// silently dropping it, matching Hardcover's dispatch.
func TestFetchByISBN_UniCatUnresolved_PropagatesToOutput(t *testing.T) {
	isbn := "9789463107389"
	svc := &BookService{ //nolint:exhaustruct // partial
		logger:      logging.NewNopLogger(),
		uniCat:      &fakeUCClient{}, //nolint:exhaustruct // partial
		objectStore: objectstore.NewFake(),
	}
	opts := &scanOptions{
		known: map[string]bool{"unicat": true},
	}

	proposals, unresolved := svc.fetchByISBN(
		context.Background(), logging.NewNopLogger(),
		models.Book{ISBN13: &isbn}, opts, //nolint:exhaustruct // partial
	)
	assert.Empty(t, proposals)
	assert.True(t, unresolved["unicat"])
}
