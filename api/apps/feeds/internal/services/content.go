package services

import (
	"bytes"
	"errors"
	"fmt"
	"net/url"
	"strings"

	readability "github.com/go-shiori/go-readability"

	"tools.xdoubleu.com/apps/feeds/pkg/webfetch"
)

// ErrUnsupportedURL is returned for URLs that are not http/https.
var ErrUnsupportedURL = errors.New("url scheme not supported")

// ErrNoReadableContent is returned when readability extraction finds no
// article content.
var ErrNoReadableContent = errors.New("no extractable article content")

// maxArticleBytes caps a fetched article page.
const maxArticleBytes = int64(25 << 20)

// fetchOptions builds one-shot download options (no conditional GET).
func fetchOptions(maxBytes int64, accept string) webfetch.Options {
	return webfetch.Options{
		ETag:         "",
		LastModified: "",
		MaxBytes:     maxBytes,
		Accept:       accept,
	}
}

func isHTMLContentType(ct string) bool {
	return ct == "text/html" || ct == "application/xhtml+xml" || ct == ""
}

// canonicalURL normalizes a URL for use as a dedup key: lowercases scheme and
// host, drops the fragment and any utm_* tracking query params.
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

// extractedArticle is the readable core of a fetched web page.
type extractedArticle struct {
	Title string
	HTML  string
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
	return &extractedArticle{Title: title, HTML: article.Content}, nil
}
