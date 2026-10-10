package services

import (
	"archive/zip"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"path"
	"strings"

	"github.com/google/uuid"

	"tools.xdoubleu.com/apps/books/internal/models"
	"tools.xdoubleu.com/internal/database"
)

// GetUserBookCover is GetBookCover for one of userID's books: when the book
// has no usable cover URL, the cover embedded in the user's EPUB (else KEPUB)
// is cached instead.
func (s *BookService) GetUserBookCover(
	ctx context.Context,
	userID string,
	bookID uuid.UUID,
) (*GetBookCoverResult, error) {
	cached, err := s.EnsureUserCoverCached(ctx, userID, bookID)
	if err != nil {
		s.logger.Warn("failed to cache user book cover",
			"bookID", bookID, "err", err)
	}
	if !cached {
		return nil, ErrCoverNotFound
	}
	return s.presignCover(ctx, bookCoverKey(bookID))
}

// EnsureUserCoverCached is EnsureCoverCached falling back to the cover
// embedded in userID's EPUB (else KEPUB) of the book.
func (s *BookService) EnsureUserCoverCached(
	ctx context.Context,
	userID string,
	bookID uuid.UUID,
) (bool, error) {
	cached, urlErr := s.EnsureCoverCached(ctx, bookID)
	if cached {
		return true, nil
	}
	fileErr := s.cacheCoverFromFile(ctx, userID, bookID)
	if fileErr == nil {
		return true, nil
	}
	if errors.Is(fileErr, ErrCoverNotFound) {
		return false, urlErr
	}
	return false, errors.Join(urlErr, fileErr)
}

// cacheCoverFromFile stores the cover image of userID's first ready EPUB or
// KEPUB of bookID; ErrCoverNotFound when none has one.
func (s *BookService) cacheCoverFromFile(
	ctx context.Context,
	userID string,
	bookID uuid.UUID,
) error {
	for _, format := range []string{models.FileFormatEPUB, models.FileFormatKEPUB} {
		file, err := s.bookFiles.GetByBookAndFormat(ctx, userID, bookID, format)
		if errors.Is(err, database.ErrResourceNotFound) {
			continue
		}
		if err != nil {
			return err
		}
		if file.Status != models.FileStatusReady {
			continue
		}
		zr, err := s.openStoredZip(ctx, file.StorageKey)
		if err != nil {
			return err
		}
		data, err := epubCover(zr)
		if errors.Is(err, ErrCoverNotFound) {
			continue
		}
		if err != nil {
			return err
		}
		return s.objectStore.Put(
			ctx,
			bookCoverKey(bookID),
			bytes.NewReader(data),
			int64(len(data)),
			http.DetectContentType(data),
		)
	}
	return ErrCoverNotFound
}

// openStoredZip reads a stored EPUB/KEPUB into memory as a zip.
func (s *BookService) openStoredZip(
	ctx context.Context,
	key string,
) (*zip.Reader, error) {
	rc, err := s.objectStore.Get(ctx, key)
	if err != nil {
		return nil, fmt.Errorf("get epub: %w", err)
	}
	defer func() { _ = rc.Close() }()
	data, err := io.ReadAll(rc)
	if err != nil {
		return nil, fmt.Errorf("read epub: %w", err)
	}
	return zip.NewReader(bytes.NewReader(data), int64(len(data)))
}

// epubCover returns the bytes of the package's cover image: the EPUB 3
// cover-image item, else the item named by EPUB 2's <meta name="cover">.
func epubCover(zr *zip.Reader) ([]byte, error) {
	opfPath, err := findOPFPath(zr)
	if err != nil {
		return nil, err
	}
	var opf opfDoc
	if err = decodeZipXML(zr, opfPath, &opf); err != nil {
		return nil, err
	}

	item, ok := opfCoverItem(opf)
	if !ok {
		return nil, ErrCoverNotFound
	}
	name := resolveHref(path.Dir(opfPath), item.Href)
	f, err := zr.Open(name)
	if err != nil {
		return nil, ErrCoverNotFound
	}
	defer func() { _ = f.Close() }()

	data, err := io.ReadAll(io.LimitReader(f, maxCoverBytes+1))
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", name, err)
	}
	if len(data) > maxCoverBytes ||
		!strings.HasPrefix(http.DetectContentType(data), "image/") {
		return nil, ErrCoverNotFound
	}
	return data, nil
}

func opfCoverItem(opf opfDoc) (opfItem, bool) {
	for _, it := range opf.Items {
		if strings.Contains(" "+it.Properties+" ", " cover-image ") {
			return it, true
		}
	}
	for _, m := range opf.Metadata.Metas {
		if m.Name != "cover" {
			continue
		}
		for _, it := range opf.Items {
			if it.ID == m.Content && strings.HasPrefix(it.MediaType, "image/") {
				return it, true
			}
		}
	}
	return opfItem{}, false //nolint:exhaustruct // zero value on a miss
}
