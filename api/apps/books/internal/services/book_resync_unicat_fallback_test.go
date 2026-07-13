//nolint:testpackage // testing unexported service helpers
package services

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/xdoubleu/essentia/v4/pkg/logging"

	"tools.xdoubleu.com/apps/books/internal/models"
	"tools.xdoubleu.com/apps/books/pkg/objectstore"
	"tools.xdoubleu.com/apps/books/pkg/unicat"
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
