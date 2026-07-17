package services

import (
	"bytes"
	"context"
	"fmt"
	"html"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	readability "github.com/go-shiori/go-readability"

	"tools.xdoubleu.com/apps/reading/internal/models"
)

// ArticleMeta carries the bibliographic fields passed to the HTML→EPUB
// converter.
type ArticleMeta struct {
	Title   string
	Authors []string
}

// HTMLConverter converts the HTML file at inPath into an EPUB at outPath.
// Mirrors PDFConverter for test injection; production shells out to Calibre.
type HTMLConverter func(
	ctx context.Context, inPath, outPath string, meta ArticleMeta,
) error

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

// ArticleContent is the input to IngestArticleContent: everything needed to
// create the catalog entry and build its EPUB.
type ArticleContent struct {
	// SourceURL is the canonical dedup URL stored on the catalog row.
	SourceURL string
	// BaseURL resolves relative image links inside HTML (usually the
	// post-redirect page URL, or the feed item link).
	BaseURL  string
	Category string
	Title    string
	Byline   string
	Excerpt  string
	CoverURL string
	HTML     string
}

// IngestArticleContent is the shared "have HTML → library entry + EPUB" tail
// used by both AddByURL and feed polling.
func (s *IngestService) IngestArticleContent(
	ctx context.Context,
	userID string,
	content ArticleContent,
) (*models.UserBook, error) {
	var authors []string
	if content.Byline != "" {
		authors = []string{content.Byline}
	}

	sourceURL := content.SourceURL
	book := models.Book{ //nolint:exhaustruct // catalog metadata only
		Title:     content.Title,
		Authors:   authors,
		Category:  content.Category,
		SourceURL: &sourceURL,
	}
	if content.Excerpt != "" {
		book.Description = &content.Excerpt
	}
	if content.CoverURL != "" {
		book.CoverURL = &content.CoverURL
	}

	saved, err := s.booksRepo.UpsertBookBySourceURL(ctx, book)
	if err != nil {
		return nil, err
	}

	if content.CoverURL != "" {
		if cacheErr := s.books.cacheCoverFromURL(
			ctx, saved.ID, content.CoverURL,
		); cacheErr != nil {
			s.logger.WarnContext(ctx, "failed to cache article cover",
				"bookID", saved.ID, "error", cacheErr)
		}
	}

	if _, err = s.ensureUserBook(ctx, userID, saved.ID); err != nil {
		return nil, err
	}

	art := &extractedArticle{
		Title:    content.Title,
		Byline:   content.Byline,
		Excerpt:  content.Excerpt,
		ImageURL: content.CoverURL,
		HTML:     content.HTML,
	}
	epub, err := s.buildArticleEPUB(ctx, art, content.BaseURL)
	if err != nil {
		return nil, err
	}
	if err = s.storeReadyFile(
		ctx, userID, saved.ID, epub, models.FileFormatEPUB,
	); err != nil {
		return nil, err
	}

	return s.booksRepo.GetUserBook(ctx, userID, saved.ID)
}

// buildArticleEPUB assembles a standalone HTML document (images downloaded
// and rewritten to local files) in a temp dir and converts it to EPUB.
func (s *IngestService) buildArticleEPUB(
	ctx context.Context,
	art *extractedArticle,
	baseURL string,
) ([]byte, error) {
	dir, err := os.MkdirTemp("", "article-epub-*")
	if err != nil {
		return nil, fmt.Errorf("create temp dir: %w", err)
	}
	defer func() { _ = os.RemoveAll(dir) }()

	localHTML := s.localizeImages(ctx, dir, art.HTML, baseURL)

	doc := articleHTMLDocument(art, localHTML)
	htmlPath := filepath.Join(dir, "index.html")
	//nolint:gosec // temp file, no sensitive contents
	if err = os.WriteFile(htmlPath, []byte(doc), 0o644); err != nil {
		return nil, fmt.Errorf("write article html: %w", err)
	}

	epubPath := filepath.Join(dir, "article.epub")
	meta := ArticleMeta{Title: art.Title, Authors: nil}
	if art.Byline != "" {
		meta.Authors = []string{art.Byline}
	}
	if err = s.htmlConvert(ctx, htmlPath, epubPath, meta); err != nil {
		return nil, fmt.Errorf("html to epub: %w", err)
	}

	epub, err := os.ReadFile(epubPath)
	if err != nil {
		return nil, fmt.Errorf("read converted epub: %w", err)
	}
	return epub, nil
}

// articleHTMLDocument wraps extracted article HTML in a minimal standalone
// document with title and byline.
func articleHTMLDocument(art *extractedArticle, bodyHTML string) string {
	var b strings.Builder
	b.WriteString("<!DOCTYPE html>\n<html>\n<head>\n")
	b.WriteString(`<meta charset="utf-8">` + "\n")
	b.WriteString("<title>" + html.EscapeString(art.Title) + "</title>\n")
	b.WriteString("</head>\n<body>\n")
	b.WriteString("<h1>" + html.EscapeString(art.Title) + "</h1>\n")
	if art.Byline != "" {
		b.WriteString("<p><em>" + html.EscapeString(art.Byline) + "</em></p>\n")
	}
	b.WriteString(bodyHTML)
	b.WriteString("\n</body>\n</html>\n")
	return b.String()
}
