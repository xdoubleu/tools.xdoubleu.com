package services

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/google/uuid"

	"tools.xdoubleu.com/apps/books/internal/models"
	"tools.xdoubleu.com/apps/books/pkg/ebookmeta"
	"tools.xdoubleu.com/internal/database"
)

// ErrInvalidFormat is returned when the magic bytes match no supported format.
var ErrInvalidFormat = errors.New("unsupported or unrecognized file format")

// ErrFileTooLarge is returned when the declared file size exceeds MaxUploadBytes.
var ErrFileTooLarge = errors.New("file exceeds maximum allowed size")

// ErrInvalidUploadID is returned when upload_id isn't under the caller's
// users/<userID>/uploads/ prefix.
var ErrInvalidUploadID = errors.New("invalid or unauthorized upload_id")

// ErrUploadMissing is returned when the client skipped the PUT (already_exists)
// but the blob is gone; the client should retry the full upload.
var ErrUploadMissing = errors.New("upload missing: retry the upload")

// ErrUnrecognizedBook is returned when an upload matches no known book; the
// temp object is removed.
var ErrUnrecognizedBook = errors.New("book could not be recognized from metadata")

// MaxUploadBytes caps one raw upload. Keep in sync with MAX_UPLOAD_BYTES in
// web/lib/backlog/zipFiles.ts.
const MaxUploadBytes = 250 * 1024 * 1024

const uploadPresignTTL = 60 * time.Minute

const magicBytesLen = 4

const maxFilenameBytes = 255

// booksFolderPrefix is where every book's files live: books/<bookID>/<name>.
const booksFolderPrefix = "books/"

// bookFileKey returns books/<bookID>/<checksum><ext>.
func bookFileKey(bookID fmt.Stringer, checksum, ext string) string {
	return booksFolderPrefix + bookID.String() + "/" + checksum + ext
}

func bookCoverKey(bookID fmt.Stringer) string {
	return booksFolderPrefix + bookID.String() + "/cover.jpg"
}

// bookCoverMissingKey marks a book as having no cover (negative cache).
func bookCoverMissingKey(bookID fmt.Stringer) string {
	return booksFolderPrefix + bookID.String() + "/cover.missing"
}

const extEPUB = ".epub"
const extPDF = ".pdf"
const extKEPUB = ".kepub"

// UploadFileResult holds the outcome of a successful FinalizeUpload call.
type UploadFileResult struct {
	BookFile        *models.BookFile
	UserBook        *models.UserBook
	MatchedExisting bool
}

// CreateUpload validates the size and, unless a blob with the checksum already
// exists, returns a presigned R2 PUT URL. When alreadyExists is true the
// client skips the PUT and calls FinalizeUpload directly.
func (s *BookService) CreateUpload(
	ctx context.Context,
	userID string,
	filename string,
	contentType string,
	size int64,
	checksum string,
) (string, string, bool, error) {
	if size > MaxUploadBytes {
		return "", "", false, ErrFileTooLarge
	}

	if checksum != "" {
		_, lookupErr := s.bookFiles.FindByChecksumGlobal(ctx, checksum)
		if lookupErr == nil {
			return "", "", true, nil
		}
		if !errors.Is(lookupErr, database.ErrResourceNotFound) {
			return "", "", false, lookupErr
		}
	}

	ext := extForContentType(contentType, filename)
	uploadID := fmt.Sprintf("users/%s/uploads/%s%s", userID, uuid.New().String(), ext)

	presignURL, presignErr := s.objectStore.PresignPut(
		ctx,
		uploadID,
		uploadPresignTTL,
		contentType,
	)
	if presignErr != nil {
		return "", "", false, fmt.Errorf("presign upload: %w", presignErr)
	}
	return uploadID, presignURL, false, nil
}

// FinalizeUpload processes an uploaded (or skipped, already-existing) file,
// storing one canonical R2 object per content checksum across all users.
func (s *BookService) FinalizeUpload(
	ctx context.Context,
	userID string,
	uploadID string,
	filename string,
	_ string,
	checksum string,
	titleOverride string,
	authorOverride string,
) (*UploadFileResult, error) {
	existing, err := s.bookFiles.FindByChecksumGlobal(ctx, checksum)
	if err == nil {
		return s.finalizeDuplicate(ctx, userID, uploadID, filename, checksum, existing)
	}
	if !errors.Is(err, database.ErrResourceNotFound) {
		return nil, err
	}

	return s.finalizeNew(
		ctx, userID, uploadID, filename, checksum, titleOverride, authorOverride,
	)
}

