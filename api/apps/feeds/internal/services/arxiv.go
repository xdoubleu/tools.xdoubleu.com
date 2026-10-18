package services

import (
	"context"
	"html"
	"net/url"
	"regexp"
	"strings"
	"time"

	"github.com/mmcdole/gofeed"

	"tools.xdoubleu.com/apps/feeds/internal/models"
)

// arxivQuietMinGap floors the quiet check for arXiv feeds, which announce
// nothing over weekends, holidays and the year-end break.
const arxivQuietMinGap = 14 * 24 * time.Hour

// arxivMinRequestGap is arXiv's requested spacing between API requests.
const arxivMinRequestGap = 3 * time.Second

// arxivIDPattern matches a whole arXiv abs URL or OAI identifier and
// captures its new-style (2410.01234) or old-style (math.GT/0309136) id,
// without the version suffix.
var arxivIDPattern = regexp.MustCompile(
	`(?i)^(?:https?://(?:www\.|export\.)?arxiv\.org/abs/|oai:arxiv\.org:)` +
		`(\d{4}\.\d{4,5}|[a-z-]+(?:\.[a-z]{2})?/\d{7})(?:v\d+)?$`,
)

// isArxivFeed reports whether feedURL is an arXiv API query or RSS feed.
func isArxivFeed(feedURL string) bool {
	u, err := url.Parse(feedURL)
	if err != nil {
		return false
	}
	host := strings.ToLower(u.Hostname())
	return host == "export.arxiv.org" || host == "rss.arxiv.org"
}

// arxivID returns the version-less arXiv id from the item's GUID or link,
// or "" when neither carries one.
func arxivID(item *gofeed.Item) string {
	for _, s := range []string{item.GUID, item.Link} {
		if m := arxivIDPattern.FindStringSubmatch(s); m != nil {
			return m[1]
		}
	}
	return ""
}

// arxivAbsURL is the paper's version-less abstract page, used as both GUID
// and source URL so a revised paper never resurfaces.
func arxivAbsURL(id string) string {
	return "https://arxiv.org/abs/" + id
}

// isArxivReplacement reports an RSS announcement of a revised paper.
func isArxivReplacement(item *gofeed.Item) bool {
	types := item.Extensions["arxiv"]["announce_type"]
	if len(types) == 0 {
		return false
	}
	t := strings.TrimSpace(types[0].Value)
	return t == "replace" || t == "replace-cross"
}

// arxivAbstract strips the RSS description's "arXiv:… Announce Type: …
// Abstract:" preamble and collapses hard-wrapped whitespace.
func arxivAbstract(description string) string {
	if strings.HasPrefix(strings.TrimSpace(description), "arXiv:") {
		if _, after, ok := strings.Cut(description, "Abstract:"); ok {
			description = after
		}
	}
	return strings.Join(strings.Fields(description), " ")
}

// arxivAPIError returns the message of the error entry the arXiv API sends,
// with HTTP 200, for a malformed query; "" when there is none.
func arxivAPIError(items []*gofeed.Item) string {
	for _, item := range items {
		if strings.Contains(item.GUID, "arxiv.org/api/errors") {
			return strings.Join(strings.Fields(item.Description), " ")
		}
	}
	return ""
}

// arxivItemHTML renders the paper's authors, abstract and links to its HTML
// rendition and PDF, without fetching anything.
func arxivItemHTML(id string, item *gofeed.Item) string {
	var b strings.Builder
	names := make([]string, 0, len(item.Authors))
	for _, a := range item.Authors {
		if a != nil && strings.TrimSpace(a.Name) != "" {
			names = append(names, strings.TrimSpace(a.Name))
		}
	}
	if len(names) > 0 {
		b.WriteString("<p><em>" + html.EscapeString(strings.Join(names, ", ")) +
			"</em></p>")
	}
	if abstract := arxivAbstract(item.Description); abstract != "" {
		b.WriteString("<p>" + html.EscapeString(abstract) + "</p>")
	}
	b.WriteString(`<p><a href="https://arxiv.org/html/` + id +
		`">Read the paper (HTML)</a> · <a href="https://arxiv.org/pdf/` + id +
		`">PDF</a></p>`)
	return b.String()
}

// feedItemGUID is the item's dedup key: the version-less abs URL for an
// arXiv paper, else the item's GUID or link. It is "" (skipped) for an arXiv
// feed's replacement announcements and non-paper entries.
func feedItemGUID(arxivFeed bool, item *gofeed.Item) string {
	if !arxivFeed {
		return itemGUID(item)
	}
	if id := arxivID(item); id != "" && !isArxivReplacement(item) {
		return arxivAbsURL(id)
	}
	return ""
}

// buildArxivItem builds a paper item from the entry alone; no page fetch.
func buildArxivItem(
	feed models.Feed,
	item *gofeed.Item,
	id, guid string,
	rules []models.FilterRule,
) *models.Item {
	absURL := arxivAbsURL(id)
	title := titleOrDefault(strings.Join(strings.Fields(item.Title), " "), absURL)
	categories := normalizeCategories(item.Categories)

	var publishedAt time.Time
	if item.PublishedParsed != nil {
		publishedAt = *item.PublishedParsed
	}

	//nolint:exhaustruct // read/dismissed/bookmarked/ingest_error start empty
	built := &models.Item{
		FeedID:      feed.ID,
		GUID:        guid,
		Title:       title,
		SourceURL:   absURL,
		ContentHTML: arxivItemHTML(id, item),
		PublishedAt: publishedAt,
		Categories:  categories,
	}
	markFiltered(built, matchFilterRule(rules, feed.ID, title, categories))
	return built
}

// arxivPacer spaces consecutive arXiv polls arxivMinRequestGap apart.
type arxivPacer struct {
	last time.Time
}

// wait blocks until feedURL may be fetched; non-arXiv feeds never wait.
func (p *arxivPacer) wait(ctx context.Context, feedURL string) error {
	if !isArxivFeed(feedURL) {
		return nil
	}
	if err := sleepCtx(ctx, arxivMinRequestGap-time.Since(p.last)); err != nil {
		return err
	}
	p.last = time.Now()
	return nil
}

// sleepCtx waits d (no-op when d <= 0), returning early with ctx's error.
func sleepCtx(ctx context.Context, d time.Duration) error {
	if d <= 0 {
		return nil
	}
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-t.C:
		return nil
	}
}
