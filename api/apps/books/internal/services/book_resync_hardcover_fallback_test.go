//nolint:testpackage // testing unexported service helpers
package services

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/xdoubleu/essentia/v4/pkg/logging"

	"tools.xdoubleu.com/apps/books/internal/models"
	"tools.xdoubleu.com/apps/books/pkg/hardcover"
	"tools.xdoubleu.com/apps/books/pkg/objectstore"
)

// TestFetchHardcoverByISBN_MissFallsBackToSearch is a regression test for a
// book found by "Search with these terms" but not by resync: Hardcover's
// editions table often lacks the exact ISBN-13 (niche/non-US editions), even
// though its Typesense work index has the book by title. On an ISBN miss,
// fetchHardcoverByISBN must fall back to a title+author search rather than
// giving up.
func TestFetchHardcoverByISBN_MissFallsBackToSearch(t *testing.T) {
	isbn := "9780000000000"
	book := models.Book{ //nolint:exhaustruct // partial
		Title: "Androids", Authors: []string{"Chet Haase"}, ISBN13: &isbn,
	}
	hc := &fakeHCClient{ //nolint:exhaustruct // partial: byISBN nil -> ErrNotFound
		searchResults: []hardcover.ExternalBook{
			{ //nolint:exhaustruct // partial
				Title:   "Androids",
				Authors: []string{"Chet Haase"},
			},
		},
	}
	svc := &BookService{ //nolint:exhaustruct // partial
		logger:      logging.NewNopLogger(),
		hardcover:   hc,
		objectStore: objectstore.NewFake(),
	}

	p, unresolved := svc.fetchHardcoverByISBN(
		context.Background(), logging.NewNopLogger(), book, nil,
	)
	require.NotNil(t, p, "the search fallback must surface a proposal")
	assert.False(t, unresolved)
	assert.Equal(t, "hardcover", p.Source)
	assert.Equal(t, "Androids", p.Title)
}

// TestFetchHardcoverByISBN_MissFallback_GuardsWrongTitle verifies the search
// fallback is guarded like every other title search: a result that doesn't
// match the book's title/author must not be proposed, even though the ISBN
// lookup missed.
func TestFetchHardcoverByISBN_MissFallback_GuardsWrongTitle(t *testing.T) {
	isbn := "9780000000000"
	book := models.Book{ //nolint:exhaustruct // partial
		Title: "Androids", Authors: []string{"Chet Haase"}, ISBN13: &isbn,
	}
	hc := &fakeHCClient{ //nolint:exhaustruct // partial
		searchResults: []hardcover.ExternalBook{
			{ //nolint:exhaustruct // partial
				Title:   "An Unrelated Book",
				Authors: []string{"Someone Else"},
			},
		},
	}
	svc := &BookService{ //nolint:exhaustruct // partial
		logger:      logging.NewNopLogger(),
		hardcover:   hc,
		objectStore: objectstore.NewFake(),
	}

	p, unresolved := svc.fetchHardcoverByISBN(
		context.Background(), logging.NewNopLogger(), book, nil,
	)
	assert.Nil(t, p, "a title/author mismatch must not be proposed")
	assert.False(t, unresolved, "a clean guarded miss is resolved, not unresolved")
}
