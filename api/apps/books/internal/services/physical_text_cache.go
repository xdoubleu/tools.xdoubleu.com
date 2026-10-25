package services

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"

	"tools.xdoubleu.com/apps/books/internal/models"
	"tools.xdoubleu.com/internal/database"
)

// bookTextCacheSize is small: a text model holds a whole book's text.
const bookTextCacheSize = 4

// bookTextKey identifies a stored EPUB.
type bookTextKey struct {
	fileID     uuid.UUID
	storageKey string
}

// epubText resolves the user's ready EPUB of bookID. ok is false when there's
// none, or when the user's KEPUB was converted from a PDF, since its
// positions are PDF pages.
func (s *PositionService) epubText(
	ctx context.Context,
	userID string,
	bookID uuid.UUID,
) (bookTextKey, bool) {
	none := bookTextKey{fileID: uuid.Nil, storageKey: ""}
	if s.resolveAll(ctx, userID, []uuid.UUID{bookID})[bookID].pdf {
		return none, false
	}
	epub, err := s.files.GetByBookAndFormat(ctx, userID, bookID, models.FileFormatEPUB)
	if err != nil {
		if !errors.Is(err, database.ErrResourceNotFound) {
			s.logger.WarnContext(ctx, "epub lookup failed", "book_id", bookID, "err", err)
		}
		return none, false
	}
	if epub.Status != models.FileStatusReady {
		return none, false
	}
	return bookTextKey{fileID: epub.ID, storageKey: epub.StorageKey}, true
}

// bookText is bookID's text model, waiting up to wait for it to build. It is
// nil when the book has no usable EPUB, or (pending) is still building or
// failing retryably within the pending grace.
func (s *PositionService) bookText(
	ctx context.Context,
	userID string,
	bookID uuid.UUID,
	wait time.Duration,
) (*bookText, bool) {
	key, ok := s.epubText(ctx, userID, bookID)
	if !ok {
		return nil, false
	}
	if t, hit := s.texts.get(key); hit {
		return usableText(t), false
	}
	buildCtx := context.WithoutCancel(ctx)
	ch := s.builds.DoChan("text:"+key.fileID.String()+":"+key.storageKey,
		func() (any, error) {
			if t, hit := s.texts.get(key); hit {
				return t, nil
			}
			return s.buildText(buildCtx, key), nil
		})
	timer := time.NewTimer(wait)
	defer timer.Stop()
	select {
	case res := <-ch:
		if t, _ := res.Val.(*bookText); t != nil {
			return usableText(t), false
		}
	case <-timer.C:
	case <-ctx.Done():
	}
	return nil, s.stillPending(key)
}

// usableText is nil for the empty model cached for a broken EPUB.
func usableText(t *bookText) *bookText {
	if t == nil || len(t.docs) == 0 {
		return nil
	}
	return t
}

// buildText parses key's EPUB and caches it, as build does span maps: a
// broken EPUB caches an empty model, a retryable failure caches nothing and
// returns nil.
func (s *PositionService) buildText(ctx context.Context, key bookTextKey) *bookText {
	waitCtx, cancelWait := context.WithTimeout(ctx, s.buildTimeout)
	defer cancelWait()
	select {
	case s.slots <- struct{}{}:
		defer func() { <-s.slots }()
	case <-waitCtx.Done():
		return nil
	}
	ctx, cancel := context.WithTimeout(ctx, s.buildTimeout)
	defer cancel()

	t, err := s.parseText(ctx, key)
	if err != nil {
		s.logger.WarnContext(ctx, "epub text model build failed",
			"epub_file_id", key.fileID, "err", err)
		if errors.Is(err, errSpanMapRetry) || ctx.Err() != nil {
			return nil
		}
		t = &bookText{docs: nil, cum: []int{0}, start: 0, printPages: nil, chapters: nil}
	}
	s.texts.add(key, t)
	s.clearPending(key)
	return t
}

// parseText downloads and parses key's EPUB; a panic counts as a broken EPUB.
func (s *PositionService) parseText(
	ctx context.Context,
	key bookTextKey,
) (*bookText, error) {
	zr, err := s.openZip(ctx, key.storageKey)
	if err != nil {
		return nil, err
	}
	defer zr.close()
	var t *bookText
	func() {
		defer func() {
			if r := recover(); r != nil {
				err = fmt.Errorf("epub text model build panicked: %v", r)
			}
		}()
		t, err = buildBookText(ctx, zr.zr)
	}()
	return t, err
}
