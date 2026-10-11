//nolint:testpackage // testing the unexported physical service constructor
package services

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
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

type fakePhysicalStore struct {
	mu      sync.Mutex
	copy    *models.PhysicalCopy
	err     error
	updated *models.PhysicalCopy
	pages   chan int
	states  []models.BookReadingState
}

func (f *fakePhysicalStore) GetPhysicalCopy(
	context.Context, string, uuid.UUID,
) (*models.PhysicalCopy, error) {
	if f.err != nil {
		return nil, f.err
	}
	c := *f.copy
	return &c, nil
}

func (f *fakePhysicalStore) UpdatePhysicalCopy(
	_ context.Context, _ string, _ uuid.UUID, c models.PhysicalCopy,
) error {
	f.updated = &c
	return nil
}

func (f *fakePhysicalStore) SetCurrentPageIfLatest(
	_ context.Context, _ string, _ uuid.UUID, page int, _ time.Time,
) error {
	f.pages <- page
	return nil
}

func (f *fakePhysicalStore) UpsertIfNewer(
	_ context.Context, state models.BookReadingState,
) (*time.Time, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.states = append(f.states, state)
	now := time.Now()
	return &now, nil
}

func (f *fakePhysicalStore) written() []models.BookReadingState {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]models.BookReadingState(nil), f.states...)
}

type physicalFixture struct {
	svc   *PhysicalService
	store *fakePhysicalStore
	blobs *countingStore
	files *fakeKEPUBFiles
}

func newPhysicalFixture(
	t *testing.T,
	epub []byte,
	budget time.Duration,
) *physicalFixture {
	t.Helper()
	blobs := &countingStore{
		FakeClient: objectstore.NewFake(), gets: atomic.Int32{}, gate: nil,
		panics: false,
	}
	require.NoError(t, blobs.Put(context.Background(), "epub", bytes.NewReader(epub),
		int64(len(epub)), "application/epub+zip"))
	files := &fakeKEPUBFiles{
		byFormat: map[string]*models.BookFile{
			models.FileFormatEPUB: {
				ID: uuid.New(), Format: models.FileFormatEPUB, StorageKey: "epub",
				Status: models.FileStatusReady,
			},
		},
		byID: map[uuid.UUID]*models.BookFile{},
		err:  nil,
	}
	store := &fakePhysicalStore{
		mu: sync.Mutex{},
		copy: &models.PhysicalCopy{
			PageCount: 99, TextStartPage: 1, Anchors: nil, Syncable: true,
		},
		err: nil, updated: nil, pages: make(chan int, 1), states: nil,
	}
	logger := slog.New(&logRecorder{mu: sync.Mutex{}, msgs: nil})
	positions := newPositionService(logger, files, blobs, budget)
	return &physicalFixture{
		svc:   newPhysicalService(logger, store, store, positions),
		store: store, blobs: blobs, files: files,
	}
}

func TestPhysicalService_SyncPageWritesPhysicalState(t *testing.T) {
	f := newPhysicalFixture(t, mocks.PhysicalEPUB("x", false), time.Minute)
	readAt := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)

	require.NoError(t, f.svc.SyncPage(context.Background(), "u", uuid.New(), 1, readAt))
	states := f.store.written()
	require.Len(t, states, 1)
	st := states[0]
	assert.Equal(t, models.ReadingSourcePhysical, st.Source)
	assert.Equal(t, &readAt, st.ReadAt)
	assert.Equal(t, &models.ReadingPosition{
		Href: mocks.PhysicalCh1Href, Offset: 0, Page: 0,
	}, st.Position)
	assert.Nil(t, st.KoboLocation)

	require.NoError(t, f.svc.SyncPage(context.Background(), "u", uuid.New(), 50, readAt))
	assert.InDelta(t, 50, f.store.written()[1].Percent, 2)

	require.NoError(t, f.svc.SyncPage(context.Background(), "u", uuid.New(), 0, readAt))
	f.store.copy.Syncable = false
	require.NoError(t, f.svc.SyncPage(context.Background(), "u", uuid.New(), 5, readAt))
	f.store.copy.Syncable, f.store.copy.PageCount = true, 0
	require.NoError(t, f.svc.SyncPage(context.Background(), "u", uuid.New(), 5, readAt))
	assert.Len(t, f.store.written(), 2, "page 0, not owned physically, no page count")

	f.store.err = database.ErrResourceNotFound
	require.NoError(t, f.svc.SyncPage(context.Background(), "u", uuid.New(), 5, readAt))
	f.store.err = errors.New("db down")
	require.Error(t, f.svc.SyncPage(context.Background(), "u", uuid.New(), 5, readAt))
}