// attachToCatalogBook adds an already-fetched catalog book to the user's
// library unless present, returning the user_book.
func (s *BookService) attachToCatalogBook(
	ctx context.Context,
	userID string,
	book *models.Book,
) (*models.UserBook, error) {
	ub, err := s.books.GetUserBook(ctx, userID, book.ID)
	if err == nil {
		return ub, nil
	}
	if !errors.Is(err, database.ErrResourceNotFound) {
		return nil, err
	}

	newUB := models.UserBook{ //nolint:exhaustruct //optional fields
		UserID:         userID,
		BookID:         book.ID,
		Book:           book,
		Status:         models.StatusToRead,
		Tags:           []string{},
		ShelfPositions: map[string]int{},
	}
	if upsertErr := s.books.UpsertUserBook(ctx, newUB); upsertErr != nil {
		return nil, upsertErr
	}
	return &newUB, nil
}

// finalizeDuplicate points the user's book_files row at the existing blob,
// transferring no bytes.
func (s *BookService) finalizeDuplicate(
	ctx context.Context,
	userID string,
	uploadID string,
	filename string,
	checksum string,
	existing *models.BookFile,
) (*UploadFileResult, error) {
	matchedExisting := true
	ub, err := s.books.GetUserBook(ctx, userID, existing.BookID)
	if errors.Is(err, database.ErrResourceNotFound) {
		matchedExisting = false
		newUB := models.UserBook{ //nolint:exhaustruct //optional fields
			UserID:         userID,
			BookID:         existing.BookID,
			Status:         models.StatusToRead,
			Tags:           []string{},
			ShelfPositions: map[string]int{},
		}
		if upsertErr := s.books.UpsertUserBook(ctx, newUB); upsertErr != nil {
			return nil, upsertErr
		}
		ub, err = s.books.GetUserBook(ctx, userID, existing.BookID)
		if err != nil {
			return nil, err
		}
	} else if err != nil {
		return nil, err
	}

	if tagErr := s.ensureTag(ctx, userID, ub.BookID, models.TagOwnDigital); tagErr != nil {
		return nil, tagErr
	}

	row, lookupErr := s.bookFiles.FindByChecksum(
		ctx, userID, existing.BookID, existing.Format, checksum,
	)
	if lookupErr == nil {
		cleanupTempUpload(ctx, s, uploadID, userID)
		return &UploadFileResult{
			BookFile:        row,
			UserBook:        ub,
			MatchedExisting: matchedExisting,
		}, nil
	}
	if !errors.Is(lookupErr, database.ErrResourceNotFound) {
		return nil, lookupErr
	}

	destKey := existing.StorageKey

	bf, insertErr := s.bookFiles.Insert(
		ctx,
		models.BookFile{ //nolint:exhaustruct //optional fields
			BookID:           existing.BookID,
			UserID:           userID,
			Format:           existing.Format,
			StorageKey:       destKey,
			SizeBytes:        existing.SizeBytes,
			Checksum:         &checksum,
			OriginalFilename: &filename,
			Status:           models.FileStatusReady,
		},
	)
	if insertErr != nil {
		return nil, insertErr
	}

	cleanupTempUpload(ctx, s, uploadID, userID)

	return &UploadFileResult{
		BookFile:        bf,
		UserBook:        ub,
		MatchedExisting: matchedExisting,
	}, nil
}

type uploadedFile struct {
	tmp      *os.File
	size     int64
	format   string
	meta     ebookmeta.Metadata
	checksum string
}

// loadUploadedFile streams the upload to a temp file, validates magic bytes,
// extracts metadata and checksums it. The caller closes and removes tmp.
func (s *BookService) loadUploadedFile(
	ctx context.Context,
	uploadID string,
) (*uploadedFile, error) {
	rc, err := s.objectStore.Get(ctx, uploadID)
	if err != nil {
		return nil, ErrUploadMissing
	}
	defer rc.Close()

	tmp, err := os.CreateTemp("", "bookupload-*")
	if err != nil {
		return nil, fmt.Errorf("create temp file: %w", err)
	}

	size, err := io.Copy(tmp, rc)
	if err != nil {
		_ = tmp.Close()
		_ = os.Remove(tmp.Name())
		return nil, fmt.Errorf("stream upload to disk: %w", err)
	}

	magic := make([]byte, magicBytesLen)
	if _, readErr := tmp.ReadAt(magic, 0); readErr != nil {
		_ = tmp.Close()
		_ = os.Remove(tmp.Name())
		return nil, ErrInvalidFormat
	}
	format := ebookmeta.DetectFormatFromMagic(magic)
	if format == "" {
		_ = tmp.Close()
		_ = os.Remove(tmp.Name())
		return nil, ErrInvalidFormat
	}

	meta, _ := ebookmeta.Extract(format, tmp, size)
	checksum, err := checksumFile(tmp)
	if err != nil {
		_ = tmp.Close()
		_ = os.Remove(tmp.Name())
		return nil, err
	}

	return &uploadedFile{
		tmp:      tmp,
		size:     size,
		format:   format,
		meta:     meta,
		checksum: checksum,
	}, nil
}

