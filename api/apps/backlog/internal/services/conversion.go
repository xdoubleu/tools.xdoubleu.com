package services

import (
	"archive/zip"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"

	"connectrpc.com/connect"
	"github.com/google/uuid"
	kepubpkg "github.com/pgaskin/kepubify/v4/kepub"
	"github.com/xdoubleu/essentia/v4/pkg/database"

	"tools.xdoubleu.com/apps/backlog/internal/models"
	"tools.xdoubleu.com/apps/backlog/internal/repositories"
	"tools.xdoubleu.com/apps/backlog/pkg/objectstore"
)

// EPUBConverter converts raw EPUB bytes into KEPUB bytes.
// The interface exists for test injection; production uses kepubifyConverter.
type EPUBConverter interface {
	Convert(ctx context.Context, epubData []byte) ([]byte, error)
}

// PDFConverter converts a PDF file at inPath to an EPUB file at outPath.
// The interface exists for test injection; production shells out to ebook-convert.
type PDFConverter func(ctx context.Context, inPath, outPath string) error

// ConversionService produces KEPUBs from stored EPUBs or PDFs.
// Callers must use EnsureKEPUB; internal conversion is lazy and idempotent.
type ConversionService struct {
	logger      *slog.Logger
	bookFiles   *repositories.BookFilesRepository
	objectStore objectstore.Client
	converter   EPUBConverter
	convertPDF  PDFConverter
}

// NewConversionService constructs a ConversionService. Pass nil for converter
// or convertPDF to use the default implementations (kepubify and
// calibrePDFConverter respectively).
func NewConversionService(
	logger *slog.Logger,
	bookFiles *repositories.BookFilesRepository,
	objectStore objectstore.Client,
	converter EPUBConverter,
	convertPDF PDFConverter,
) *ConversionService {
	if converter == nil {
		converter = newKepubifyConverter()
	}
	if convertPDF == nil {
		convertPDF = calibrePDFConverter
	}
	return &ConversionService{
		logger:      logger,
		bookFiles:   bookFiles,
		objectStore: objectStore,
		converter:   converter,
		convertPDF:  convertPDF,
	}
}

