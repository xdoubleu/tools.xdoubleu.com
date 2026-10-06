//nolint:testpackage // testing the unexported translator constructor
package services

import (
	"bytes"
	"context"
	"errors"
	"io"
	"log/slog"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"tools.xdoubleu.com/apps/books/internal/mocks"
	"tools.xdoubleu.com/apps/books/internal/models"
	"tools.xdoubleu.com/apps/books/pkg/objectstore"
	"tools.xdoubleu.com/internal/database"
)

type fakeKEPUBFiles struct {
	byFormat map[string]*models.BookFile
	byID     map[uuid.UUID]*models.BookFile
	err      error
}

func (f *fakeKEPUBFiles) GetByBookAndFormat(
	_ context.Context, _ string, _ uuid.UUID, format string,
) (*models.BookFile, error) {
	if f.err != nil {
		return nil, f.err
	}
	if bf, ok := f.byFormat[format]; ok {
		return bf, nil
	}
	return nil, database.ErrResourceNotFound
}

func (f *fakeKEPUBFiles) GetByID(
	_ context.Context, id uuid.UUID,
) (*models.BookFile, error) {
	if bf, ok := f.byID[id]; ok {
		return bf, nil
	}
	return nil, database.ErrResourceNotFound
}

// countingStore counts Gets and, while gate is set, blocks them until it
// closes; panics makes Get panic.
type countingStore struct {
	*objectstore.FakeClient
	gets   atomic.Int32
	gate   chan struct{}
	panics bool
}

func (s *countingStore) Get(ctx context.Context, key string) (io.ReadCloser, error) {
	s.gets.Add(1)
	if s.gate != nil {
		<-s.gate
	}
	if s.panics {
		panic("store exploded")
	}
	return s.FakeClient.Get(ctx, key)
}

// logRecorder is a slog.Handler keeping each record's message.
type logRecorder struct {
	mu   sync.Mutex
	msgs []string
}

func (r *logRecorder) Enabled(context.Context, slog.Level) bool { return true }

func (r *logRecorder) Handle(_ context.Context, rec slog.Record) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.msgs = append(r.msgs, rec.Message)
	return nil
}

func (r *logRecorder) WithAttrs([]slog.Attr) slog.Handler { return r }
func (r *logRecorder) WithGroup(string) slog.Handler      { return r }

func (r *logRecorder) messages() []string {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]string(nil), r.msgs...)
}

type translatorFixture struct {
	svc    *PositionService
	store  *countingStore
	files  *fakeKEPUBFiles
	logs   *logRecorder
	bookID uuid.UUID
}

func newTranslatorFixture(
	t *testing.T, kepubData []byte, sourceFormat string, budget time.Duration,
) *translatorFixture {
	t.Helper()
	orig := mocks.ChapterEPUB("Translate", "Author")
	if kepubData == nil {
		var err error
		kepubData, err = newKepubifyConverter().Convert(context.Background(), orig)
		require.NoError(t, err)
	}

	store := &countingStore{
		FakeClient: objectstore.NewFake(), gets: atomic.Int32{}, gate: nil,
		panics: false,
	}
	ctx := context.Background()
	require.NoError(t, store.Put(ctx, "src", bytes.NewReader(orig),
		int64(len(orig)), "application/epub+zip"))
	require.NoError(t, store.Put(ctx, "kepub", bytes.NewReader(kepubData),
		int64(len(kepubData)), "application/epub+zip"))

	src := &models.BookFile{ //nolint:exhaustruct //optional fields
		ID: uuid.New(), Format: sourceFormat, StorageKey: "src",
		Status: models.FileStatusReady,
	}
	kepub := &models.BookFile{ //nolint:exhaustruct //optional fields
		ID: uuid.New(), Format: models.FileFormatKEPUB, StorageKey: "kepub",
		Status: models.FileStatusReady, SourceFileID: &src.ID,
		ConverterVersion: currentKEPUBConverterVersion,
	}
	files := &fakeKEPUBFiles{
		byFormat: map[string]*models.BookFile{models.FileFormatKEPUB: kepub},
		byID:     map[uuid.UUID]*models.BookFile{src.ID: src},
		err:      nil,
	}
	logs := &logRecorder{mu: sync.Mutex{}, msgs: nil}
	svc := newPositionService(slog.New(logs), files, store, budget)
	return &translatorFixture{
		svc: svc, store: store, files: files, logs: logs, bookID: uuid.New(),
	}
}