// finalizeNew validates new content, copies it to its content-addressed key,
// and inserts the book_files row.
func (s *BookService) finalizeNew(
	ctx context.Context,
	userID string,
	uploadID string,
	filename string,
	_ string,
	titleOverride string,
	authorOverride string,
) (*UploadFileResult, error) {
	prefix := fmt.Sprintf("users/%s/uploads/", userID)
	if !strings.HasPrefix(uploadID, prefix) {
		return nil, ErrInvalidUploadID
	}

	uf, err := s.loadUploadedFile(ctx, uploadID)
	if err != nil {
		if errors.Is(err, ErrInvalidFormat) {
			_ = s.objectStore.Delete(context.WithoutCancel(ctx), uploadID)
		}
		return nil, err
	}
	defer func() {
		_ = uf.tmp.Close()
		_ = os.Remove(uf.tmp.Name())
	}()

	if len(filename) > maxFilenameBytes {
		filename = filename[:maxFilenameBytes]
	}

	// A caller-supplied title/author (recovery UI) wins; otherwise fall back to
	// the filename when metadata has no title (common for PDFs).
	meta := uf.meta
	if titleOverride != "" {
		meta.Title = titleOverride
	} else if meta.Title == "" {
		meta.Title = titleFromFilename(filename)
	}
	if authorOverride != "" {
		meta.Authors = []string{authorOverride}
	}

	ub, matchedExisting, err := s.recognizeBook(ctx, userID, meta)
	if err != nil {
		if errors.Is(err, ErrUnrecognizedBook) {
			_ = s.objectStore.Delete(context.WithoutCancel(ctx), uploadID)
		}
		return nil, err
	}

	if tagErr := s.ensureTag(ctx, userID, ub.BookID, models.TagOwnDigital); tagErr != nil {
		return nil, tagErr
	}

	// Dedup within (user, book, format) against a concurrent finalizeNew.
	dupe, dupeErr := s.bookFiles.FindByChecksum(
		ctx, userID, ub.BookID, uf.format, uf.checksum,
	)
	if dupeErr == nil {
		_ = s.objectStore.Delete(context.WithoutCancel(ctx), uploadID)
		return &UploadFileResult{
			BookFile:        dupe,
			UserBook:        ub,
			MatchedExisting: matchedExisting,
		}, nil
	}
	if !errors.Is(dupeErr, database.ErrResourceNotFound) {
		return nil, dupeErr
	}

	canonicalKey := bookFileKey(ub.BookID, uf.checksum, extForFormat(uf.format))
	bgCtx := context.WithoutCancel(ctx)
	if copyErr := s.objectStore.Copy(bgCtx, uploadID, canonicalKey); copyErr != nil {
		return nil, fmt.Errorf("copy to canonical key: %w", copyErr)
	}
	_ = s.objectStore.Delete(bgCtx, uploadID)

	bf, err := s.bookFiles.Insert(
		ctx,
		models.BookFile{ //nolint:exhaustruct //optional fields
			BookID:           ub.BookID,
			UserID:           userID,
			Format:           uf.format,
			StorageKey:       canonicalKey,
			SizeBytes:        uf.size,
			Checksum:         &uf.checksum,
			OriginalFilename: &filename,
			Status:           models.FileStatusReady,
		},
	)
	if err != nil {
		return nil, err
	}

	return &UploadFileResult{
		BookFile:        bf,
		UserBook:        ub,
		MatchedExisting: matchedExisting,
	}, nil
}

// titleFromFilename derives a best-effort title: extension stripped, "-"/"_"
// turned into spaces.
func titleFromFilename(filename string) string {
	base := strings.TrimSuffix(filename, filepath.Ext(filename))
	base = strings.NewReplacer("-", " ", "_", " ").Replace(base)
	return strings.TrimSpace(base)
}