// EnsureKEPUB returns the existing KEPUB book_files row for (userID, bookID),
// or creates one by converting the stored EPUB or PDF. If neither is stored the
// call returns a FailedPrecondition error.
func (s *ConversionService) EnsureKEPUB(
	ctx context.Context,
	userID string,
	bookID uuid.UUID,
) (*models.BookFile, error) {
	// Return existing KEPUB (covers idempotency / concurrent callers).
	existing, err := s.bookFiles.GetByBookAndFormat(
		ctx, userID, bookID, models.FileFormatKEPUB,
	)
	if err == nil {
		return existing, nil
	}
	if !errors.Is(err, database.ErrResourceNotFound) {
		return nil, err
	}

	// Resolve the source file: prefer EPUB, fall back to PDF.
	sourceFile, sourceFormat, err := s.resolveSourceFile(ctx, userID, bookID)
	if err != nil {
		return nil, err
	}

	// Check whether a canonical KEPUB blob already exists for this source
	// content. If so, insert a row for this user and return immediately; no
	// conversion is needed. Also returns the canonical key for use below.
	shared, canonicalKey, err := s.resolveCanonicalKEPUB(
		ctx,
		userID,
		bookID,
		sourceFile,
	)
	if err != nil {
		return nil, err
	}
	if shared != nil {
		return shared, nil
	}

	// Insert a placeholder row so concurrent callers and the UI can observe the
	// "converting" state via GetByBookAndFormat.
	sourceID := sourceFile.ID
	kepubRow, err := s.bookFiles.Insert(
		ctx,
		models.BookFile{ //nolint:exhaustruct //optional fields not applicable here
			BookID:       bookID,
			UserID:       userID,
			Format:       models.FileFormatKEPUB,
			StorageKey:   "",
			SizeBytes:    0,
			Status:       models.FileStatusConverting,
			SourceFileID: &sourceID,
		},
	)
	if err != nil {
		return nil, err
	}

	epubData, convertErr := s.getEPUBBytes(ctx, sourceFile.StorageKey, sourceFormat)
	if convertErr != nil {
		s.logger.ErrorContext(ctx, "source preparation failed",
			"book_id", bookID,
			"source_file_id", sourceFile.ID,
			"source_format", sourceFormat,
			"err", convertErr,
		)
		_ = s.bookFiles.UpdateStatus(ctx, kepubRow.ID, models.FileStatusFailed)
		return nil, fmt.Errorf("prepare epub source: %w", convertErr)
	}

	kepubData, convertErr := s.converter.Convert(ctx, epubData)
	if convertErr != nil {
		s.logger.ErrorContext(ctx, "kepub conversion failed",
			"book_id", bookID,
			"source_file_id", sourceFile.ID,
			"err", convertErr,
		)
		_ = s.bookFiles.UpdateStatus(ctx, kepubRow.ID, models.FileStatusFailed)
		return nil, fmt.Errorf("convert epub to kepub: %w", convertErr)
	}

	// Use the canonical key when available; fall back to a per-user path for
	// source files without a checksum (e.g. legacy or test-seeded rows).
	key := canonicalKey
	if key == "" {
		key = fmt.Sprintf(
			"users/%s/books/%s/%s.kepub", userID, bookID.String(), kepubRow.ID.String(),
		)
	}
	if putErr := s.objectStore.Put(
		ctx, key, bytes.NewReader(kepubData), int64(len(kepubData)), "application/epub+zip",
	); putErr != nil {
		_ = s.bookFiles.UpdateStatus(ctx, kepubRow.ID, models.FileStatusFailed)
		return nil, fmt.Errorf("store kepub: %w", putErr)
	}

	if updateErr := s.bookFiles.UpdateAfterConversion(
		ctx, kepubRow.ID, key, int64(len(kepubData)),
	); updateErr != nil {
		return nil, updateErr
	}

	kepubRow.StorageKey = key
	kepubRow.SizeBytes = int64(len(kepubData))
	kepubRow.Status = models.FileStatusReady
	return kepubRow, nil
}

// resolveCanonicalKEPUB checks whether a canonical KEPUB blob already exists
// for the given source file's checksum. Returns (row, canonicalKey, nil) where:
//   - row != nil: dedup hit — a ready row was inserted for userID; caller returns it.
//   - row == nil, canonicalKey != "": cache miss — caller should convert and Put
//     the result at canonicalKey.
//   - row == nil, canonicalKey == "": source has no checksum — caller falls back
//     to a per-user storage key.
func (s *ConversionService) resolveCanonicalKEPUB(
	ctx context.Context,
	userID string,
	bookID uuid.UUID,
	sourceFile *models.BookFile,
) (*models.BookFile, string, error) {
	if sourceFile.Checksum == nil || *sourceFile.Checksum == "" {
		return nil, "", nil
	}
	canonicalKey := canonicalKeyPrefix + *sourceFile.Checksum + extKEPUB

	globalRow, err := s.bookFiles.FindByStorageKeyGlobal(ctx, canonicalKey)
	if errors.Is(err, database.ErrResourceNotFound) {
		return nil, canonicalKey, nil // miss: proceed with conversion
	}
	if err != nil {
		return nil, "", err
	}

	// Hit: insert a ready row for this user pointing at the shared canonical blob.
	sourceID := sourceFile.ID
	row, insertErr := s.bookFiles.Insert(
		ctx,
		models.BookFile{ //nolint:exhaustruct //optional fields not applicable here
			BookID:       bookID,
			UserID:       userID,
			Format:       models.FileFormatKEPUB,
			StorageKey:   canonicalKey,
			SizeBytes:    globalRow.SizeBytes,
			Status:       models.FileStatusReady,
			SourceFileID: &sourceID,
		},
	)
	return row, canonicalKey, insertErr
}