// settledPosition is KoboPosition for a translation that isn't pending.
func settledPosition(
	t *testing.T,
	svc *PositionService,
	ctx context.Context, //nolint:revive // test helper takes t first
	bookID uuid.UUID,
	loc models.KoboLocation,
) *models.ReadingPosition {
	t.Helper()
	pos, pending := svc.KoboPosition(ctx, "u", bookID, loc)
	assert.False(t, pending)
	return pos
}

func chapterTwoSpan(value string) models.KoboLocation {
	return models.KoboLocation{
		Source: mocks.ChapterTwoHref, Type: koboSpanType, Value: value,
	}
}

func TestPositionService_TranslatesBothWaysAndCaches(t *testing.T) {
	f := newTranslatorFixture(t, nil, models.FileFormatEPUB, time.Minute)
	ctx := context.Background()

	pos := settledPosition(t, f.svc, ctx, f.bookID, chapterTwoSpan("kobo.1.2"))
	require.NotNil(t, pos)
	second := strings.Index(mocks.ChapterTwoText, "It continues.")
	assert.Equal(t, models.ReadingPosition{
		Href: mocks.ChapterTwoHref, Offset: second, Page: 0,
	}, *pos)

	locs, pending := f.svc.KoboLocations(
		ctx,
		"u",
		statesOf(map[uuid.UUID]models.ReadingPosition{
			f.bookID: {Href: mocks.ChapterTwoHref, Offset: second + 4, Page: 0},
		}),
	)
	assert.Empty(t, pending)
	require.Contains(t, locs, f.bookID)
	assert.Equal(t, chapterTwoSpan("kobo.1.2"), *locs[f.bookID])
	assert.Equal(t, int32(2), f.store.gets.Load(), "one build, then cached")
}

func TestPositionService_NothingToTranslate(t *testing.T) {
	ctx := context.Background()

	f := newTranslatorFixture(t, nil, models.FileFormatEPUB, time.Minute)
	assert.Nil(t, settledPosition(t, f.svc, ctx, f.bookID, models.KoboLocation{
		Source: mocks.ChapterTwoHref, Type: "EpubCfi", Value: "kobo.1.2",
	}), "only KoboSpan bookmarks translate")
	locs, pending := f.svc.KoboLocations(
		ctx,
		"u",
		statesOf(map[uuid.UUID]models.ReadingPosition{
			f.bookID: {Href: "", Offset: 0, Page: 4},
		}),
	)
	assert.Empty(t, locs, "a PDF page has no span")
	assert.Empty(t, pending)

	pdf := newTranslatorFixture(t, nil, models.FileFormatPDF, time.Minute)
	assert.Nil(t, settledPosition(t, pdf.svc, ctx, pdf.bookID, chapterTwoSpan("kobo.1.2")),
		"a PDF-sourced KEPUB without page anchors has no page")

	converting := newTranslatorFixture(t, nil, models.FileFormatEPUB, time.Minute)
	converting.files.byFormat[models.FileFormatKEPUB].Status = models.FileStatusConverting
	assert.Nil(t, settledPosition(t, converting.svc,
		ctx, converting.bookID, chapterTwoSpan("kobo.1.2")))

	orphan := newTranslatorFixture(t, nil, models.FileFormatEPUB, time.Minute)
	orphan.files.byFormat[models.FileFormatKEPUB].SourceFileID = nil
	assert.Nil(t, settledPosition(t, orphan.svc,
		ctx, orphan.bookID, chapterTwoSpan("kobo.1.2")))

	missingSrc := newTranslatorFixture(t, nil, models.FileFormatEPUB, time.Minute)
	missingSrc.files.byID = map[uuid.UUID]*models.BookFile{}
	assert.Nil(t, settledPosition(t, missingSrc.svc,
		ctx, missingSrc.bookID, chapterTwoSpan("kobo.1.2")))

	noKEPUB := newTranslatorFixture(t, nil, models.FileFormatEPUB, time.Minute)
	noKEPUB.files.byFormat = map[string]*models.BookFile{}
	assert.Nil(t, settledPosition(t, noKEPUB.svc,
		ctx, noKEPUB.bookID, chapterTwoSpan("kobo.1.2")))

	dbErr := newTranslatorFixture(t, nil, models.FileFormatEPUB, time.Minute)
	dbErr.files.err = errors.New("db down")
	assert.Nil(t, settledPosition(t, dbErr.svc,
		ctx, dbErr.bookID, chapterTwoSpan("kobo.1.2")))

	assert.Equal(t, int32(1), pdf.store.gets.Load(), "only the KEPUB")
	for _, fx := range []*translatorFixture{
		converting, orphan, missingSrc, noKEPUB, dbErr,
	} {
		assert.Zero(t, fx.store.gets.Load())
	}
}

