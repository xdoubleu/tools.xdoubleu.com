package arxiv

import (
	"context"
	"encoding/xml"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"
)

const (
	apiBaseURL     = "https://export.arxiv.org/api/query"
	requestTimeout = 30 * time.Second
)

// idPattern matches both id styles, with an optional version suffix:
// new-style "2401.12345v2" and old-style "math.GT/0309136v1".
//

var idPattern = regexp.MustCompile(
	`^(?:(\d{4}\.\d{4,5})|([a-z-]+(?:\.[A-Za-z-]+)?/\d{7}))(?:v\d+)?$`,
)

// ParseID extracts a canonical versionless arXiv id from any accepted form:
// arxiv.org/abs/<id>, arxiv.org/pdf/<id>[.pdf], export.arxiv.org variants,
// doi.org/10.48550/arXiv.<id>, or a bare id. Returns ok=false when the input
// is not an arXiv reference.
func ParseID(raw string) (string, bool) {
	candidate := strings.TrimSpace(raw)

	if u, err := url.Parse(candidate); err == nil && u.Host != "" {
		host := strings.TrimPrefix(strings.ToLower(u.Host), "www.")
		path := strings.Trim(u.Path, "/")
		switch host {
		case "arxiv.org", "export.arxiv.org":
			for _, prefix := range []string{"abs/", "pdf/"} {
				if rest, found := strings.CutPrefix(path, prefix); found {
					candidate = strings.TrimSuffix(rest, ".pdf")
				}
			}
			if candidate == strings.TrimSpace(raw) {
				return "", false // arxiv.org URL but not an abs/pdf path
			}
		case "doi.org":
			rest, found := strings.CutPrefix(path, "10.48550/arXiv.")
			if !found {
				return "", false
			}
			candidate = rest
		default:
			return "", false
		}
	}

	m := idPattern.FindStringSubmatch(candidate)
	if m == nil {
		return "", false
	}
	if m[1] != "" {
		return m[1], true
	}
	return m[2], true
}

// AbsURL returns the canonical abstract page URL for an id.
func AbsURL(id string) string {
	return "https://arxiv.org/abs/" + id
}

// PDFURL returns the canonical PDF download URL for an id.
func PDFURL(id string) string {
	return "https://arxiv.org/pdf/" + id
}

type client struct {
	logger  *slog.Logger
	http    *http.Client
	baseURL string
}

// New returns the production Client.
func New(logger *slog.Logger) Client {
	return NewWithBaseURL(logger, apiBaseURL)
}

// NewWithBaseURL returns a Client against a custom API endpoint (tests).
func NewWithBaseURL(logger *slog.Logger, baseURL string) Client {
	return &client{
		logger:  logger,
		http:    &http.Client{Timeout: requestTimeout},
		baseURL: baseURL,
	}
}

// atomFeed mirrors the subset of the arXiv Atom response we consume.
type atomFeed struct {
	Entries []atomEntry `xml:"entry"`
}

type atomEntry struct {
	ID        string   `xml:"id"`
	Title     string   `xml:"title"`
	Summary   string   `xml:"summary"`
	Published string   `xml:"published"`
	Authors   []author `xml:"author"`
}

type author struct {
	Name string `xml:"name"`
}

func (c *client) GetByID(ctx context.Context, id string) (*Paper, error) {
	reqURL := c.baseURL + "?id_list=" + url.QueryEscape(id) + "&max_results=1"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		return nil, err
	}

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("arxiv: unexpected status %d", resp.StatusCode)
	}

	var feed atomFeed
	if err = xml.NewDecoder(resp.Body).Decode(&feed); err != nil {
		return nil, fmt.Errorf("arxiv: decode response: %w", err)
	}

	// Unknown ids come back either as an empty feed or as a single error
	// entry whose id points at api/errors.
	if len(feed.Entries) == 0 {
		return nil, ErrNotFound
	}
	entry := feed.Entries[0]
	if strings.Contains(entry.ID, "api/errors") || entry.Title == "" {
		return nil, ErrNotFound
	}

	paper := &Paper{
		ID:        id,
		Title:     collapseWhitespace(entry.Title),
		Abstract:  collapseWhitespace(entry.Summary),
		Published: time.Time{},
		PDFURL:    PDFURL(id),
		AbsURL:    AbsURL(id),
		Authors:   nil,
	}
	for _, a := range entry.Authors {
		if name := strings.TrimSpace(a.Name); name != "" {
			paper.Authors = append(paper.Authors, name)
		}
	}
	if t, parseErr := time.Parse(time.RFC3339, entry.Published); parseErr == nil {
		paper.Published = t
	}
	return paper, nil
}

// collapseWhitespace flattens the newline-wrapped text arXiv returns into a
// single-spaced string.
func collapseWhitespace(s string) string {
	return strings.Join(strings.Fields(s), " ")
}
