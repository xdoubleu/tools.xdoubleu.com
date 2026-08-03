package services

import (
	"bytes"
	"context"
	"fmt"
	"net/url"
	"strings"

	readability "github.com/go-shiori/go-readability"

	"tools.xdoubleu.com/apps/books/internal/models"
)

// ArticleMeta carries the bibliographic fields passed to the HTML→EPUB
// converter (see conversion_epubbuild.go, shared by PDF-to-EPUB conversion).
type ArticleMeta struct {
	Title   string
	Authors []string
}

// extractedArticle is the readable core of a fetched web page.
type extractedArticle struct {
	Title    string
	Byline   string
	Excerpt  string
	ImageURL string
	HTML     string
}

// extractReadable runs readability extraction over a fetched HTML page.
// finalURL (post-redirect) resolves relative links inside the page.
func extractReadable(finalURL string, body []byte) (*extractedArticle, error) {
	pageURL, err := url.Parse(finalURL)
	if err != nil {
		return nil, fmt.Errorf("%w: bad final url", ErrNoReadableContent)
	}

	article, err := readability.FromReader(bytes.NewReader(body), pageURL)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrNoReadableContent, err)
	}
	if strings.TrimSpace(article.TextContent) == "" ||
		strings.TrimSpace(article.Content) == "" {
		return nil, ErrNoReadableContent
	}

	title := strings.TrimSpace(article.Title)
	if title == "" {
		title = finalURL
	}
	return &extractedArticle{
		Title:    title,
		Byline:   strings.TrimSpace(article.Byline),
		Excerpt:  strings.TrimSpace(article.Excerpt),
		ImageURL: article.Image,
		HTML:     article.Content,
	}, nil
}

// BackfillContentHTML lazily fetches and stores the readability-extracted
// body for a book that has none yet — either ingested before content_html
// existed, or hit a since-fixed extraction bug. Returns the HTML it stored,
// or "" if it still couldn't get any; the caller falls back to the existing
// "no content" UI.
func (s *IngestService) BackfillContentHTML(
	ctx context.Context,
	book *models.Book,
) string {
	if book.SourceURL == nil || *book.SourceURL == "" {
		return ""
	}

	url := *book.SourceURL
	res, err := s.webFetch.Get(
		ctx, url, fetchOptions(maxArticleBytes, articleAccept),
	)
	if err != nil {
		s.logger.WarnContext(ctx, "backfill content fetch failed",
			"bookID", book.ID, "url", url, "error", err)
		return ""
	}
	if !isHTMLContentType(res.ContentType) {
		return ""
	}
	art, err := extractReadable(res.FinalURL, res.Body)
	if err != nil {
		s.logger.WarnContext(ctx, "backfill readability extraction failed",
			"bookID", book.ID, "url", url, "error", err)
		return ""
	}

	if setErr := s.booksRepo.SetBookContentHTML(ctx, book.ID, art.HTML); setErr != nil {
		s.logger.WarnContext(ctx, "failed to store backfilled article content html",
			"bookID", book.ID, "error", setErr)
	}
	return art.HTML
}
