//nolint:testpackage // testing the unexported translator constructor
package services

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"tools.xdoubleu.com/apps/books/internal/models"
)

// kepubReadyAt is when the fixtures' KEPUB was (re)generated.
//
//nolint:gochecknoglobals // fixed fixture time
var kepubReadyAt = time.Date(2026, 10, 1, 12, 0, 0, 0, time.UTC)

func pagedFixture(t *testing.T, budget time.Duration) *translatorFixture {
	t.Helper()
	f := newTranslatorFixture(t, pagedKEPUBBytes(t), models.FileFormatPDF, budget)
	f.files.byFormat[models.FileFormatKEPUB].UpdatedAt = kepubReadyAt
	return f
}

// koboState is a reading state recorded at updatedAt.
func koboState(
	loc *models.KoboLocation, pos *models.ReadingPosition, updatedAt time.Time,
) *models.BookReadingState {
	return &models.BookReadingState{ //nolint:exhaustruct //optional fields
		Percent: 30, KoboLocation: loc, Position: pos, UpdatedAt: updatedAt,
	}
}

func TestPositionService_PDFSourcedTranslatesPages(t *testing.T) {
	f := pagedFixture(t, time.Minute)
	ctx := context.Background()

	pos := settledPosition(t, f.svc, ctx, f.bookID, *spanLoc(pagedDoc1, "kobo.1.2"))
	assert.Equal(t, &models.ReadingPosition{Href: "", Offset: 0, Page: 3}, pos,
		"a PDF-sourced span is stored as its page")

	page := pagePos(5)
	locs, pending := f.svc.KoboLocations(ctx, "u",
		map[uuid.UUID]*models.BookReadingState{
			f.bookID: koboState(nil, &page, kepubReadyAt.Add(time.Hour)),
		})
	assert.Empty(t, pending)
	assert.Equal(t, map[uuid.UUID]*models.KoboLocation{
		f.bookID: spanLoc(pagedDoc1, "kobo.2.1"),
	}, locs)

	assert.Equal(t,
		models.ReadingPosition{Href: pagedDoc1, Offset: len("Gamma. "), Page: 3},
		f.svc.ReaderPosition(ctx, "u", f.bookID, pagePos(3)))

	assert.Equal(t, int32(1), f.store.gets.Load(),
		"only the KEPUB is downloaded, once")
}

func TestPositionService_ReaderPositionLeavesOthersAlone(t *testing.T) {
	ctx := context.Background()
	epub := newTranslatorFixture(t, nil, models.FileFormatEPUB, time.Minute)
	stored := models.ReadingPosition{Href: "OEBPS/Text/ch2.xhtml", Offset: 3, Page: 0}
	assert.Equal(t, stored, epub.svc.ReaderPosition(ctx, "u", epub.bookID, stored))
	assert.Zero(t, epub.store.gets.Load(), "an EPUB source needs no map")

	f := pagedFixture(t, time.Minute)
	missing := models.ReadingPosition{Href: "OEBPS/missing.xhtml", Offset: 1, Page: 0}
	assert.Equal(t, missing, f.svc.ReaderPosition(ctx, "u", f.bookID, missing))

	none := pagedFixture(t, time.Minute)
	none.files.byFormat = map[string]*models.BookFile{}
	assert.Equal(t, pagePos(2), none.svc.ReaderPosition(ctx, "u", none.bookID, pagePos(2)))
}

func TestPositionService_ReaderPositionPendingKeepsStored(t *testing.T) {
	f := pagedFixture(t, 10*time.Millisecond)
	f.store.gate = make(chan struct{})
	assert.Equal(t, pagePos(3),
		f.svc.ReaderPosition(context.Background(), "u", f.bookID, pagePos(3)))
	close(f.store.gate)
}

func TestPositionService_StaleKoboLocationOfPDFSourcedKEPUB(t *testing.T) {
	ctx := context.Background()
	old := spanLoc(pagedDoc0, "kobo.2.1")
	before, after := kepubReadyAt.Add(-time.Hour), kepubReadyAt.Add(time.Hour)
	page := pagePos(5)

	f := pagedFixture(t, time.Minute)
	locs, pending := f.svc.KoboLocations(ctx, "u",
		map[uuid.UUID]*models.BookReadingState{
			f.bookID: koboState(old, &page, before),
		})
	assert.Empty(t, pending)
	assert.Equal(t, map[uuid.UUID]*models.KoboLocation{
		f.bookID: spanLoc(pagedDoc1, "kobo.2.1"),
	}, locs, "re-derived from the stored page against the new KEPUB")

	noPos := pagedFixture(t, time.Minute)
	locs, _ = noPos.svc.KoboLocations(ctx, "u", map[uuid.UUID]*models.BookReadingState{
		noPos.bookID: koboState(old, nil, before),
	})
	require.Contains(t, locs, noPos.bookID)
	assert.Nil(t, locs[noPos.bookID], "no position: percent only")
	assert.Zero(t, noPos.store.gets.Load())

	fresh := pagedFixture(t, time.Minute)
	locs, _ = fresh.svc.KoboLocations(ctx, "u", map[uuid.UUID]*models.BookReadingState{
		fresh.bookID: koboState(old, &page, after),
	})
	assert.Empty(t, locs, "a bookmark from the current KEPUB is kept")

	epub := newTranslatorFixture(t, nil, models.FileFormatEPUB, time.Minute)
	epub.files.byFormat[models.FileFormatKEPUB].UpdatedAt = kepubReadyAt
	locs, _ = epub.svc.KoboLocations(ctx, "u", map[uuid.UUID]*models.BookReadingState{
		epub.bookID: koboState(chapterTwoSpanPtr("kobo.1.2"), nil, before),
	})
	assert.Empty(t, locs, "kepubify output is stable for an EPUB source")
}

func TestPositionService_StaleKoboLocationPendingDegradesToPercent(t *testing.T) {
	f := pagedFixture(t, 10*time.Millisecond)
	f.store.gate = make(chan struct{})
	page := pagePos(5)
	locs, pending := f.svc.KoboLocations(context.Background(), "u",
		map[uuid.UUID]*models.BookReadingState{
			f.bookID: koboState(spanLoc(pagedDoc0, "kobo.2.1"), &page,
				kepubReadyAt.Add(-time.Hour)),
		})
	close(f.store.gate)
	require.Contains(t, locs, f.bookID)
	assert.Nil(t, locs[f.bookID], "never the old span while the map builds")
	assert.True(t, pending[f.bookID])
}

func chapterTwoSpanPtr(value string) *models.KoboLocation {
	loc := chapterTwoSpan(value)
	return &loc
}
