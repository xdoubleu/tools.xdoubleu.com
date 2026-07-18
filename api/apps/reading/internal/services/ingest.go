package services

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"log/slog"
	"net/url"
	"strings"

	"github.com/google/uuid"
	"github.com/xdoubleu/essentia/v4/pkg/database"

	"tools.xdoubleu.com/apps/reading/internal/models"
	"tools.xdoubleu.com/apps/reading/internal/repositories"
	"tools.xdoubleu.com/apps/reading/pkg/arxiv"
	"tools.xdoubleu.com/apps/reading/pkg/ebookmeta"
	"tools.xdoubleu.com/apps/reading/pkg/objectstore"
	"tools.xdoubleu.com/apps/reading/pkg/webfetch"
)

var (
	// ErrUnsupportedURL is returned for URLs that are not http/https.
	ErrUnsupportedURL = errors.New("url scheme not supported")
	// ErrNoReadableContent is returned when readability extraction finds no
	// article content, or the response is neither HTML nor PDF.
	ErrNoReadableContent = errors.New("no extractable article content")
	// ErrNotAPDF is returned when a downloaded "PDF" fails the magic-byte
	// check (e.g. an HTML error page served with 200).
	ErrNotAPDF = errors.New("downloaded file is not a PDF")
)

const (
	// maxArticleBytes caps a fetched article page.
	maxArticleBytes = int64(25 << 20)
	// maxPaperPDFBytes caps a downloaded paper PDF.
	maxPaperPDFBytes = int64(100 << 20)

	contentTypePDF = "application/pdf"
	// articleAccept asks for HTML first but tolerates directly-linked PDFs.
	articleAccept = "text/html,application/xhtml+xml,application/pdf"
)

// fetchOptions builds one-shot download options (no conditional GET).
func fetchOptions(maxBytes int64, accept string) webfetch.Options {
	return webfetch.Options{
		ETag:         "",
		LastModified: "",
		MaxBytes:     maxBytes,
		Accept:       accept,
	}
}

// IngestService turns pasted URLs and feed items into library entries with
// stored files: arXiv papers become PDF book_files (converted to KEPUB by the
// existing lazy pipeline), web articles are readability-extracted and built
// into EPUBs.
type IngestService struct {
	logger      *slog.Logger
	books       *BookService
	booksRepo   *repositories.BooksRepository
	bookFiles   *repositories.BookFilesRepository
	objectStore objectstore.Client
	webFetch    webfetch.Client
	arxiv       arxiv.Client
	htmlConvert HTMLConverter
}

// NewIngestService constructs an IngestService. Pass nil for htmlConvert to
// use the default Calibre-backed converter.
func NewIngestService(
	logger *slog.Logger,
	books *BookService,
	repos *repositories.Repositories,
	objectStore objectstore.Client,
	webFetchClient webfetch.Client,
	arxivClient arxiv.Client,
	htmlConvert HTMLConverter,
) *IngestService {
	if htmlConvert == nil {
		htmlConvert = calibreHTMLConverter
	}
	return &IngestService{
		logger:      logger,
		books:       books,
		booksRepo:   repos.Books,
		bookFiles:   repos.BookFiles,
		objectStore: objectStore,
		webFetch:    webFetchClient,
		arxiv:       arxivClient,
		htmlConvert: htmlConvert,
	}
}

// AddByURLResult is the outcome of AddByURL.
type AddByURLResult struct {
	UserBook *models.UserBook
	// AlreadyInLibrary is true when the caller already had this item; the
	// call is then a no-op apart from rebuilding a missing file.
	AlreadyInLibrary bool
}

// AddByURL is the manual-ingest entry point. categoryOverride is "" (auto),
// models.CategoryPaper, or models.CategoryArticle.
func (s *IngestService) AddByURL(
	ctx context.Context,
	userID, rawURL, categoryOverride string,
) (*AddByURLResult, error) {
	if id, ok := arxiv.ParseID(rawURL); ok &&
		categoryOverride != models.CategoryArticle {
		return s.addPaper(ctx, userID, id)
	}

	canonical, err := canonicalURL(rawURL)
	if err != nil {
		return nil, err
	}
	category := models.CategoryArticle
	if categoryOverride != "" {
		category = categoryOverride
	}
	return s.addWebItem(ctx, userID, canonical, category)
}