func TestPhysicalService_SyncPageFinishesInBackground(t *testing.T) {
	f := newPhysicalFixture(t, mocks.PhysicalEPUB("x", false), 10*time.Millisecond)
	f.blobs.gate = make(chan struct{})

	require.NoError(
		t,
		f.svc.SyncPage(context.Background(), "u", uuid.New(), 3, time.Now()),
	)
	assert.Empty(t, f.store.written(), "the EPUB is still downloading")
	close(f.blobs.gate)
	require.Eventually(t, func() bool { return len(f.store.written()) == 1 },
		5*time.Second, 10*time.Millisecond)
}

func TestPhysicalService_FollowDigitalSetsCurrentPage(t *testing.T) {
	f := newPhysicalFixture(t, mocks.PhysicalEPUB("x", true), time.Minute)
	// Chapter two's page 4 starts halfway through, after its epigraph and title.
	half := len(
		"Epigraph.Two",
	) + len(
		mocks.PhysicalSentence(1, 1),
	)*mocks.PhysicalSentences/2
	state := models.BookReadingState{ //nolint:exhaustruct //only what's read
		UserID: "u", BookID: uuid.New(), Source: models.ReadingSourceKobo,
		Position: &models.ReadingPosition{Href: mocks.PhysicalCh2Href, Offset: half, Page: 0},
	}

	f.svc.FollowDigital(context.Background(), state, time.Now())
	select {
	case page := <-f.store.pages:
		assert.Equal(t, 4, page)
	case <-time.After(5 * time.Second):
		t.Fatal("current_page wasn't set")
	}

	//nolint:exhaustruct // only what FollowDigital reads
	skips := []models.BookReadingState{
		{Source: models.ReadingSourcePhysical, Position: state.Position},
		{Source: models.ReadingSourceWeb, Position: nil},
		{
			Source:   models.ReadingSourceWeb,
			Position: &models.ReadingPosition{Href: "", Offset: 0, Page: 3},
		},
	}
	for _, skip := range skips {
		f.svc.FollowDigital(context.Background(), skip, time.Now())
	}
	f.store.copy.Syncable = false
	f.svc.FollowDigital(context.Background(), state, time.Now())
	select {
	case page := <-f.store.pages:
		t.Fatalf("unexpected current_page %d", page)
	case <-time.After(100 * time.Millisecond):
	}
}

func TestPhysicalService_LocateAndCopy(t *testing.T) {
	f := newPhysicalFixture(t, mocks.PhysicalEPUB("x", false), time.Minute)
	ctx := context.Background()
	at := models.ReadingPosition{Href: mocks.PhysicalCh1Href, Offset: 0, Page: 0}

	page, excerpt := f.svc.Locate(ctx, "u", uuid.New(), at)
	assert.Equal(t, 1, page)
	assert.Equal(t, "Chapter 1 sentence 01 is right here.", excerpt)
	page, _ = f.svc.Locate(ctx, "u", uuid.New(),
		models.ReadingPosition{Href: "", Offset: 0, Page: 2})
	assert.Zero(t, page, "a PDF page isn't mapped")

	view, err := f.svc.Copy(ctx, "u", uuid.New())
	require.NoError(t, err)
	assert.True(t, view.Syncable)
	assert.False(t, view.HasPrintPages)
	require.Len(t, view.Chapters, 3)
	assert.Equal(t, PhysicalChapter{
		Title:    "Chapter Two",
		Position: models.ReadingPosition{Href: mocks.PhysicalCh2Href, Offset: 9, Page: 0},
	}, view.Chapters[1])

	f.store.copy.Syncable = false
	view, err = f.svc.Copy(ctx, "u", uuid.New())
	require.NoError(t, err)
	assert.False(t, view.Syncable)
	page, _ = f.svc.Locate(ctx, "u", uuid.New(), at)
	assert.Zero(t, page)

	f.store.err = database.ErrResourceNotFound
	_, err = f.svc.Copy(ctx, "u", uuid.New())
	require.ErrorIs(t, err, database.ErrResourceNotFound)
}