// cleanupTempUpload best-effort deletes a temp upload owned by userID.
func cleanupTempUpload(
	ctx context.Context,
	s *BookService,
	uploadID string,
	userID string,
) {
	prefix := fmt.Sprintf("users/%s/uploads/", userID)
	if uploadID != "" && strings.HasPrefix(uploadID, prefix) {
		_ = s.objectStore.Delete(context.WithoutCancel(ctx), uploadID)
	}
}

func checksumFile(f *os.File) (string, error) {
	hasher := sha256.New()
	if _, err := f.Seek(0, io.SeekStart); err != nil {
		return "", fmt.Errorf("seek temp file: %w", err)
	}
	if _, err := io.Copy(hasher, f); err != nil {
		return "", fmt.Errorf("checksum temp file: %w", err)
	}
	return fmt.Sprintf("%x", hasher.Sum(nil)), nil
}

func extForContentType(contentType, filename string) string {
	switch contentType {
	case "application/epub+zip":
		return extEPUB
	case contentTypePDF:
		return extPDF
	}
	lower := strings.ToLower(filename)
	switch {
	case strings.HasSuffix(lower, extEPUB):
		return extEPUB
	case strings.HasSuffix(lower, extPDF):
		return extPDF
	}
	return ""
}

func extForFormat(format string) string {
	switch format {
	case models.FileFormatEPUB:
		return extEPUB
	case models.FileFormatKEPUB:
		return extKEPUB
	case models.FileFormatPDF:
		return extPDF
	}
	return ""
}

// recognizeBook matches meta to a user_book, most precise first: ISBN13, exact
// title + first author, catalog-wide normalized/fuzzy title + author (so a
// second format attaches to the existing entry), then an external search
// (matchedExisting=false).
func (s *BookService) recognizeBook(
	ctx context.Context,
	userID string,
	meta ebookmeta.Metadata,
) (*models.UserBook, bool, error) {
	if meta.ISBN13 != nil {
		ub, err := s.books.FindUserBookByISBN13(ctx, userID, *meta.ISBN13)
		if err == nil {
			return ub, true, nil
		}
		if !errors.Is(err, database.ErrResourceNotFound) {
			return nil, false, err
		}
	}

	if meta.Title != "" && len(meta.Authors) > 0 {
		ub, err := s.books.FindUserBookByTitleAndAuthor(
			ctx, userID, meta.Title, meta.Authors[0],
		)
		if err == nil {
			return ub, true, nil
		}
		if !errors.Is(err, database.ErrResourceNotFound) {
			return nil, false, err
		}
	}

	// The whole catalog is cheap compared to the external round trips that follow.
	catalog, err := s.books.GetCatalogWithUserOverlay(ctx, userID)
	if err != nil {
		return nil, false, err
	}
	if match := matchCatalogByMetadata(catalog, meta); match != nil {
		ub, attachErr := s.attachToCatalogBook(ctx, userID, match.Book)
		return ub, attachErr == nil, attachErr
	}

	if ub := s.tryExternalLookup(ctx, userID, meta); ub != nil {
		return ub, false, nil
	}

	return nil, false, ErrUnrecognizedBook
}

// tryExternalLookup adds the providers' top result to the library, or returns nil.
func (s *BookService) tryExternalLookup(
	ctx context.Context,
	userID string,
	meta ebookmeta.Metadata,
) *models.UserBook {
	if meta.Title == "" {
		return nil
	}
	query := meta.Title
	if len(meta.Authors) > 0 {
		query = meta.Title + " " + meta.Authors[0]
	}
	results := s.SearchExternal(ctx, query)
	if len(results) == 0 {
		return nil
	}
	ub, addErr := s.AddToLibrary(
		ctx,
		userID,
		results[0],
		models.StatusToRead,
		[]string{},
	)
	if addErr != nil {
		return nil
	}
	return ub
}

func (s *BookService) ensureTag(
	ctx context.Context,
	userID string,
	bookID uuid.UUID,
	tag string,
) error {
	ub, err := s.books.GetUserBook(ctx, userID, bookID)
	if err != nil {
		return err
	}
	for _, t := range ub.Tags {
		if t == tag {
			return nil
		}
	}
	newTags := append(ub.Tags, tag) //nolint:gocritic // intentional: tags is owned here
	return s.books.UpdateTags(
		ctx, userID, bookID, newTags,
		slices.Contains(newTags, models.TagKoboSync),
	)
}

// KEPUBStatusResult is returned by GetKEPUBStatus.
type KEPUBStatusResult struct {
	HasEPUB     bool
	HasPDF      bool
	KepubStatus string // "", "converting", "ready", or "failed"
	// KepubStale means a "ready" KEPUB from an older converter version; treat it
	// as missing and re-trigger conversion.
	KepubStale bool
}