// resolveSourceFile finds the best available source file for KEPUB conversion:
// EPUB is preferred; PDF is used as a fallback. Returns FailedPrecondition when
// neither is available.
func (s *ConversionService) resolveSourceFile(
	ctx context.Context,
	userID string,
	bookID uuid.UUID,
) (*models.BookFile, string, error) {
	epubFile, err := s.bookFiles.GetByBookAndFormat(
		ctx, userID, bookID, models.FileFormatEPUB,
	)
	if err == nil {
		return epubFile, models.FileFormatEPUB, nil
	}
	if !errors.Is(err, database.ErrResourceNotFound) {
		return nil, "", err
	}

	pdfFile, err := s.bookFiles.GetByBookAndFormat(
		ctx, userID, bookID, models.FileFormatPDF,
	)
	if err == nil {
		return pdfFile, models.FileFormatPDF, nil
	}
	if !errors.Is(err, database.ErrResourceNotFound) {
		return nil, "", err
	}

	return nil, "", connect.NewError(
		connect.CodeFailedPrecondition,
		errors.New("no EPUB or PDF available for this book"),
	)
}

// getEPUBBytes returns raw EPUB bytes ready for kepubify.
// When the source is an EPUB it downloads it directly.
// When the source is a PDF it downloads to a temp file, calls convertPDF to
// produce a temp EPUB, reads that, then cleans up both temp files.
func (s *ConversionService) getEPUBBytes(
	ctx context.Context,
	storageKey string,
	sourceFormat string,
) ([]byte, error) {
	if sourceFormat == models.FileFormatEPUB {
		return s.downloadBytes(ctx, storageKey)
	}

	// PDF path: download to temp file, convert to temp EPUB, read, clean up.
	pdfTmp, err := os.CreateTemp("", "bookpdf-*.pdf")
	if err != nil {
		return nil, fmt.Errorf("create pdf temp file: %w", err)
	}
	pdfPath := pdfTmp.Name()
	defer func() { _ = os.Remove(pdfPath) }()

	rc, err := s.objectStore.Get(ctx, storageKey)
	if err != nil {
		_ = pdfTmp.Close()
		return nil, fmt.Errorf("download pdf: %w", err)
	}
	if _, copyErr := io.Copy(pdfTmp, rc); copyErr != nil {
		_ = pdfTmp.Close()
		_ = rc.Close()
		return nil, fmt.Errorf("write pdf temp file: %w", copyErr)
	}
	_ = pdfTmp.Close()
	_ = rc.Close()

	epubTmp, err := os.CreateTemp("", "bookepub-*.epub")
	if err != nil {
		return nil, fmt.Errorf("create epub temp file: %w", err)
	}
	epubPath := epubTmp.Name()
	_ = epubTmp.Close()
	defer func() { _ = os.Remove(epubPath) }()

	if convErr := s.convertPDF(ctx, pdfPath, epubPath); convErr != nil {
		return nil, fmt.Errorf("pdf to epub: %w", convErr)
	}

	data, err := os.ReadFile(epubPath)
	if err != nil {
		return nil, fmt.Errorf("read converted epub: %w", err)
	}
	return data, nil
}

// downloadBytes streams an object from the object store into memory.
func (s *ConversionService) downloadBytes(
	ctx context.Context,
	storageKey string,
) ([]byte, error) {
	rc, err := s.objectStore.Get(ctx, storageKey)
	if err != nil {
		return nil, fmt.Errorf("download epub: %w", err)
	}
	defer rc.Close()

	data, err := io.ReadAll(rc)
	if err != nil {
		return nil, fmt.Errorf("read epub: %w", err)
	}
	return data, nil
}

// kepubifyConverter is the production EPUBConverter backed by kepubify.
type kepubifyConverter struct {
	c *kepubpkg.Converter
}

func newKepubifyConverter() EPUBConverter {
	return &kepubifyConverter{c: kepubpkg.NewConverter()}
}

func (k *kepubifyConverter) Convert(
	ctx context.Context,
	epubData []byte,
) ([]byte, error) {
	zr, err := zip.NewReader(bytes.NewReader(epubData), int64(len(epubData)))
	if err != nil {
		return nil, fmt.Errorf("open epub zip: %w", err)
	}

	var buf bytes.Buffer
	if err = k.c.Convert(ctx, &buf, zr); err != nil {
		return nil, fmt.Errorf("kepubify: %w", err)
	}

	return buf.Bytes(), nil
}
