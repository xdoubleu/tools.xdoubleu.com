package services

import (
	"errors"
	"log/slog"

	"tools.xdoubleu.com/apps/books/internal/repositories"
	"tools.xdoubleu.com/apps/books/pkg/webfetch"
)

// ErrNoReadableContent is returned when readability extraction finds no
// article content, or the response is neither HTML nor PDF.
var ErrNoReadableContent = errors.New("no extractable article content")

const (
	// maxArticleBytes caps a fetched article page.
	maxArticleBytes = int64(25 << 20)

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

// IngestService lazily backfills readability-extracted article content for
// library items that have a source URL but no stored content_html.
type IngestService struct {
	logger    *slog.Logger
	booksRepo *repositories.BooksRepository
	webFetch  webfetch.Client
}

// NewIngestService constructs an IngestService.
func NewIngestService(
	logger *slog.Logger,
	repos *repositories.Repositories,
	webFetchClient webfetch.Client,
) *IngestService {
	return &IngestService{
		logger:    logger,
		booksRepo: repos.Books,
		webFetch:  webFetchClient,
	}
}

func isHTMLContentType(ct string) bool {
	return ct == "text/html" || ct == "application/xhtml+xml" || ct == ""
}
