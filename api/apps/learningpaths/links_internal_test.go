//nolint:exhaustruct // sparse fixtures
package learningpaths

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"tools.xdoubleu.com/apps/books"
	"tools.xdoubleu.com/apps/feeds"
	"tools.xdoubleu.com/apps/learningpaths/internal/services"
)

type stubBooks struct {
	book      *books.LibraryBook
	ensureErr error
	err       error
}

func (s stubBooks) GetLibraryBookByID(
	context.Context, string, uuid.UUID,
) (*books.LibraryBook, error) {
	return s.book, s.err
}

func (s stubBooks) EnsureLibraryBook(
	context.Context, string, string, string, string,
) (uuid.UUID, error) {
	return uuid.Nil, s.ensureErr
}

type stubFeeds struct {
	item *feeds.SharedItem
	err  error
}

func (s stubFeeds) GetItemByID(
	context.Context, string, uuid.UUID,
) (*feeds.SharedItem, error) {
	return s.item, s.err
}

func TestBookPort_MapsLibraryBook(t *testing.T) {
	p := bookPort{src: stubBooks{book: &books.LibraryBook{
		Title: "T", Status: "Reading", ProgressPercent: 40, CoverURL: "u",
	}}}

	got, err := p.GetLibraryBook(t.Context(), "u", uuid.New())
	require.NoError(t, err)
	assert.Equal(t, "T", got.Title)
	assert.Equal(t, "Reading", got.Status)
	assert.Equal(t, 40, got.ProgressPercent)
	assert.Equal(t, "u", got.CoverURL)
}

func TestBookPort_PropagatesError(t *testing.T) {
	boom := errors.New("boom")
	p := bookPort{src: stubBooks{err: boom}}
	_, err := p.GetLibraryBook(t.Context(), "u", uuid.New())
	assert.ErrorIs(t, err, boom)
}

func TestBookPort_EnsureTranslatesExternalNotFound(t *testing.T) {
	p := bookPort{src: stubBooks{ensureErr: books.ErrExternalBookNotFound}}
	_, err := p.EnsureLibraryBook(t.Context(), "u", "p", "id", "shelf")
	assert.ErrorIs(t, err, services.ErrExternalBookNotFound)

	boom := errors.New("boom")
	p = bookPort{src: stubBooks{ensureErr: boom}}
	_, err = p.EnsureLibraryBook(t.Context(), "u", "p", "id", "shelf")
	assert.ErrorIs(t, err, boom)
}

func TestFeedPort_MapsItem(t *testing.T) {
	p := feedPort{src: stubFeeds{item: &feeds.SharedItem{
		Title: "F", SourceURL: "s", Read: true, Bookmarked: true,
	}}}
	got, err := p.GetFeedItem(t.Context(), "u", uuid.New())
	require.NoError(t, err)
	assert.Equal(t, "F", got.Title)
	assert.Equal(t, "s", got.SourceURL)
	assert.True(t, got.Read)
	assert.True(t, got.Bookmarked)
}

func TestFeedPort_PropagatesError(t *testing.T) {
	boom := errors.New("boom")
	_, err := feedPort{src: stubFeeds{err: boom}}.GetFeedItem(
		t.Context(), "u", uuid.New(),
	)
	assert.ErrorIs(t, err, boom)
}
