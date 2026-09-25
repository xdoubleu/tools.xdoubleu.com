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

	"tools.xdoubleu.com/apps/books/internal/models"
	"tools.xdoubleu.com/apps/books/internal/repositories"
	"tools.xdoubleu.com/apps/books/pkg/objectstore"
	"tools.xdoubleu.com/internal/database"
)

// EPUBConverter converts raw EPUB bytes into KEPUB bytes.
type EPUBConverter interface {
	Convert(ctx context.Context, epubData []byte) ([]byte, error)
}

// PDFConverter converts the PDF at inPath to an EPUB at outPath. identifier is
// stamped into dc:identifier so regenerated files keep their identity;
// catalogTitle/catalogAuthors override anything derived from the PDF.
type PDFConverter func(
	ctx context.Context, inPath, outPath, identifier, catalogTitle string,
	catalogAuthors []string,
) error

// currentKEPUBConverterVersion: bump by hand whenever the pipeline would
// produce different output; older rows are then regenerated on access.
const currentKEPUBConverterVersion int16 = 12

// IsKEPUBStale reports whether version predates the current pipeline.
func (s *ConversionService) IsKEPUBStale(version int16) bool {
	return version < currentKEPUBConverterVersion
}

// CurrentKEPUBConverterVersion returns the version EnsureKEPUB stamps.
func CurrentKEPUBConverterVersion() int16 {
	return currentKEPUBConverterVersion
}

// ConversionService produces KEPUBs from stored EPUBs or PDFs via EnsureKEPUB.
type ConversionService struct {
	logger      *slog.Logger
	books       *repositories.BooksRepository
	bookFiles   *repositories.BookFilesRepository
	objectStore objectstore.Client
	converter   EPUBConverter
	convertPDF  PDFConverter
}

// NewConversionService constructs a ConversionService; nil converter/convertPDF
// select the defaults.
func NewConversionService(
	logger *slog.Logger,
	books *repositories.BooksRepository,
	bookFiles *repositories.BookFilesRepository,
	objectStore objectstore.Client,
	converter EPUBConverter,
	convertPDF PDFConverter,
) *ConversionService {
	if converter == nil {
		converter = newKepubifyConverter()
	}
	if convertPDF == nil {
		convertPDF = goPDFConverter
	}
	return &ConversionService{
		logger:      logger,
		books:       books,
		bookFiles:   bookFiles,
		objectStore: objectStore,
		converter:   converter,
		convertPDF:  convertPDF,
	}
}

// EnsureKEPUB returns the KEPUB row for (userID, bookID), converting the stored
// EPUB or PDF if needed; FailedPrecondition when neither exists.
func (s *ConversionService) EnsureKEPUB(
	ctx context.Context,
	userID string,
	bookID uuid.UUID,
) (*models.BookFile, error) {
	// A stale existing KEPUB is deleted and regenerated.
	existing, err := s.bookFiles.GetByBookAndFormat(
		ctx, userID, bookID, models.FileFormatKEPUB,
	)
	if err == nil {
		if existing.ConverterVersion >= currentKEPUBConverterVersion {
			return existing, nil
		}
		if delErr := s.bookFiles.Delete(ctx, existing.ID); delErr != nil {
			return nil, delErr
		}
	} else if !errors.Is(err, database.ErrResourceNotFound) {
		return nil, err
	}

	sourceFile, sourceFormat, err := s.resolveSourceFile(ctx, userID, bookID)
	if err != nil {
		return nil, err
	}

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

	// Placeholder row exposes the "converting" state to concurrent callers and the UI.
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

	epubData, convertErr := s.prepareEPUBData(ctx, bookID, sourceFile, sourceFormat)
	if convertErr != nil {
		_ = s.bookFiles.UpdateStatus(ctx, kepubRow.ID, models.FileStatusFailed)
		return nil, convertErr
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

	// Sources without a checksum fall back to a per-user key.
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
		ctx, kepubRow.ID, key, int64(len(kepubData)), currentKEPUBConverterVersion,
	); updateErr != nil {
		return nil, updateErr
	}

	kepubRow.StorageKey = key
	kepubRow.SizeBytes = int64(len(kepubData))
	kepubRow.Status = models.FileStatusReady
	kepubRow.ConverterVersion = currentKEPUBConverterVersion
	return kepubRow, nil
}