// addPaper ingests an arXiv paper: metadata from the API, the PDF stored as
// a ready book_file (KEPUB conversion stays lazy, via the PDF path).
func (s *IngestService) addPaper(
	ctx context.Context,
	userID, arxivID string,
) (*AddByURLResult, error) {
	paper, err := s.arxiv.GetByID(ctx, arxivID)
	if err != nil {
		return nil, err
	}

	abs := paper.AbsURL
	//nolint:exhaustruct // catalog metadata only; the rest is DB-owned
	book, err := s.booksRepo.UpsertBookBySourceURL(ctx, models.Book{
		Title:       paper.Title,
		Authors:     paper.Authors,
		Description: &paper.Abstract,
		Category:    models.CategoryPaper,
		SourceURL:   &abs,
	})
	if err != nil {
		return nil, err
	}

	existed, err := s.ensureUserBook(ctx, userID, book.ID)
	if err != nil {
		return nil, err
	}

	if err = s.ensurePaperPDF(ctx, userID, book.ID, paper.PDFURL); err != nil {
		return nil, err
	}

	ub, err := s.booksRepo.GetUserBook(ctx, userID, book.ID)
	if err != nil {
		return nil, err
	}
	return &AddByURLResult{UserBook: ub, AlreadyInLibrary: existed}, nil
}

// ensurePaperPDF downloads and stores the paper's PDF unless the user
// already has a ready PDF for the book.
func (s *IngestService) ensurePaperPDF(
	ctx context.Context,
	userID string,
	bookID uuid.UUID,
	pdfURL string,
) error {
	_, err := s.bookFiles.GetByBookAndFormat(
		ctx, userID, bookID, models.FileFormatPDF,
	)
	if err == nil {
		return nil
	}
	if !errors.Is(err, database.ErrResourceNotFound) {
		return err
	}

	res, err := s.webFetch.Get(
		ctx, pdfURL, fetchOptions(maxPaperPDFBytes, contentTypePDF),
	)
	if err != nil {
		return fmt.Errorf("download pdf: %w", err)
	}
	if ebookmeta.DetectFormatFromMagic(res.Body) != models.FileFormatPDF {
		return ErrNotAPDF
	}

	return s.storeReadyFile(ctx, userID, bookID, res.Body, models.FileFormatPDF)
}

// addWebItem ingests a non-arXiv URL: PDFs are stored directly, HTML pages
// are readability-extracted and built into an EPUB.
func (s *IngestService) addWebItem(
	ctx context.Context,
	userID, canonical, category string,
) (*AddByURLResult, error) {
	// Dedup: a catalog row for this URL may already exist (this user or
	// another). Attach the caller and rebuild a missing file instead of
	// re-creating anything.
	if book, err := s.booksRepo.GetBookBySourceURL(ctx, canonical); err == nil {
		return s.attachExisting(ctx, userID, book, canonical)
	} else if !errors.Is(err, database.ErrResourceNotFound) {
		return nil, err
	}

	res, err := s.webFetch.Get(
		ctx, canonical, fetchOptions(maxArticleBytes, articleAccept),
	)
	if err != nil {
		return nil, err
	}

	switch {
	case res.ContentType == contentTypePDF:
		return s.addFetchedPDF(ctx, userID, canonical, category, res.Body)
	case isHTMLContentType(res.ContentType):
		art, extractErr := extractReadable(res.FinalURL, res.Body)
		if extractErr != nil {
			return nil, extractErr
		}
		ub, ingestErr := s.IngestArticleContent(ctx, userID, ArticleContent{
			SourceURL: canonical,
			BaseURL:   res.FinalURL,
			Category:  category,
			Title:     art.Title,
			Byline:    art.Byline,
			Excerpt:   art.Excerpt,
			CoverURL:  art.ImageURL,
			HTML:      art.HTML,
		})
		if ingestErr != nil {
			return nil, ingestErr
		}
		return &AddByURLResult{UserBook: ub, AlreadyInLibrary: false}, nil
	default:
		return nil, fmt.Errorf(
			"%w: content type %q", ErrNoReadableContent, res.ContentType,
		)
	}
}