func TestPositionService_BrokenKEPUBIsCachedAsEmpty(t *testing.T) {
	f := newTranslatorFixture(t, []byte("not a zip"), models.FileFormatEPUB, time.Minute)
	ctx := context.Background()

	assert.Nil(t, settledPosition(t, f.svc, ctx, f.bookID, chapterTwoSpan("kobo.1.2")))
	assert.Nil(t, settledPosition(t, f.svc, ctx, f.bookID, chapterTwoSpan("kobo.1.2")))
	assert.LessOrEqual(t, f.store.gets.Load(), int32(2), "not rebuilt per request")
}

func TestPositionService_DownloadFailureIsRetried(t *testing.T) {
	f := newTranslatorFixture(t, nil, models.FileFormatEPUB, time.Minute)
	ctx := context.Background()
	positions := map[uuid.UUID]models.ReadingPosition{
		f.bookID: {Href: mocks.ChapterTwoHref, Offset: 0, Page: 0},
	}
	kepub, _ := f.store.GetContent("kepub")
	require.NoError(t, f.store.Delete(ctx, "kepub"))

	locs, pending := f.svc.KoboLocations(ctx, "u", statesOf(positions))
	assert.Empty(t, locs)
	assert.True(t, pending[f.bookID], "a failed download is retried, not cached")

	f.store.PutAt("kepub", kepub, time.Now())
	locs, pending = f.svc.KoboLocations(ctx, "u", statesOf(positions))
	assert.Empty(t, pending)
	assert.Equal(t, chapterTwoSpan("kobo.1.1"), *locs[f.bookID])
}

func TestPositionService_NoFreeBuildSlotIsPending(t *testing.T) {
	f := newTranslatorFixture(t, nil, models.FileFormatEPUB, time.Minute)
	f.svc.buildTimeout = 10 * time.Millisecond
	for range spanMapBuildSlots {
		f.svc.slots <- struct{}{}
	}

	_, pending := f.svc.KoboLocations(context.Background(), "u",
		statesOf(map[uuid.UUID]models.ReadingPosition{
			f.bookID: {Href: mocks.ChapterTwoHref, Offset: 0, Page: 0},
		}))
	assert.True(t, pending[f.bookID])
	assert.Zero(t, f.store.gets.Load())
}

