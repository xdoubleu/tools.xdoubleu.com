//nolint:testpackage // testing the unexported translator constructor
package services

import (
	"bytes"
	"context"
	"errors"
	"io"
	"strings"
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
	"tools.xdoubleu.com/internal/logging"
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
// closes.
type countingStore struct {
	*objectstore.FakeClient
	gets atomic.Int32
	gate chan struct{}
}

func (s *countingStore) Get(ctx context.Context, key string) (io.ReadCloser, error) {
	s.gets.Add(1)
	if s.gate != nil {
		<-s.gate
	}
	return s.FakeClient.Get(ctx, key)
}

type translatorFixture struct {
	svc    *PositionService
	store  *countingStore
	files  *fakeKEPUBFiles
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
	svc := newPositionService(logging.NewNopLogger(), files, store, budget)
	return &translatorFixture{svc: svc, store: store, files: files, bookID: uuid.New()}
}

func chapterTwoSpan(value string) models.KoboLocation {
	return models.KoboLocation{
		Source: mocks.ChapterTwoHref, Type: koboSpanType, Value: value,
	}
}

func TestPositionService_TranslatesBothWaysAndCaches(t *testing.T) {
	f := newTranslatorFixture(t, nil, models.FileFormatEPUB, time.Minute)
	ctx := context.Background()

	pos := f.svc.KoboPosition(ctx, "u", f.bookID, chapterTwoSpan("kobo.1.2"))
	require.NotNil(t, pos)
	second := strings.Index(mocks.ChapterTwoText, "It continues.")
	assert.Equal(t, models.ReadingPosition{
		Href: mocks.ChapterTwoHref, Offset: second, Page: 0,
	}, *pos)

	locs, pending := f.svc.KoboLocations(ctx, "u", map[uuid.UUID]models.ReadingPosition{
		f.bookID: {Href: mocks.ChapterTwoHref, Offset: second + 4, Page: 0},
	})
	assert.Empty(t, pending)
	require.Contains(t, locs, f.bookID)
	assert.Equal(t, chapterTwoSpan("kobo.1.2"), *locs[f.bookID])
	assert.Equal(t, int32(2), f.store.gets.Load(), "one build, then cached")
}

func TestPositionService_NothingToTranslate(t *testing.T) {
	ctx := context.Background()

	f := newTranslatorFixture(t, nil, models.FileFormatEPUB, time.Minute)
	assert.Nil(t, f.svc.KoboPosition(ctx, "u", f.bookID, models.KoboLocation{
		Source: mocks.ChapterTwoHref, Type: "EpubCfi", Value: "kobo.1.2",
	}), "only KoboSpan bookmarks translate")
	locs, pending := f.svc.KoboLocations(ctx, "u", map[uuid.UUID]models.ReadingPosition{
		f.bookID: {Href: "", Offset: 0, Page: 4},
	})
	assert.Empty(t, locs, "a PDF page has no span")
	assert.Empty(t, pending)

	pdf := newTranslatorFixture(t, nil, models.FileFormatPDF, time.Minute)
	assert.Nil(t, pdf.svc.KoboPosition(ctx, "u", pdf.bookID, chapterTwoSpan("kobo.1.2")),
		"a PDF-sourced KEPUB has no original EPUB to point into")

	converting := newTranslatorFixture(t, nil, models.FileFormatEPUB, time.Minute)
	converting.files.byFormat[models.FileFormatKEPUB].Status = models.FileStatusConverting
	assert.Nil(t, converting.svc.KoboPosition(
		ctx, "u", converting.bookID, chapterTwoSpan("kobo.1.2")))

	orphan := newTranslatorFixture(t, nil, models.FileFormatEPUB, time.Minute)
	orphan.files.byFormat[models.FileFormatKEPUB].SourceFileID = nil
	assert.Nil(t, orphan.svc.KoboPosition(
		ctx, "u", orphan.bookID, chapterTwoSpan("kobo.1.2")))

	missingSrc := newTranslatorFixture(t, nil, models.FileFormatEPUB, time.Minute)
	missingSrc.files.byID = map[uuid.UUID]*models.BookFile{}
	assert.Nil(t, missingSrc.svc.KoboPosition(
		ctx, "u", missingSrc.bookID, chapterTwoSpan("kobo.1.2")))

	noKEPUB := newTranslatorFixture(t, nil, models.FileFormatEPUB, time.Minute)
	noKEPUB.files.byFormat = map[string]*models.BookFile{}
	assert.Nil(t, noKEPUB.svc.KoboPosition(
		ctx, "u", noKEPUB.bookID, chapterTwoSpan("kobo.1.2")))

	dbErr := newTranslatorFixture(t, nil, models.FileFormatEPUB, time.Minute)
	dbErr.files.err = errors.New("db down")
	assert.Nil(t, dbErr.svc.KoboPosition(
		ctx, "u", dbErr.bookID, chapterTwoSpan("kobo.1.2")))

	for _, fx := range []*translatorFixture{
		pdf, converting, orphan, missingSrc, noKEPUB, dbErr,
	} {
		assert.Zero(t, fx.store.gets.Load())
	}
}

func TestPositionService_BrokenKEPUBIsCachedAsEmpty(t *testing.T) {
	f := newTranslatorFixture(t, []byte("not a zip"), models.FileFormatEPUB, time.Minute)
	ctx := context.Background()

	assert.Nil(t, f.svc.KoboPosition(ctx, "u", f.bookID, chapterTwoSpan("kobo.1.2")))
	assert.Nil(t, f.svc.KoboPosition(ctx, "u", f.bookID, chapterTwoSpan("kobo.1.2")))
	assert.LessOrEqual(t, f.store.gets.Load(), int32(2), "not rebuilt per request")
}

func TestPositionService_MissingObjectIsCachedAsEmpty(t *testing.T) {
	f := newTranslatorFixture(t, nil, models.FileFormatEPUB, time.Minute)
	require.NoError(t, f.store.Delete(context.Background(), "kepub"))

	assert.Nil(t, f.svc.KoboPosition(
		context.Background(), "u", f.bookID, chapterTwoSpan("kobo.1.2")))
}

func TestPositionService_BudgetExceededReportsPendingThenTranslates(t *testing.T) {
	f := newTranslatorFixture(t, nil, models.FileFormatEPUB, 20*time.Millisecond)
	f.store.gate = make(chan struct{})
	ctx := context.Background()
	positions := map[uuid.UUID]models.ReadingPosition{
		f.bookID: {Href: mocks.ChapterTwoHref, Offset: 0, Page: 0},
	}

	locs, pending := f.svc.KoboLocations(ctx, "u", positions)
	assert.Empty(t, locs)
	assert.True(t, pending[f.bookID])
	assert.Nil(t, f.svc.KoboPosition(ctx, "u", f.bookID, chapterTwoSpan("kobo.1.1")),
		"a PUT past the budget stores no position")

	close(f.store.gate)
	require.Eventually(t, func() bool {
		locs, pending = f.svc.KoboLocations(ctx, "u", positions)
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

	_, pending := f.svc.KoboLocations(ctx, "u", map[uuid.UUID]models.ReadingPosition{
		f.bookID: {Href: mocks.ChapterTwoHref, Offset: 0, Page: 0},
	})
	assert.True(t, pending[f.bookID])
}
