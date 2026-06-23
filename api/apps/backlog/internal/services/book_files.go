package services

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"slices"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/xdoubleu/essentia/v4/pkg/database"

	"tools.xdoubleu.com/apps/backlog/internal/models"
	"tools.xdoubleu.com/apps/backlog/pkg/ebookmeta"
)

// ErrInvalidFormat is returned when the uploaded file's magic bytes do not
// match any supported format (epub, pdf).
var ErrInvalidFormat = errors.New("unsupported or unrecognized file format")

// ErrFileTooLarge is returned when the declared file size exceeds MaxUploadBytes.
var ErrFileTooLarge = errors.New("file exceeds maximum allowed size")

// ErrInvalidUploadID is returned when the upload_id does not belong to the
// requesting user (i.e. does not start with "users/<userID>/uploads/").
var ErrInvalidUploadID = errors.New("invalid or unauthorized upload_id")

// ErrUploadMissing is returned by FinalizeUpload when the client skipped the
// PUT (because CreateUpload reported already_exists) but the referenced blob
// is no longer available. The client should retry the full upload flow.
var ErrUploadMissing = errors.New("upload missing: retry the upload")

// ErrUnrecognizedBook is returned when an uploaded file's metadata does not
// match any known book (no ISBN/title+author match and no Open Library result).
// The upload is rejected and the temp object is removed from the bucket.
var ErrUnrecognizedBook = errors.New("book could not be recognized from metadata")

// MaxUploadBytes is the server-side cap on a single raw upload (250 MB).
// Keep in sync with MAX_UPLOAD_BYTES in web/lib/backlog/zipFiles.ts.
const MaxUploadBytes = 250 * 1024 * 1024

// uploadPresignTTL is how long the presigned PUT URL remains valid.
// Generous to handle large files on slow connections.
const uploadPresignTTL = 60 * time.Minute

// magicBytesLen is the number of leading bytes needed to detect file format.
const magicBytesLen = 4

// maxFilenameBytes caps the stored original_filename to avoid overly long values.
const maxFilenameBytes = 255

// booksFolderPrefix is the R2 prefix under which per-book asset folders live.
// Every book's files (epub/pdf/kepub/cover) are stored under
// books/<bookID>/<name>.
const booksFolderPrefix = "books/"

// bookFileKey returns the canonical R2 key for a book file:
//
//	books/<bookID>/<checksum><ext>
func bookFileKey(bookID fmt.Stringer, checksum, ext string) string {
	return booksFolderPrefix + bookID.String() + "/" + checksum + ext
}

// bookCoverKey returns the R2 key used to cache a book's cover image.
func bookCoverKey(bookID fmt.Stringer) string {
	return booksFolderPrefix + bookID.String() + "/cover.jpg"
}

// bookCoverMissingKey returns the R2 key used as a negative-cache marker when
// a book has no cover (or its stored cover URL returns 404).
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

// CreateUpload validates the declared file size, checks for an existing blob
// with the same checksum, and (when the content is new) allocates a storage
// key under the user's uploads/ prefix and returns a short-lived presigned R2
// PUT URL. When alreadyExists is true, uploadID and url are empty and the
// client must skip the PUT then call FinalizeUpload directly.
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

	// A non-empty checksum lets us skip the upload entirely when a canonical
	// blob for this content already exists in the store.
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

// FinalizeUpload processes a file that the client has already PUT directly to
// R2 (or that was skipped because the content already existed). It deduplicates
// by checksum globally across all users and stores a single canonical R2 object
// per unique file content.
func (s *BookService) FinalizeUpload(
	ctx context.Context,
	userID string,
	uploadID string,
	filename string,
	_ string,
	checksum string,
) (*UploadFileResult, error) {
	// Fast path: the content already has a canonical blob in the store.
	existing, err := s.bookFiles.FindByChecksumGlobal(ctx, checksum)
	if err == nil {
		return s.finalizeDuplicate(ctx, userID, uploadID, filename, checksum, existing)
	}
	if !errors.Is(err, database.ErrResourceNotFound) {
		return nil, err
	}

	// Slow path: new content — bytes must be in R2 under the upload key.
	return s.finalizeNew(ctx, userID, uploadID, filename, checksum)
}

