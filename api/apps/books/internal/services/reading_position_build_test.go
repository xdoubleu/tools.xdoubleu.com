//nolint:testpackage // testing unexported translator internals
package services

import (
	"archive/zip"
	"bytes"
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"tools.xdoubleu.com/apps/books/internal/models"
)

func TestPositionService_PanicCachesEmptyAndLogs(t *testing.T) {
	f := newTranslatorFixture(t, nil, models.FileFormatEPUB, time.Minute)
	f.store.panics = true
	ctx := context.Background()

	assert.Nil(t, settledPosition(t, f.svc, ctx, f.bookID, chapterTwoSpan("kobo.1.2")))
	f.store.panics = false
	assert.Nil(t, settledPosition(t, f.svc, ctx, f.bookID, chapterTwoSpan("kobo.1.2")),
		"a panicking book is cached as untranslatable")
	assert.Equal(t, []string{"kepub span map build panicked"}, f.logs.messages())
}

func TestPositionService_SuccessLogsNothing(t *testing.T) {
	f := newTranslatorFixture(t, nil, models.FileFormatEPUB, time.Minute)
	require.NotNil(t, settledPosition(
		t, f.svc, context.Background(), f.bookID, chapterTwoSpan("kobo.1.2")))
	assert.Empty(t, f.logs.messages())
}

func TestPositionService_LookupErrorsAreLoggedNotFoundIsNot(t *testing.T) {
	ctx := context.Background()
	f := newTranslatorFixture(t, nil, models.FileFormatEPUB, time.Minute)
	f.files.byFormat = map[string]*models.BookFile{}
	assert.Nil(t, settledPosition(t, f.svc, ctx, f.bookID, chapterTwoSpan("kobo.1.2")))
	assert.Empty(t, f.logs.messages())

	f.files.err = errors.New("db down")
	assert.Nil(t, settledPosition(t, f.svc, ctx, f.bookID, chapterTwoSpan("kobo.1.2")))
	assert.Equal(t, []string{"kepub span map lookup failed"}, f.logs.messages())
}

func TestPositionService_BackfillStoreErrorIsLogged(t *testing.T) {
	f := newTranslatorFixture(t, nil, models.FileFormatEPUB, time.Minute)
	ctx := context.Background()
	ok := func(context.Context, models.ReadingPosition) error { return nil }
	f.svc.BackfillKoboPosition(ctx, "u", f.bookID, chapterTwoSpan("kobo.1.2"), ok)
	assert.Empty(t, f.logs.messages())

	failing := func(context.Context, models.ReadingPosition) error {
		return errors.New("write failed")
	}
	f.svc.BackfillKoboPosition(ctx, "u", f.bookID, chapterTwoSpan("kobo.1.2"), failing)
	assert.Equal(t, []string{"kobo position backfill failed"}, f.logs.messages())
}

func TestPositionService_PendingGraceIsExclusive(t *testing.T) {
	f := newTranslatorFixture(t, nil, models.FileFormatEPUB, time.Minute)
	clock := time.Date(2026, 10, 1, 8, 0, 0, 0, time.UTC)
	f.svc.now = func() time.Time { return clock }
	key := spanMapKey{kepubID: uuid.New(), sourceID: uuid.New(), version: 1}

	assert.True(t, f.svc.stillPending(key), "first seen")
	clock = clock.Add(f.svc.pendingGrace - time.Nanosecond)
	assert.True(t, f.svc.stillPending(key))
	clock = clock.Add(time.Nanosecond)
	assert.False(t, f.svc.stillPending(key), "exactly the grace is past it")

	f.svc.markBuilt(key, &spanMap{docs: nil})
	assert.True(t, f.svc.stillPending(key), "a build resets the clock")
}

func TestOpenZip_DownloadSizeCap(t *testing.T) {
	f := newTranslatorFixture(t, nil, models.FileFormatEPUB, time.Minute)
	ctx := context.Background()
	kepub, ok := f.store.GetContent("kepub")
	require.True(t, ok)

	f.svc.maxDownload = int64(len(kepub))
	z, err := f.svc.openZip(ctx, "kepub")
	require.NoError(t, err, "exactly the cap is allowed")
	z.close()

	f.svc.maxDownload = int64(len(kepub)) - 1
	_, err = f.svc.openZip(ctx, "kepub")
	require.Error(t, err)
	assert.NotErrorIs(t, err, errSpanMapRetry, "too large is permanent")
}

func TestOpenZip_RejectsZipBomb(t *testing.T) {
	f := newTranslatorFixture(t, nil, models.FileFormatEPUB, time.Minute)
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	w, err := zw.CreateRaw(&zip.FileHeader{ //nolint:exhaustruct //raw entry
		Name: "bomb.xhtml", Method: zip.Store,
		CompressedSize64: 1, UncompressedSize64: maxEPUBUncompressedBytes + 1,
	})
	require.NoError(t, err)
	_, err = w.Write([]byte("x"))
	require.NoError(t, err)
	require.NoError(t, zw.Close())
	f.store.PutAt("bomb", buf.Bytes(), time.Now())

	_, err = f.svc.openZip(context.Background(), "bomb")
	require.Error(t, err)

	_, err = f.svc.openZip(context.Background(), "missing")
	require.ErrorIs(t, err, errSpanMapRetry)
}
