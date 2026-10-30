package services

import (
	"archive/zip"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"maps"
	"net/http"
	"path"
	"slices"
	"strings"

	"github.com/google/uuid"

	"tools.xdoubleu.com/apps/books/internal/models"
	"tools.xdoubleu.com/apps/books/internal/services/conversion"
	"tools.xdoubleu.com/apps/books/pkg/ebooks"
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

// maxCoverMissingBytes bounds the cover.missing marker read.
const maxCoverMissingBytes = 64 << 10

// cacheCoverFromFile stores the cover image of userID's first ready EPUB or
// KEPUB of bookID; ErrCoverNotFound when none has one. The book's
// cover.missing marker lists the files found coverless, which are never
// downloaded again; a new or reconverted file isn't listed, so it is.
func (s *BookService) cacheCoverFromFile(
	ctx context.Context,
	userID string,
	bookID uuid.UUID,
) error {
	coverless := s.coverlessFiles(ctx, bookID)
	known := len(coverless)
	defer func() {
		if len(coverless) > known {
			s.markCoverless(ctx, bookID, coverless)
		}
	}()
	for _, format := range []string{models.FileFormatEPUB, models.FileFormatKEPUB} {
		file, err := s.bookFiles.GetByBookAndFormat(ctx, userID, bookID, format)
		if errors.Is(err, database.ErrResourceNotFound) {
			continue
		}
		if err != nil {
			return err
		}
		// A KEPUB's key derives from its source, so its version tells
		// reconversions apart.
		id := fmt.Sprintf("%s@%d", file.StorageKey, file.ConverterVersion)
		if file.Status != models.FileStatusReady || coverless[id] {
			continue
		}
		data, err := s.storedCover(ctx, file.StorageKey)
		if errors.Is(err, ErrCoverNotFound) {
			coverless[id] = true
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

func (s *BookService) storedCover(ctx context.Context, key string) ([]byte, error) {
	z, err := s.openStoredZip(ctx, key)
	if err != nil {
		return nil, err
	}
	defer z.close()
	return epubCover(z.zr)
}

// coverlessFiles reads the files bookID's cover.missing marker lists; a
// missing or unreadable marker lists none.
func (s *BookService) coverlessFiles(
	ctx context.Context,
	bookID uuid.UUID,
) map[string]bool {
	keys := map[string]bool{}
	rc, err := s.objectStore.Get(ctx, bookCoverMissingKey(bookID))
	if err != nil {
		return keys
	}
	defer func() { _ = rc.Close() }()
	data, err := io.ReadAll(io.LimitReader(rc, maxCoverMissingBytes))
	if err != nil {
		return keys
	}
	for _, key := range strings.Fields(string(data)) {
		keys[key] = true
	}
	return keys
}

func (s *BookService) markCoverless(
	ctx context.Context,
	bookID uuid.UUID,
	keys map[string]bool,
) {
	body := strings.Join(slices.Sorted(maps.Keys(keys)), "\n")
	err := s.objectStore.Put(ctx, bookCoverMissingKey(bookID),
		strings.NewReader(body), int64(len(body)), "text/plain")
	if err != nil {
		s.logger.Warn("failed to write cover missing marker",
			"bookID", bookID, "err", err)
	}
}

// openStoredZip opens a stored EPUB/KEPUB as a temp-file zip.
func (s *BookService) openStoredZip(
	ctx context.Context,
	key string,
) (*tempZip, error) {
	return openTempZip(ctx, s.objectStore, key, conversion.MaxConversionInputBytes)
}

// epubCover returns the bytes of the package's cover image: the EPUB 3
// cover-image item, else the item named by EPUB 2's <meta name="cover">.
func epubCover(zr *zip.Reader) ([]byte, error) {
	opfPath, err := ebooks.FindOPFPath(zr)
	if err != nil {
		return nil, err
	}
	var opf ebooks.OPFDoc
	if err = ebooks.DecodeZipXML(zr, opfPath, &opf); err != nil {
		return nil, err
	}

	item, ok := ebooks.OPFCoverItem(opf)
	if !ok {
		return nil, ErrCoverNotFound
	}
	name := ebooks.ResolveHref(path.Dir(opfPath), item.Href)
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