// addFetchedPDF stores a directly-linked PDF as a library item.
func (s *IngestService) addFetchedPDF(
	ctx context.Context,
	userID, canonical, category string,
	data []byte,
) (*AddByURLResult, error) {
	if ebookmeta.DetectFormatFromMagic(data) != models.FileFormatPDF {
		return nil, ErrNotAPDF
	}

	title := titleFromPDF(data, canonical)
	//nolint:exhaustruct // catalog metadata only; the rest is DB-owned
	book, err := s.booksRepo.UpsertBookBySourceURL(ctx, models.Book{
		Title:     title,
		Authors:   []string{},
		Category:  category,
		SourceURL: &canonical,
	})
	if err != nil {
		return nil, err
	}
	if _, err = s.ensureUserBook(ctx, userID, book.ID); err != nil {
		return nil, err
	}
	if err = s.storeReadyFile(
		ctx, userID, book.ID, data, models.FileFormatPDF,
	); err != nil {
		return nil, err
	}

	ub, err := s.booksRepo.GetUserBook(ctx, userID, book.ID)
	if err != nil {
		return nil, err
	}
	return &AddByURLResult{UserBook: ub, AlreadyInLibrary: false}, nil
}

// attachExisting handles pasting a URL whose catalog row already exists:
// ensure the caller has a user_book, and rebuild the stored file if none is
// ready (also the retry path for failed feed-item ingests).
func (s *IngestService) attachExisting(
	ctx context.Context,
	userID string,
	book *models.Book,
	canonical string,
) (*AddByURLResult, error) {
	existed, err := s.ensureUserBook(ctx, userID, book.ID)
	if err != nil {
		return nil, err
	}

	if !s.hasReadyFile(ctx, userID, book.ID) {
		if rebuildErr := s.rebuildFile(
			ctx, userID, book, canonical,
		); rebuildErr != nil {
			return nil, rebuildErr
		}
	}

	ub, err := s.booksRepo.GetUserBook(ctx, userID, book.ID)
	if err != nil {
		return nil, err
	}
	return &AddByURLResult{UserBook: ub, AlreadyInLibrary: existed}, nil
}

func (s *IngestService) hasReadyFile(
	ctx context.Context,
	userID string,
	bookID uuid.UUID,
) bool {
	for _, format := range []string{models.FileFormatEPUB, models.FileFormatPDF} {
		if _, err := s.bookFiles.GetByBookAndFormat(
			ctx, userID, bookID, format,
		); err == nil {
			return true
		}
	}
	return false
}

// rebuildFile re-fetches the item's source and stores a fresh file.
func (s *IngestService) rebuildFile(
	ctx context.Context,
	userID string,
	book *models.Book,
	canonical string,
) error {
	if book.Category == models.CategoryPaper {
		if id, ok := arxiv.ParseID(canonical); ok {
			return s.ensurePaperPDF(ctx, userID, book.ID, arxiv.PDFURL(id))
		}
	}

	res, err := s.webFetch.Get(
		ctx, canonical, fetchOptions(maxArticleBytes, articleAccept),
	)
	if err != nil {
		return err
	}
	if res.ContentType == contentTypePDF {
		if ebookmeta.DetectFormatFromMagic(res.Body) != models.FileFormatPDF {
			return ErrNotAPDF
		}
		return s.storeReadyFile(
			ctx, userID, book.ID, res.Body, models.FileFormatPDF,
		)
	}
	if !isHTMLContentType(res.ContentType) {
		return fmt.Errorf(
			"%w: content type %q", ErrNoReadableContent, res.ContentType,
		)
	}

	art, err := extractReadable(res.FinalURL, res.Body)
	if err != nil {
		return err
	}
	epub, err := s.buildArticleEPUB(ctx, art, res.FinalURL)
	if err != nil {
		return err
	}
	return s.storeReadyFile(ctx, userID, book.ID, epub, models.FileFormatEPUB)
}