// finalizeDuplicate handles an upload where a canonical blob for the checksum
// already exists. It creates (or returns) the calling user's book_files row
// pointing at the existing blob, without transferring any bytes.
func (s *BookService) finalizeDuplicate(
	ctx context.Context,
	userID string,
	uploadID string,
	filename string,
	checksum string,
	existing *models.BookFile,
) (*UploadFileResult, error) {
	// Resolve or create the user's user_book entry for the existing book.
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

	// Return the user's existing row if they already have this exact file.
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

	// Ensure the blob lives under this book's per-book folder. If the existing
	// row was written with a flat (legacy) key, copy it into the folder-based
	// key first. The copy is idempotent — objectStore.Copy overwrites.
	destKey := bookFileKey(existing.BookID, checksum, extForFormat(existing.Format))
	if destKey != existing.StorageKey {
		bgCtx := context.WithoutCancel(ctx)
		copyErr := s.objectStore.Copy(bgCtx, existing.StorageKey, destKey)
		if copyErr != nil {
			return nil, fmt.Errorf("copy to book folder: %w", copyErr)
		}
	}

	// Insert a new row pointing at the per-book folder key.
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

// uploadedFile holds the results of downloading and validating an upload object.
type uploadedFile struct {
	tmp      *os.File
	size     int64
	format   string
	meta     ebookmeta.Metadata
	checksum string
}

// loadUploadedFile streams the R2 object at uploadID to a temp file, validates
// the magic bytes, extracts ebook metadata, and computes the SHA-256 checksum.
// The caller is responsible for closing and removing tmp.
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

// finalizeNew handles an upload where the content is genuinely new: validates
// the bytes, extracts metadata, copies the blob to its canonical content-
// addressed key, and inserts the book_files row.
func (s *BookService) finalizeNew(
	ctx context.Context,
	userID string,
	uploadID string,
	filename string,
	_ string,
) (*UploadFileResult, error) {
	// 1. Ownership: upload_id must be under this user's uploads/ prefix.
	prefix := fmt.Sprintf("users/%s/uploads/", userID)
	if !strings.HasPrefix(uploadID, prefix) {
		return nil, ErrInvalidUploadID
	}

	// 2–6. Download, validate magic bytes, extract metadata, compute checksum.
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

	// 7. Match existing user_book or upsert a new one.
	ub, matchedExisting, err := s.recognizeBook(ctx, userID, uf.meta)
	if err != nil {
		if errors.Is(err, ErrUnrecognizedBook) {
			_ = s.objectStore.Delete(context.WithoutCancel(ctx), uploadID)
		}
		return nil, err
	}

	// 8. Ensure own-digital tag.
	if tagErr := s.ensureTag(ctx, userID, ub.BookID, models.TagOwnDigital); tagErr != nil {
		return nil, tagErr
	}

	// 9. Dedup within (user, book, format) — handles a concurrent finalizeNew.
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

	// 10. Copy to per-book canonical key; delete the temp upload.
	canonicalKey := bookFileKey(ub.BookID, uf.checksum, extForFormat(uf.format))
	bgCtx := context.WithoutCancel(ctx)
	if copyErr := s.objectStore.Copy(bgCtx, uploadID, canonicalKey); copyErr != nil {
		return nil, fmt.Errorf("copy to canonical key: %w", copyErr)
	}
	_ = s.objectStore.Delete(bgCtx, uploadID)

	// 11. Insert book_files row at the canonical key.
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

// cleanupTempUpload deletes a temp upload object if one was uploaded (i.e.
// uploadID is non-empty and owned by userID). Best-effort; errors are ignored.
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

// extForContentType returns a file extension for the given MIME type, falling
// back to the filename extension when the MIME type is not recognised.
func extForContentType(contentType, filename string) string {
	switch contentType {
	case "application/epub+zip":
		return extEPUB
	case "application/pdf":
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

// extForFormat returns the canonical file extension for a known format.
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

// recognizeBook matches meta to an existing user_book or creates a new one.
// Matching is attempted in order from most to least precise:
//  1. ISBN13 exact match
//  2. ISBN10 exact match
//  3. Exact case-insensitive title + first author
//  4. Normalized title + author last-name overlap (strips subtitles, folds
//     diacritics, handles "Last, First" vs "First Last" formatting)
//  5. Open Library search (creates a new library entry, matchedExisting=false)
func (s *BookService) recognizeBook(
	ctx context.Context,
	userID string,
	meta ebookmeta.Metadata,
) (*models.UserBook, bool, error) {
	// 1. Match by ISBN13.
	if meta.ISBN13 != nil {
		ub, err := s.books.FindUserBookByISBN13(ctx, userID, *meta.ISBN13)
		if err == nil {
			return ub, true, nil
		}
		if !errors.Is(err, database.ErrResourceNotFound) {
			return nil, false, err
		}
	}

	// 2. Match by ISBN10.
	if meta.ISBN10 != nil {
		ub, err := s.books.FindUserBookByISBN10(ctx, userID, *meta.ISBN10)
		if err == nil {
			return ub, true, nil
		}
		if !errors.Is(err, database.ErrResourceNotFound) {
			return nil, false, err
		}
	}

	// 3. Exact case-insensitive title + first author.
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

	// 4. Normalized title + author last-name overlap.
	// Fetches the full library once; the list is small relative to the
	// cost of an Open Library HTTP round-trip that would otherwise follow.
	lib, err := s.books.GetLibrary(ctx, userID)
	if err != nil {
		return nil, false, err
	}
	if ub := matchLibraryByMetadata(lib, meta); ub != nil {
		return ub, true, nil
	}

	// 5. Try Open Library when a title is available.
	if ub := s.tryExternalLookup(ctx, userID, meta); ub != nil {
		return ub, false, nil
	}

	// No match — reject the upload.
	return nil, false, ErrUnrecognizedBook
}

// tryExternalLookup searches Open Library and adds the top result to the
// library. Returns nil when there is no title, no results, or the add fails.
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
	results, err := s.SearchExternal(ctx, query)
	if err != nil || len(results) == 0 {
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

// ensureTag adds tag to the user_book if not already present.
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
}

// GetKEPUBStatus reports whether the book has an EPUB or PDF file and the
// status of its derived KEPUB. Used by the Kobo-sync toggle to gate the UI
// and poll conversion progress.
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
	} else if !errors.Is(kepubErr, database.ErrResourceNotFound) {
		return nil, kepubErr
	}

	return result, nil
}

// GetKoboFileFormat returns the file format to serve to the Kobo device for the
// given book. Returns "pdf" when the user has set the kobo-format-pdf tag,
// "kepub" otherwise. Returns ErrResourceNotFound when the user_book does not
// exist.
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
	return s.ensureTag(ctx, userID, bookID, models.TagKoboSync)
}

const presignTTL = 5 * time.Minute

// GetBookFileResult holds the outcome of a successful GetBookFile call.
type GetBookFileResult struct {
	URL       string
	ExpiresAt time.Time
	Format    string
}

// GetBookFile returns a short-lived presigned URL for the book's stored file.
// format is optional; when empty the first ready pdf/epub is returned.
// Returns database.ErrResourceNotFound when no matching file exists (including
// when the file belongs to a different user — callers must not distinguish).
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

// FormatsByUser returns a map of book ID → ready file formats (pdf/epub) for a
// user's entire library in a single query.
func (s *BookService) FormatsByUser(
	ctx context.Context,
	userID string,
) (map[uuid.UUID][]string, error) {
	return s.bookFiles.FormatsByUser(ctx, userID)
}

// RelocateFlatKeyFiles migrates book_files rows that still use the legacy flat
// storage scheme (books/<checksum><ext>) to the per-book folder scheme
// (books/<bookID>/<checksum><ext>). Returns the number of rows migrated.
// Safe to call concurrently; it skips rows that already use the new scheme.
func (s *BookService) RelocateFlatKeyFiles(
	ctx context.Context,
	logger *slog.Logger,
) (int, error) {
	files, err := s.bookFiles.ListWithFlatStorageKey(ctx)
	if err != nil {
		return 0, err
	}

	migrated := 0
	for i := range files {
		f := &files[i]
		if f.Checksum == nil {
			continue
		}
		newKey := bookFileKey(f.BookID, *f.Checksum, extForFormat(f.Format))
		if newKey == f.StorageKey {
			continue // already migrated
		}

		bgCtx := context.WithoutCancel(ctx)
		if copyErr := s.objectStore.Copy(bgCtx, f.StorageKey, newKey); copyErr != nil {
			logger.WarnContext(ctx, "failed to copy file to per-book folder",
				slog.String("id", f.ID.String()),
				slog.String("src", f.StorageKey),
				slog.String("dst", newKey),
				slog.Any("error", copyErr),
			)
			continue
		}

		if updateErr := s.bookFiles.UpdateStorageKey(ctx, f.ID, newKey); updateErr != nil {
			logger.WarnContext(ctx, "failed to update storage_key after copy",
				slog.String("id", f.ID.String()),
				slog.Any("error", updateErr),
			)
			continue
		}

		migrated++
	}

	return migrated, nil
}
