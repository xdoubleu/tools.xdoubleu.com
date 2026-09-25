//nolint:testpackage // testing unexported service helpers
package services

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"tools.xdoubleu.com/apps/books/internal/models"
	"tools.xdoubleu.com/apps/books/pkg/objectstore"
	"tools.xdoubleu.com/apps/books/pkg/unicat"
	"tools.xdoubleu.com/internal/logging"
)

// TestFetchUniCatByISBN_MissFallsBackToSearch: an ISBN miss falls back to a
// title+author search.
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

// TestFetchUniCatByISBN_MissFallback_GuardsWrongTitle: the fallback is guarded.
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

// TestFetchUniCatByISBN_NoTitle_FallbackSkipsSearch: no title, no search.
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

// fakeUCClientSearchErr misses on GetByISBN and errors on Search.
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

// TestFetchUniCatByISBN_SearchFallback_Errors: a Search error is unresolved.
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

// TestFetchUniCatByISBN_GetByISBNErrors: a lookup error is unresolved, no fallback.
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

// TestFetchUniCatByISBN_SkipKnown: skip-if-known short-circuits.
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

// TestFetchByISBN_UniCatUnresolved_PropagatesToOutput: unresolved UniCat is
// reported, not dropped.
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
