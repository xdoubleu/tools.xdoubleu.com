//nolint:testpackage // testing unexported service helpers
package services

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/mmcdole/gofeed"
	"github.com/stretchr/testify/assert"
	"github.com/xdoubleu/essentia/v4/pkg/logging"

	"tools.xdoubleu.com/apps/reading/internal/models"
)

// fakeGUIDLookup is a test stub for feedsGUIDLookup.
type fakeGUIDLookup struct {
	bookIDs map[string]uuid.UUID
	err     error
}

func (f *fakeGUIDLookup) GetBookIDsByGUIDs(
	_ context.Context,
	_ uuid.UUID,
	_ []string,
) (map[string]uuid.UUID, error) {
	return f.bookIDs, f.err
}

// fakeAddedAtSetter is a test stub for addedAtSetter.
type fakeAddedAtSetter struct {
	err   error
	calls int
}

func (f *fakeAddedAtSetter) SetAddedAt(
	_ context.Context,
	_ string,
	_ uuid.UUID,
	_ time.Time,
) error {
	f.calls++
	return f.err
}

func newResyncTestFeed() models.Feed {
	//nolint:exhaustruct // only the fields resyncAddedAt reads
	return models.Feed{UserID: "user1", ID: uuid.New()}
}

func newResyncTestItem(published *time.Time) *gofeed.Item {
	//nolint:exhaustruct // only PublishedParsed is read
	return &gofeed.Item{PublishedParsed: published}
}

// TestResyncAddedAt_LookupError proves a GetBookIDsByGUIDs failure is logged
// and swallowed — a resync failure must never fail the poll/refresh itself.
func TestResyncAddedAt_LookupError(t *testing.T) {
	setter := &fakeAddedAtSetter{err: nil, calls: 0}
	svc := &FeedService{ //nolint:exhaustruct // only fields under test
		logger:        logging.NewNopLogger(),
		guidLookup:    &fakeGUIDLookup{err: errors.New("db down"), bookIDs: nil},
		addedAtSetter: setter,
	}

	published := time.Now()
	svc.resyncAddedAt(
		context.Background(), newResyncTestFeed(),
		[]string{"g1"}, nil,
		map[string]*gofeed.Item{"g1": newResyncTestItem(&published)},
	)

	assert.Equal(t, 0, setter.calls, "a lookup failure must skip every update")
}

// TestResyncAddedAt_SkipsUnparsedDate proves an existing item whose pubDate
// never parsed is left alone instead of being overwritten with a zero time.
func TestResyncAddedAt_SkipsUnparsedDate(t *testing.T) {
	bookID := uuid.New()
	setter := &fakeAddedAtSetter{err: nil, calls: 0}
	svc := &FeedService{ //nolint:exhaustruct // only fields under test
		logger: logging.NewNopLogger(),
		guidLookup: &fakeGUIDLookup{
			bookIDs: map[string]uuid.UUID{"g1": bookID},
			err:     nil,
		},
		addedAtSetter: setter,
	}

	svc.resyncAddedAt(
		context.Background(), newResyncTestFeed(),
		[]string{"g1"}, nil,
		map[string]*gofeed.Item{"g1": newResyncTestItem(nil)},
	)

	assert.Equal(t, 0, setter.calls, "an item with no parsed date must be skipped")
}

// TestResyncAddedAt_SetError proves a SetAddedAt failure for one item is
// logged and does not stop the others or the caller.
func TestResyncAddedAt_SetError(t *testing.T) {
	bookID := uuid.New()
	setter := &fakeAddedAtSetter{err: errors.New("write failed"), calls: 0}
	svc := &FeedService{ //nolint:exhaustruct // only fields under test
		logger: logging.NewNopLogger(),
		guidLookup: &fakeGUIDLookup{
			bookIDs: map[string]uuid.UUID{"g1": bookID},
			err:     nil,
		},
		addedAtSetter: setter,
	}

	published := time.Now()
	svc.resyncAddedAt(
		context.Background(), newResyncTestFeed(),
		[]string{"g1"}, nil,
		map[string]*gofeed.Item{"g1": newResyncTestItem(&published)},
	)

	assert.Equal(t, 1, setter.calls)
}