func TestPositionService_BudgetExceededReportsPendingThenTranslates(t *testing.T) {
	f := newTranslatorFixture(t, nil, models.FileFormatEPUB, 20*time.Millisecond)
	f.store.gate = make(chan struct{})
	ctx := context.Background()
	positions := map[uuid.UUID]models.ReadingPosition{
		f.bookID: {Href: mocks.ChapterTwoHref, Offset: 0, Page: 0},
	}

	locs, pending := f.svc.KoboLocations(ctx, "u", statesOf(positions))
	assert.Empty(t, locs)
	assert.True(t, pending[f.bookID])
	pos, pendingPos := f.svc.KoboPosition(ctx, "u", f.bookID, chapterTwoSpan("kobo.1.1"))
	assert.Nil(t, pos, "a PUT past the budget stores no position")
	assert.True(t, pendingPos, "and is backfilled once the map is built")

	close(f.store.gate)
	require.Eventually(t, func() bool {
		locs, pending = f.svc.KoboLocations(ctx, "u", statesOf(positions))
		return len(pending) == 0 && locs[f.bookID] != nil
	}, 5*time.Second, 10*time.Millisecond)
	assert.Equal(t, chapterTwoSpan("kobo.1.1"), *locs[f.bookID])
	assert.Equal(t, int32(2), f.store.gets.Load(), "concurrent requests share a build")
}

func TestPositionService_CanceledRequestIsPending(t *testing.T) {
	f := newTranslatorFixture(t, nil, models.FileFormatEPUB, time.Minute)
	f.store.gate = make(chan struct{})
	t.Cleanup(func() { close(f.store.gate) })
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, pending := f.svc.KoboLocations(
		ctx,
		"u",
		statesOf(map[uuid.UUID]models.ReadingPosition{
			f.bookID: {Href: mocks.ChapterTwoHref, Offset: 0, Page: 0},
		}),
	)
	assert.True(t, pending[f.bookID])
}

func TestPositionService_PendingEndsAfterGrace(t *testing.T) {
	f := newTranslatorFixture(t, nil, models.FileFormatEPUB, 10*time.Millisecond)
	f.store.gate = make(chan struct{})
	t.Cleanup(func() { close(f.store.gate) })
	f.svc.pendingGrace = 0
	positions := map[uuid.UUID]models.ReadingPosition{
		f.bookID: {Href: mocks.ChapterTwoHref, Offset: 0, Page: 0},
	}

	_, pending := f.svc.KoboLocations(context.Background(), "u", statesOf(positions))
	assert.True(t, pending[f.bookID], "first seen pending")
	locs, pending := f.svc.KoboLocations(context.Background(), "u", statesOf(positions))
	assert.Empty(t, pending, "past the grace the book goes out as percent only")
	assert.Empty(t, locs)
}

func TestPositionService_BackfillKoboPosition(t *testing.T) {
	f := newTranslatorFixture(t, nil, models.FileFormatEPUB, 10*time.Millisecond)
	ctx := context.Background()
	var stored []models.ReadingPosition
	store := func(_ context.Context, pos models.ReadingPosition) error {
		stored = append(stored, pos)
		return errors.New("logged, not returned")
	}

	f.svc.BackfillKoboPosition(ctx, "u", f.bookID, chapterTwoSpan("kobo.1.2"), store)
	second := strings.Index(mocks.ChapterTwoText, "It continues.")
	assert.Equal(t, []models.ReadingPosition{
		{Href: mocks.ChapterTwoHref, Offset: second, Page: 0},
	}, stored)

	f.svc.BackfillKoboPosition(ctx, "u", f.bookID, chapterTwoSpan("kobo.9.9"), store)
	f.files.byFormat = map[string]*models.BookFile{}
	f.svc.BackfillKoboPosition(ctx, "u", f.bookID, chapterTwoSpan("kobo.1.2"), store)
	assert.Len(t, stored, 1, "unknown spans and books without a KEPUB store nothing")
}

// statesOf wraps positions as reading states with no Kobo bookmark.
func statesOf(
	positions map[uuid.UUID]models.ReadingPosition,
) map[uuid.UUID]*models.BookReadingState {
	states := make(map[uuid.UUID]*models.BookReadingState, len(positions))
	for id, pos := range positions {
		//nolint:exhaustruct // only the position matters
		states[id] = &models.BookReadingState{
			Position: &pos,
		}
	}
	return states
}