func TestPhysicalService_NoUsableEPUB(t *testing.T) {
	f := newPhysicalFixture(t, []byte("not a zip"), time.Minute)
	ctx := context.Background()
	at := models.ReadingPosition{Href: mocks.PhysicalCh1Href, Offset: 0, Page: 0}

	page, _ := f.svc.Locate(ctx, "u", uuid.New(), at)
	assert.Zero(t, page, "a broken EPUB")
	_, _ = f.svc.Locate(ctx, "u", uuid.New(), at)
	assert.Equal(t, int32(1), f.blobs.gets.Load(), "cached as empty")

	f.files.byFormat[models.FileFormatEPUB].Status = models.FileStatusConverting
	view, err := f.svc.Copy(ctx, "u", uuid.New())
	require.NoError(t, err)
	assert.False(t, view.Syncable, "no ready EPUB")

	delete(f.files.byFormat, models.FileFormatEPUB)
	view, _ = f.svc.Copy(ctx, "u", uuid.New())
	assert.False(t, view.Syncable)

	src := &models.BookFile{ //nolint:exhaustruct //optional fields
		ID: uuid.New(), Format: models.FileFormatPDF, StorageKey: "pdf",
		Status: models.FileStatusReady,
	}
	f.files.byID[src.ID] = src
	//nolint:exhaustruct // optional fields
	f.files.byFormat[models.FileFormatKEPUB] = &models.BookFile{
		ID:           uuid.New(),
		Format:       models.FileFormatKEPUB,
		StorageKey:   "kepub",
		Status:       models.FileStatusReady,
		SourceFileID: &src.ID,
	}
	//nolint:exhaustruct // optional fields
	f.files.byFormat[models.FileFormatEPUB] = &models.BookFile{
		ID:         uuid.New(),
		Format:     models.FileFormatEPUB,
		StorageKey: "epub",
		Status:     models.FileStatusReady,
	}
	view, _ = f.svc.Copy(ctx, "u", uuid.New())
	assert.False(t, view.Syncable, "the Kobo reads a PDF-sourced KEPUB")

	f.files.err = errors.New("db down")
	view, _ = f.svc.Copy(ctx, "u", uuid.New())
	assert.False(t, view.Syncable)
}

func TestPhysicalService_DownloadFailureIsRetried(t *testing.T) {
	f := newPhysicalFixture(t, mocks.PhysicalEPUB("x", false), time.Minute)
	ctx := context.Background()
	data, _ := f.blobs.GetContent("epub")
	require.NoError(t, f.blobs.Delete(ctx, "epub"))

	view, err := f.svc.Copy(ctx, "u", uuid.New())
	require.NoError(t, err)
	assert.True(t, view.Syncable, "pending: there is an EPUB")
	assert.Empty(t, view.Chapters)

	f.blobs.PutAt("epub", data, time.Now())
	view, _ = f.svc.Copy(ctx, "u", uuid.New())
	assert.Len(t, view.Chapters, 3)
}

func TestPhysicalService_UpdateCopyValidates(t *testing.T) {
	f := newPhysicalFixture(t, mocks.PhysicalEPUB("x", false), time.Minute)
	ctx := context.Background()
	ok := models.PhysicalCopy{
		PageCount: 300, TextStartPage: 9, Syncable: false,
		Anchors: []models.PhysicalAnchor{{Page: 87, Href: mocks.PhysicalCh2Href, Offset: 9}},
	}
	require.NoError(t, f.svc.UpdateCopy(ctx, "u", uuid.New(), ok))
	assert.Equal(t, &ok, f.store.updated)

	tooMany := make([]models.PhysicalAnchor, maxPhysicalAnchors+1)
	anchor := func(page int, href string, offset int) []models.PhysicalAnchor {
		return []models.PhysicalAnchor{{Page: page, Href: href, Offset: offset}}
	}
	//nolint:exhaustruct // each case sets only what it breaks
	bads := []models.PhysicalCopy{
		{PageCount: -1},
		{PageCount: 10, TextStartPage: 200_000},
		{Anchors: tooMany},
		{Anchors: anchor(0, "a", 0)},
		{Anchors: anchor(3, "", 0)},
		{Anchors: anchor(3, "a", -1)},
	}
	for _, bad := range bads {
		require.ErrorIs(t, f.svc.UpdateCopy(ctx, "u", uuid.New(), bad),
			ErrInvalidPhysicalCopy)
	}
}
