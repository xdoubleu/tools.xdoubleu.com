package positions

import (
	"context"
	"errors"
	"io"
	"os"
	"time"

	"github.com/google/uuid"

	"tools.xdoubleu.com/apps/books/internal/models"
	"tools.xdoubleu.com/internal/kit/database"
)

// pdfLabels is bookID's PDF page labels, loaded within wait. It is nil when
// the book has no ready PDF, the PDF has no printed page labels, or loading
// failed (retried next time; a PDF read to the end is cached).
func (s *PositionService) pdfLabels(
	ctx context.Context,
	userID string,
	bookID uuid.UUID,
	wait time.Duration,
) *pdfLabels {
	pdfFile, err := s.files.GetByBookAndFormat(ctx, userID, bookID, models.FileFormatPDF)
	if err != nil {
		if !errors.Is(err, database.ErrResourceNotFound) {
			s.logger.WarnContext(ctx, "pdf lookup failed", "book_id", bookID, "err", err)
		}
		return nil
	}
	if pdfFile.Status != models.FileStatusReady {
		return nil
	}
	key := bookTextKey{fileID: pdfFile.ID, storageKey: pdfFile.StorageKey}
	if l, hit := s.labels.get(key); hit {
		return usableLabels(l)
	}
	ctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), wait)
	defer cancel()
	l, err := s.loadPDFLabels(ctx, key.storageKey)
	if err != nil {
		s.logger.WarnContext(ctx, "pdf page labels failed",
			"pdf_file_id", key.fileID, "err", err)
		// A broken PDF is cached empty; a download or timeout is retried.
		if errors.Is(err, errSpanMapRetry) || ctx.Err() != nil {
			return nil
		}
		l = nil
	}
	s.labels.add(key, l)
	return usableLabels(l)
}

func usableLabels(l *pdfLabels) *pdfLabels {
	if l.usable() {
		return l
	}
	return nil
}

// loadPDFLabels downloads the PDF to an unlinked temp file, as OpenStoredZip
// does, and reads its labels.
func (s *PositionService) loadPDFLabels(
	ctx context.Context,
	key string,
) (*pdfLabels, error) {
	rc, err := s.objectStore.Get(ctx, key)
	if err != nil {
		return nil, errors.Join(errSpanMapRetry, err)
	}
	defer func() { _ = rc.Close() }()
	f, err := os.CreateTemp("", "book-*.pdf")
	if err != nil {
		return nil, errors.Join(errSpanMapRetry, err)
	}
	_ = os.Remove(f.Name())
	defer func() { _ = f.Close() }()
	n, err := io.Copy(f, io.LimitReader(rc, s.maxDownload+1))
	if err != nil {
		return nil, errors.Join(errSpanMapRetry, err)
	}
	if n > s.maxDownload {
		return nil, errors.New("pdf is too large")
	}
	return readPDFLabels(f, n)
}
