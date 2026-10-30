package conversion

import (
	"archive/zip"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"

	"github.com/google/uuid"
	kepubpkg "github.com/pgaskin/kepubify/v4/kepub"

	"tools.xdoubleu.com/apps/books/internal/models"
	"tools.xdoubleu.com/apps/books/internal/repositories"
	"tools.xdoubleu.com/apps/books/pkg/ebooks"
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
const currentKEPUBConverterVersion int16 = 15

// ErrNoConvertibleSource means the book has neither an EPUB nor a PDF.
var ErrNoConvertibleSource = errors.New("no EPUB or PDF available for this book")

// IsKEPUBStale reports whether version predates the current pipeline.
func (s *Service) IsKEPUBStale(version int16) bool {
	return version < currentKEPUBConverterVersion
}

// CurrentKEPUBConverterVersion returns the version EnsureKEPUB stamps.
func CurrentKEPUBConverterVersion() int16 {
	return currentKEPUBConverterVersion
}

// Service produces KEPUBs from stored EPUBs or PDFs via EnsureKEPUB.
type Service struct {
	logger      *slog.Logger
	books       *repositories.BooksRepository
	bookFiles   *repositories.BookFilesRepository
	objectStore objectstore.Client
	converter   EPUBConverter
	convertPDF  PDFConverter
}

// NewService constructs a Service; nil converter/convertPDF
// select the defaults.
func NewService(
	logger *slog.Logger,
	books *repositories.BooksRepository,
	bookFiles *repositories.BookFilesRepository,
	objectStore objectstore.Client,
	converter EPUBConverter,
	convertPDF PDFConverter,
) *Service {
	if converter == nil {
		converter = NewKepubifyConverter()
	}
	if convertPDF == nil {
		convertPDF = GoPDFConverter
	}
	return &Service{
		logger:      logger,
		books:       books,
		bookFiles:   bookFiles,
		objectStore: objectStore,
		converter:   converter,
		convertPDF:  convertPDF,
	}
}

// EnsureKEPUB returns the KEPUB row for (userID, bookID), converting the stored
// EPUB or PDF if needed, or the converting row of a conversion already
// running; ErrNoConvertibleSource when neither source exists.
func (s *Service) EnsureKEPUB(
	ctx context.Context,
	userID string,
	bookID uuid.UUID,
) (*models.BookFile, error) {
	row, err := s.ensureKEPUB(ctx, userID, bookID)
	if errors.Is(err, repositories.ErrBookFileReplaced) {
		// A concurrent call already replaced the stale row and owns the conversion.
		return s.bookFiles.GetByBookAndFormat(
			ctx, userID, bookID, models.FileFormatKEPUB,
		)
	}
	return row, err
}

func (s *Service) ensureKEPUB(
	ctx context.Context,
	userID string,
	bookID uuid.UUID,
) (*models.BookFile, error) {
	existing, err := s.bookFiles.GetByBookAndFormat(
		ctx, userID, bookID, models.FileFormatKEPUB,
	)
	// A stale KEPUB stays until its replacement row is inserted.
	var staleID *uuid.UUID
	if err == nil {
		if existing.Status == models.FileStatusConverting ||
			existing.ConverterVersion >= currentKEPUBConverterVersion {
			return existing, nil
		}
		staleID = &existing.ID
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
		staleID,
	)
	if err != nil {
		return nil, err
	}
	if shared != nil {
		return shared, nil
	}

	// Placeholder row exposes the "converting" state to concurrent callers and the UI.
	sourceID := sourceFile.ID
	kepubRow, err := s.insertKEPUB(
		ctx,
		staleID,
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

// insertKEPUB inserts f, replacing staleID atomically when set.
func (s *Service) insertKEPUB(
	ctx context.Context,
	staleID *uuid.UUID,
	f models.BookFile,
) (*models.BookFile, error) {
	if staleID == nil {
		return s.bookFiles.Insert(ctx, f)
	}
	return s.bookFiles.Replace(ctx, *staleID, f)
}

// prepareEPUBData prepares EPUB bytes using the catalog title/authors.
func (s *Service) prepareEPUBData(
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
// per-user key when canonicalKey is "" (no checksum). A hit replaces staleID.
func (s *Service) resolveCanonicalKEPUB(
	ctx context.Context,
	userID string,
	bookID uuid.UUID,
	sourceFile *models.BookFile,
	staleID *uuid.UUID,
) (*models.BookFile, string, error) {
	if sourceFile.Checksum == nil || *sourceFile.Checksum == "" {
		return nil, "", nil
	}
	canonicalKey := ebooks.BookFileKey(bookID, *sourceFile.Checksum, ebooks.ExtKEPUB)

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
	row, insertErr := s.insertKEPUB(
		ctx,
		staleID,
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

// resolveSourceFile prefers EPUB, then PDF.
func (s *Service) resolveSourceFile(
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

	return nil, "", ErrNoConvertibleSource
}

// getEPUBBytes returns EPUB bytes for kepubify, converting a PDF source via
// temp files.
func (s *Service) getEPUBBytes(
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

func (s *Service) downloadBytes(
	ctx context.Context,
	storageKey string,
) ([]byte, error) {
	rc, err := s.objectStore.Get(ctx, storageKey)
	if err != nil {
		return nil, fmt.Errorf("download epub: %w", err)
	}
	defer rc.Close()

	data, err := io.ReadAll(io.LimitReader(rc, MaxConversionInputBytes+1))
	if err != nil {
		return nil, fmt.Errorf("read epub: %w", err)
	}
	if len(data) > MaxConversionInputBytes {
		return nil, fmt.Errorf("epub exceeds %d bytes", MaxConversionInputBytes)
	}
	return data, nil
}

const (
	// MaxConversionInputBytes leaves room for EPUBs built from large PDFs.
	MaxConversionInputBytes = 2 * ebooks.MaxUploadBytes
	// MaxEPUBUncompressedBytes bounds what kepubify inflates into memory;
	// archive/zip refuses to read past an entry's declared size, so checking
	// the declared sizes is enough to stop a zip bomb.
	MaxEPUBUncompressedBytes = 1 << 30
)

// CheckEPUBSize rejects an archive whose entries inflate past
// MaxEPUBUncompressedBytes.
func CheckEPUBSize(zr *zip.Reader) error {
	var total uint64
	for _, f := range zr.File {
		total += f.UncompressedSize64
		if total > MaxEPUBUncompressedBytes {
			return fmt.Errorf("epub inflates past %d bytes", MaxEPUBUncompressedBytes)
		}
	}
	return nil
}

type kepubifyConverter struct {
	c *kepubpkg.Converter
}

func NewKepubifyConverter() EPUBConverter {
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
	if err = CheckEPUBSize(zr); err != nil {
		return nil, err
	}

	var buf bytes.Buffer
	if err = k.c.Convert(ctx, &buf, zr); err != nil {
		return nil, fmt.Errorf("kepubify: %w", err)
	}

	return buf.Bytes(), nil
}