// ensureUserBook adds the book to the user's library with StatusToRead when
// missing. Returns whether the user already had it.
func (s *IngestService) ensureUserBook(
	ctx context.Context,
	userID string,
	bookID uuid.UUID,
) (bool, error) {
	_, err := s.booksRepo.GetUserBook(ctx, userID, bookID)
	if err == nil {
		return true, nil
	}
	if !errors.Is(err, database.ErrResourceNotFound) {
		return false, err
	}

	ub := models.UserBook{ //nolint:exhaustruct // optional fields
		UserID:         userID,
		BookID:         bookID,
		Status:         models.StatusToRead,
		Tags:           []string{},
		ShelfPositions: map[string]int{},
	}
	if err = s.booksRepo.UpsertUserBook(ctx, ub); err != nil {
		return false, err
	}
	return false, nil
}

// storeReadyFile checksums data, uploads it to the canonical R2 key, and
// inserts a ready book_files row. An existing row for (user, book, format)
// is kept as-is.
func (s *IngestService) storeReadyFile(
	ctx context.Context,
	userID string,
	bookID uuid.UUID,
	data []byte,
	format string,
) error {
	_, err := s.bookFiles.GetByBookAndFormat(ctx, userID, bookID, format)
	if err == nil {
		return nil
	}
	if !errors.Is(err, database.ErrResourceNotFound) {
		return err
	}

	sum := sha256.Sum256(data)
	checksum := hex.EncodeToString(sum[:])
	ext := "." + format
	key := bookFileKey(bookID, checksum, ext)

	contentType := "application/epub+zip"
	if format == models.FileFormatPDF {
		contentType = contentTypePDF
	}
	if putErr := s.objectStore.Put(
		ctx, key, bytes.NewReader(data), int64(len(data)), contentType,
	); putErr != nil {
		return fmt.Errorf("store %s: %w", format, putErr)
	}

	//nolint:exhaustruct // optional fields not applicable here
	_, err = s.bookFiles.Insert(ctx, models.BookFile{
		BookID:     bookID,
		UserID:     userID,
		Format:     format,
		StorageKey: key,
		SizeBytes:  int64(len(data)),
		Checksum:   &checksum,
		Status:     models.FileStatusReady,
	})
	return err
}

// canonicalURL normalizes a pasted URL: http/https only, lowercased
// scheme+host, fragment dropped, utm_* tracking params stripped.
func canonicalURL(raw string) (string, error) {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil {
		return "", fmt.Errorf("%w: %s", ErrUnsupportedURL, raw)
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return "", fmt.Errorf("%w: %q", ErrUnsupportedURL, u.Scheme)
	}
	u.Scheme = strings.ToLower(u.Scheme)
	u.Host = strings.ToLower(u.Host)
	u.Fragment = ""

	q := u.Query()
	for param := range q {
		if strings.HasPrefix(strings.ToLower(param), "utm_") {
			q.Del(param)
		}
	}
	u.RawQuery = q.Encode()
	return u.String(), nil
}

func isHTMLContentType(ct string) bool {
	return ct == "text/html" || ct == "application/xhtml+xml" || ct == ""
}

// titleFromPDF extracts a display title for a directly-linked PDF: embedded
// metadata title when present, else the URL's last path segment.
func titleFromPDF(data []byte, canonical string) string {
	meta, err := ebookmeta.Extract(
		ebookmeta.FormatPDF, bytes.NewReader(data), int64(len(data)),
	)
	if err == nil && strings.TrimSpace(meta.Title) != "" {
		return strings.TrimSpace(meta.Title)
	}
	if u, parseErr := url.Parse(canonical); parseErr == nil {
		segments := strings.Split(strings.Trim(u.Path, "/"), "/")
		if last := segments[len(segments)-1]; last != "" {
			return strings.TrimSuffix(last, ".pdf")
		}
	}
	return canonical
}