// prepareEPUBData prepares EPUB bytes using the catalog title/authors.
func (s *ConversionService) prepareEPUBData(
	ctx context.Context,
	bookID uuid.UUID,
	sourceFile *models.BookFile,
	sourceFormat string,
) ([]byte, error) {
	book, bookErr := s.books.GetBookByID(ctx, bookID)
	if bookErr != nil {
		s.logger.ErrorContext(ctx, "load book metadata failed",
			"book_id", bookID,
			"err", bookErr,
		)
		return nil, fmt.Errorf("load book metadata: %w", bookErr)
	}

	epubData, convertErr := s.getEPUBBytes(
		ctx, sourceFile.StorageKey, sourceFormat, bookID.String(), book.Title,
		book.Authors,
	)
	if convertErr != nil {
		s.logger.ErrorContext(ctx, "source preparation failed",
			"book_id", bookID,
			"source_file_id", sourceFile.ID,
			"source_format", sourceFormat,
			"err", convertErr,
		)
		return nil, fmt.Errorf("prepare epub source: %w", convertErr)
	}
	return epubData, nil
}

// resolveCanonicalKEPUB looks up a shared KEPUB by source checksum. A non-nil
// row is a dedup hit; otherwise convert and Put at canonicalKey, or use a
// per-user key when canonicalKey is "" (no checksum).
func (s *ConversionService) resolveCanonicalKEPUB(
	ctx context.Context,
	userID string,
	bookID uuid.UUID,
	sourceFile *models.BookFile,
) (*models.BookFile, string, error) {
	if sourceFile.Checksum == nil || *sourceFile.Checksum == "" {
		return nil, "", nil
	}
	canonicalKey := bookFileKey(bookID, *sourceFile.Checksum, extKEPUB)

	globalRow, err := s.bookFiles.FindByStorageKeyGlobal(ctx, canonicalKey)
	if errors.Is(err, database.ErrResourceNotFound) {
		return nil, canonicalKey, nil // miss: proceed with conversion
	}
	if err != nil {
		return nil, "", err
	}
	if globalRow.ConverterVersion < currentKEPUBConverterVersion {
		return nil, canonicalKey, nil
	}

	sourceID := sourceFile.ID
	row, insertErr := s.bookFiles.Insert(
		ctx,
		models.BookFile{ //nolint:exhaustruct //optional fields not applicable here
			BookID:           bookID,
			UserID:           userID,
			Format:           models.FileFormatKEPUB,
			StorageKey:       canonicalKey,
			SizeBytes:        globalRow.SizeBytes,
			Status:           models.FileStatusReady,
			SourceFileID:     &sourceID,
			ConverterVersion: globalRow.ConverterVersion,
		},
	)
	return row, canonicalKey, insertErr
}

// resolveSourceFile prefers EPUB, then PDF; FailedPrecondition when neither.
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

// getEPUBBytes returns EPUB bytes for kepubify, converting a PDF source via
// temp files.
func (s *ConversionService) getEPUBBytes(
	ctx context.Context,
	storageKey string,
	sourceFormat string,
	bookID string,
	catalogTitle string,
	catalogAuthors []string,
) ([]byte, error) {
	if sourceFormat == models.FileFormatEPUB {
		return s.downloadBytes(ctx, storageKey)
	}

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

	if convErr := s.convertPDF(
		ctx, pdfPath, epubPath, bookID, catalogTitle, catalogAuthors,
	); convErr != nil {
		return nil, fmt.Errorf("pdf to epub: %w", convErr)
	}

	data, err := os.ReadFile(epubPath)
	if err != nil {
		return nil, fmt.Errorf("read converted epub: %w", err)
	}
	return data, nil
}

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