// GetKEPUBStatus reports whether the book has an EPUB/PDF and its KEPUB's
// status, for the Kobo-sync toggle.
func (s *BookService) GetKEPUBStatus(
	ctx context.Context,
	userID string,
	bookID uuid.UUID,
) (*KEPUBStatusResult, error) {
	result := &KEPUBStatusResult{} //nolint:exhaustruct //fields set below conditionally

	_, epubErr := s.bookFiles.GetByBookAndFormat(
		ctx,
		userID,
		bookID,
		models.FileFormatEPUB,
	)
	if epubErr == nil {
		result.HasEPUB = true
	} else if !errors.Is(epubErr, database.ErrResourceNotFound) {
		return nil, epubErr
	}

	_, pdfErr := s.bookFiles.GetByBookAndFormat(
		ctx,
		userID,
		bookID,
		models.FileFormatPDF,
	)
	if pdfErr == nil {
		result.HasPDF = true
	} else if !errors.Is(pdfErr, database.ErrResourceNotFound) {
		return nil, pdfErr
	}

	kepub, kepubErr := s.bookFiles.GetByBookAndFormat(
		ctx,
		userID,
		bookID,
		models.FileFormatKEPUB,
	)
	if kepubErr == nil {
		result.KepubStatus = kepub.Status
		if kepub.Status == models.FileStatusReady &&
			kepub.ConverterVersion < currentKEPUBConverterVersion {
			result.KepubStale = true
		}
	} else if !errors.Is(kepubErr, database.ErrResourceNotFound) {
		return nil, kepubErr
	}

	return result, nil
}

// GetKoboFileFormat returns "pdf" with the kobo-format-pdf tag, else "kepub";
// ErrResourceNotFound when the user_book doesn't exist.
func (s *BookService) GetKoboFileFormat(
	ctx context.Context,
	userID string,
	bookID uuid.UUID,
) (string, error) {
	ub, err := s.books.GetUserBook(ctx, userID, bookID)
	if err != nil {
		return "", err
	}
	for _, t := range ub.Tags {
		if t == models.TagKoboFormatPDF {
			return models.FileFormatPDF, nil
		}
	}
	return models.FileFormatKEPUB, nil
}

// EnableKoboSync idempotently adds the kobo-sync tag to the user's book.
func (s *BookService) EnableKoboSync(
	ctx context.Context,
	userID string,
	bookID uuid.UUID,
) error {
	if err := s.ensureTag(ctx, userID, bookID, models.TagKoboSync); err != nil {
		return err
	}
	return s.books.DeleteKoboRemoval(ctx, userID, bookID)
}

const presignTTL = 5 * time.Minute

// GetBookFileResult holds the outcome of a successful GetBookFile call.
type GetBookFileResult struct {
	URL       string
	ExpiresAt time.Time
	Format    string
}

// GetBookFile returns a presigned URL for the book's file (first ready pdf/epub
// when format is empty). ErrResourceNotFound also covers another user's file;
// callers must not distinguish.
func (s *BookService) GetBookFile(
	ctx context.Context,
	userID string,
	bookID uuid.UUID,
	format string,
) (*GetBookFileResult, error) {
	file, err := s.resolveBookFile(ctx, userID, bookID, format)
	if err != nil {
		return nil, err
	}

	url, presignErr := s.objectStore.PresignGet(ctx, file.StorageKey, presignTTL)
	if presignErr != nil {
		return nil, fmt.Errorf("presign: %w", presignErr)
	}

	return &GetBookFileResult{
		URL:       url,
		ExpiresAt: time.Now().Add(presignTTL),
		Format:    file.Format,
	}, nil
}

func (s *BookService) resolveBookFile(
	ctx context.Context,
	userID string,
	bookID uuid.UUID,
	format string,
) (*models.BookFile, error) {
	if format != "" {
		return s.bookFiles.GetByBookAndFormat(ctx, userID, bookID, format)
	}

	files, err := s.bookFiles.ListByBook(ctx, userID, bookID)
	if err != nil {
		return nil, err
	}
	for i := range files {
		if files[i].Format != models.FileFormatKEPUB &&
			files[i].Status == models.FileStatusReady {
			return &files[i], nil
		}
	}
	return nil, database.ErrResourceNotFound
}

// FormatsByUser returns book ID -> ready formats for the user's whole library.
func (s *BookService) FormatsByUser(
	ctx context.Context,
	userID string,
) (map[uuid.UUID][]string, error) {
	return s.bookFiles.FormatsByUser(ctx, userID)
}
