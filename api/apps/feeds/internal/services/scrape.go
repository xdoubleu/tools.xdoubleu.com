package services

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"net/url"
	"slices"
	"strings"
	"time"

	"golang.org/x/net/html"

	"tools.xdoubleu.com/apps/feeds/internal/models"
)

// ErrNoPostsFound is returned when discoverPostLinks finds no plausible post
// links on a page — either the URL isn't a blog index, or its layout
// doesn't match the heuristic (issue #751).
var ErrNoPostsFound = errors.New("no post links found on page")

// maxDiscoveredLinks caps how many candidate post links discoverPostLinks
// returns from one index page.
const maxDiscoveredLinks = 30

// maxScrapePages caps how many index pages (the first page, plus discovered
// "next page" links) one scrape run fetches — some blogs (e.g. Uber's
// engineering blog, issue #1748) spread their current posts across more than
// one paginated index page, and this bounds request volume and protects
// against a pagination loop on a misbehaving site.
const maxScrapePages = 3

// minPostLinkTextLen is the minimum trimmed anchor-text length to look
// title-like rather than a nav/utility link ("Home", "More", "Sign in").
const minPostLinkTextLen = 15

// minBarePostLinkTextLen is the higher title-length bar applied to an anchor
// with no nested <time>/heading element — short topic/category filter pills
// ("Interpretability", "Economic research") clear minPostLinkTextLen but
// have no other structural signal (issue #835) distinguishing them from a
// real post link, unlike a card with a <time> or heading (issue #829).
const minBarePostLinkTextLen = 25

// excludedPathSegments are URL path segments that mark a link as a listing
// or utility page rather than a post, even when its anchor text is
// title-like.
//
//nolint:gochecknoglobals // static lookup table, read-only after init
var excludedPathSegments = map[string]bool{
	"tag": true, "tags": true, "category": true, "categories": true,
	"author": true, "authors": true, "page": true, "search": true,
	"rss": true, "feed": true, "login": true, "signup": true,
	"subscribe": true,
}

// skippedContainerTags are elements whose subtree is never a source of post
// links — site chrome, not content.
//
//nolint:gochecknoglobals // static lookup table, read-only after init
var skippedContainerTags = map[string]bool{
	"nav": true, "header": true, "footer": true, "aside": true,
	"script": true, "style": true,
}

// headingTags mark an element whose text, if present in a candidate link's
// subtree, is preferred as the post title over the anchor's full
// concatenated text — some card layouts wrap a heading alongside a
// <time>/category/excerpt in the same <a>, and using the heading alone
// avoids pulling that surrounding text into the title (issue #829).
//
//nolint:gochecknoglobals // static lookup table, read-only after init
var headingTags = map[string]bool{
	"h1": true, "h2": true, "h3": true, "h4": true, "h5": true, "h6": true,
}

// datePublishedLayout is the "Mon D, YYYY" format a scraped card's <time>
// text is parsed with (e.g. "Jun 16, 2026").
const datePublishedLayout = "Jan 2, 2006"

// discoveredLink is one candidate post found on an index page, in page (DOM)
// order.
type discoveredLink struct {
	URL   string
	Title string
	// PublishedAt is the zero time when the card had no parseable <time>
	// element.
	PublishedAt time.Time
}

// discoverPostLinks heuristically finds post-like links on a page with no
// real RSS/Atom feed: it skips nav/header/footer/aside/script/style
// subtrees, then keeps same-domain <a> elements whose title text (a nested
// heading if present, else the full anchor text minus any <time> element)
// is title-like — length ≥ minPostLinkTextLen when a nested <time>/heading
// gives some structural signal the anchor is an article card, else the
// higher minBarePostLinkTextLen bar — and whose path doesn't look like a
// listing/utility page. There is no per-site configuration — this is
// best-effort and will miss or misfire on unusual page layouts.
func discoverPostLinks(pageURL string, body []byte) ([]discoveredLink, error) {
	base, err := url.Parse(pageURL)
	if err != nil {
		return nil, fmt.Errorf("%w: bad page url", ErrNoPostsFound)
	}

	doc, err := html.Parse(bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrNoPostsFound, err)
	}

	out := []discoveredLink{}
	seen := make(map[string]bool)
	collectPostLinks(doc, base, seen, &out)

	if len(out) > maxDiscoveredLinks {
		out = out[:maxDiscoveredLinks]
	}
	if len(out) == 0 {
		return nil, ErrNoPostsFound
	}
	return out, nil
}

// collectPostLinks recursively appends every candidate post link under n, in
// DOM order, skipping site-chrome subtrees.
func collectPostLinks(
	n *html.Node,
	base *url.URL,
	seen map[string]bool,
	out *[]discoveredLink,
) {
	if n.Type == html.ElementNode && skippedContainerTags[n.Data] {
		return
	}
	if n.Type == html.ElementNode && n.Data == "a" {
		if link, ok := candidateLink(n, base); ok && !seen[link.URL] {
			seen[link.URL] = true
			*out = append(*out, link)
		}
	}
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		collectPostLinks(c, base, seen, out)
	}
}

// candidateLink checks whether an <a> node looks like a post link: a
// same-domain http(s) URL, not pointing at a listing/utility path, with
// title-like anchor text. The title prefers a nested heading (h1-h6) over
// the anchor's full text, and a nested <time> element's text (parsed with
// datePublishedLayout) becomes the link's PublishedAt and is excluded from
// the title when there's no heading to prefer instead. An anchor with
// neither a nested <time> nor heading — no structural signal it's an
// article card rather than a plain nav/filter link — must clear the higher
// minBarePostLinkTextLen bar instead of minPostLinkTextLen.
func candidateLink(n *html.Node, base *url.URL) (discoveredLink, bool) {
	resolved, ok := candidatePostURL(n, base)
	if !ok {
		//nolint:exhaustruct // rejection sentinel; caller only reads ok
		return discoveredLink{}, false
	}

	timeNode := findNode(n, func(c *html.Node) bool {
		return c.Type == html.ElementNode && c.Data == "time"
	})
	heading := findNode(n, func(c *html.Node) bool {
		return c.Type == html.ElementNode && headingTags[c.Data]
	})

	var title string
	if heading != nil {
		title = strings.Join(strings.Fields(nodeText(heading)), " ")
	} else {
		title = strings.Join(strings.Fields(nodeTextExcluding(n, timeNode)), " ")
	}

	minLen := minPostLinkTextLen
	if timeNode == nil && heading == nil {
		minLen = minBarePostLinkTextLen
	}
	if len(title) < minLen {
		//nolint:exhaustruct // rejection sentinel; caller only reads ok
		return discoveredLink{}, false
	}

	//nolint:exhaustruct // PublishedAt filled in below only when parseable
	link := discoveredLink{URL: resolved.String(), Title: title}
	if timeNode != nil {
		dateText := strings.TrimSpace(nodeText(timeNode))
		if t, err := time.Parse(datePublishedLayout, dateText); err == nil {
			link.PublishedAt = t
		}
	}
	return link, true
}

// candidatePostURL resolves an <a> node's href and checks it against the
// scrape heuristic's URL-shape rules: http(s), same domain as base, and not
// a listing/utility path.
func candidatePostURL(n *html.Node, base *url.URL) (*url.URL, bool) {
	href := strings.TrimSpace(nodeAttr(n, "href"))
	if href == "" || strings.HasPrefix(href, "#") {
		return nil, false
	}
	ref, err := url.Parse(href)
	if err != nil {
		return nil, false
	}
	resolved := base.ResolveReference(ref)
	if !isHTTPScheme(resolved.Scheme) {
		return nil, false
	}
	if !strings.EqualFold(resolved.Host, base.Host) {
		return nil, false
	}
	resolved.Fragment = ""

	path := strings.Trim(resolved.Path, "/")
	if path == "" || path == strings.Trim(base.Path, "/") {
		return nil, false
	}
	for _, seg := range strings.Split(path, "/") {
		if excludedPathSegments[strings.ToLower(seg)] {
			return nil, false
		}
	}

	return resolved, true
}

// nodeText concatenates all text within n's subtree, space-separated.
func nodeText(n *html.Node) string {
	return nodeTextExcluding(n, nil)
}

// nodeTextExcluding is nodeText, skipping exclude's entire subtree (a nil
// exclude skips nothing).
func nodeTextExcluding(n, exclude *html.Node) string {
	if n == exclude {
		return ""
	}
	if n.Type == html.TextNode {
		return n.Data
	}
	var sb strings.Builder
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		sb.WriteString(nodeTextExcluding(c, exclude))
		sb.WriteString(" ")
	}
	return sb.String()
}

// findNode returns the first node in n's subtree (n included, DOM order)
// for which match returns true, or nil.
func findNode(n *html.Node, match func(*html.Node) bool) *html.Node {
	if match(n) {
		return n
	}
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		if found := findNode(c, match); found != nil {
			return found
		}
	}
	return nil
}

func nodeAttr(n *html.Node, key string) string {
	for _, a := range n.Attr {
		if a.Key == key {
			return a.Val
		}
	}
	return ""
}

// pageTitle extracts the page's <title> text, used as a scrape feed's
// display title at creation time.
func pageTitle(body []byte) string {
	doc, err := html.Parse(bytes.NewReader(body))
	if err != nil {
		return ""
	}
	var title string
	var walk func(n *html.Node)
	walk = func(n *html.Node) {
		if title != "" {
			return
		}
		if n.Type == html.ElementNode && n.Data == "title" {
			title = strings.TrimSpace(nodeText(n))
			return
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(doc)
	return title
}

// nextPageLinkTexts are the trimmed, lowercased anchor text or aria-label
// values that mark an <a> as pointing at the next pagination page when it
// carries no rel="next" attribute.
//
//nolint:gochecknoglobals // static lookup table, read-only after init
var nextPageLinkTexts = map[string]bool{
	"next": true, "next page": true, "next posts": true,
	"older posts": true, "older": true, "more posts": true,
	"load more": true, "»": true, "›": true,
}

// hasRelNext reports whether n's space-separated rel attribute contains
// "next" — the standard way both <link> and <a> elements mark pagination.
func hasRelNext(n *html.Node) bool {
	return slices.Contains(
		slices.Collect(strings.FieldsSeq(strings.ToLower(nodeAttr(n, "rel")))),
		"next",
	)
}

// isNextPageCandidate reports whether n looks like a "next page" link: a
// <link rel="next"> (typically in <head>), or an <a> with rel="next" or
// title-like pagination text/aria-label.
func isNextPageCandidate(n *html.Node) bool {
	if n.Type != html.ElementNode {
		return false
	}
	if n.Data == "link" {
		return hasRelNext(n)
	}
	if n.Data != "a" {
		return false
	}
	if hasRelNext(n) {
		return true
	}
	text := strings.ToLower(strings.TrimSpace(nodeText(n)))
	aria := strings.ToLower(strings.TrimSpace(nodeAttr(n, "aria-label")))
	return nextPageLinkTexts[text] || nextPageLinkTexts[aria]
}

// discoverNextPageURL looks for a same-domain "next page" link on an already
// fetched index page (see isNextPageCandidate) and resolves it against
// pageURL. It never guesses a pagination URL pattern — only a link actually
// present on the page is followed.
func discoverNextPageURL(pageURL string, body []byte) (string, bool) {
	base, err := url.Parse(pageURL)
	if err != nil {
		return "", false
	}
	doc, err := html.Parse(bytes.NewReader(body))
	if err != nil {
		return "", false
	}

	n := findNode(doc, isNextPageCandidate)
	if n == nil {
		return "", false
	}
	href := strings.TrimSpace(nodeAttr(n, "href"))
	if href == "" {
		return "", false
	}
	ref, err := url.Parse(href)
	if err != nil {
		return "", false
	}

	resolved := base.ResolveReference(ref)
	if !isHTTPScheme(resolved.Scheme) {
		return "", false
	}
	if !strings.EqualFold(resolved.Host, base.Host) {
		return "", false
	}
	resolved.Fragment = ""
	return resolved.String(), true
}

// fetchPaginatedPostLinks discovers post links on an already fetched first
// page, then follows any discoverable "next page" link (discoverNextPageURL)
// up to maxScrapePages total pages, merging and deduping links across pages
// (capped overall at maxDiscoveredLinks) — the paginated counterpart to a
// single discoverPostLinks call. A later page that fails to fetch or yields
// no post links simply ends pagination rather than failing the whole call,
// since the first page already succeeded.
func (s *FeedService) fetchPaginatedPostLinks(
	ctx context.Context,
	firstPageURL string,
	firstPageBody []byte,
) ([]discoveredLink, error) {
	links, err := discoverPostLinks(firstPageURL, firstPageBody)
	if err != nil {
		return nil, err
	}

	seen := make(map[string]bool, len(links))
	out := make([]discoveredLink, 0, len(links))
	for _, link := range links {
		seen[link.URL] = true
		out = append(out, link)
	}

	pageURL, body := firstPageURL, firstPageBody
	for page := 1; page < maxScrapePages && len(out) < maxDiscoveredLinks; page++ {
		nextURL, ok := discoverNextPageURL(pageURL, body)
		if !ok {
			break
		}

		nextFinalURL, nextBody, pageLinks, ok := s.fetchScrapePage(ctx, nextURL)
		if !ok {
			// A later page failing to fetch or yielding no post links just
			// ends pagination — the first page already succeeded, so this
			// is a degraded result, not an error.
			break
		}

		for _, link := range pageLinks {
			if len(out) >= maxDiscoveredLinks || seen[link.URL] {
				continue
			}
			seen[link.URL] = true
			out = append(out, link)
		}
		pageURL, body = nextFinalURL, nextBody
	}

	return out, nil
}

// fetchScrapePage fetches one index page and discovers its post links, for
// use by fetchPaginatedPostLinks' pagination loop — ok is false on any fetch
// or discovery failure, which the caller treats as the end of pagination.
func (s *FeedService) fetchScrapePage(
	ctx context.Context,
	pageURL string,
) (string, []byte, []discoveredLink, bool) {
	res, err := s.webFetch.Get(
		ctx, pageURL, fetchOptions(0, "text/html,application/xhtml+xml"),
	)
	if err != nil {
		return "", nil, nil, false
	}
	links, err := discoverPostLinks(res.FinalURL, res.Body)
	if err != nil {
		return "", nil, nil, false
	}
	return res.FinalURL, res.Body, links, true
}

// CreateScrape validates the URL by fetching it and discovering at least one
// post link, then stores the feed (source_type "scrape") and imports its
// current contents as a first batch in the background — mirrors Create's
// detached-import shape, see its comment for why the import is not part of
// the response.
func (s *FeedService) CreateScrape(
	ctx context.Context,
	userID, rawURL string,
) (*models.Feed, error) {
	canonical, err := canonicalURL(rawURL)
	if err != nil {
		return nil, err
	}

	res, err := s.webFetch.Get(
		ctx, canonical, fetchOptions(0, "text/html,application/xhtml+xml"),
	)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrNoPostsFound, err)
	}
	links, err := s.fetchPaginatedPostLinks(ctx, res.FinalURL, res.Body)
	if err != nil {
		return nil, err
	}

	//nolint:exhaustruct // fetch state starts empty; ids are DB-owned
	feed, err := s.feeds.Insert(ctx, models.Feed{
		UserID:     userID,
		URL:        canonical,
		Title:      pageTitle(res.Body),
		SourceType: models.FeedSourceScrape,
	})
	if err != nil {
		return nil, err
	}

	// ponytail: detached goroutine, not a job-queue task — a process restart
	// mid-import can drop it; the hourly poll-feeds job backfills.
	importFeed := *feed
	go func() {
		importCtx := context.WithoutCancel(ctx)
		s.ingestDiscoveredLinks(importCtx, importFeed, links)
		s.recordFetchResult(importCtx, importFeed.ID, res, nil)
	}()
	return feed, nil
}

// pollScrapeFeed fetches one scrape feed's index page (conditional GET) and
// ingests any newly discovered post links — the scrape counterpart to
// pollFeedRSS.
func (s *FeedService) pollScrapeFeed(
	ctx context.Context,
	feed models.Feed,
) (int, error) {
	opts := fetchOptions(0, "text/html,application/xhtml+xml")
	if feed.ETag != nil {
		opts.ETag = *feed.ETag
	}
	if feed.LastModified != nil {
		opts.LastModified = *feed.LastModified
	}

	res, err := s.webFetch.Get(ctx, feed.URL, opts)
	if err != nil {
		s.recordFetchResult(ctx, feed.ID, nil, err)
		return 0, err
	}
	if res.NotModified {
		s.recordFetchResult(ctx, feed.ID, res, nil)
		return 0, nil
	}

	links, err := s.fetchPaginatedPostLinks(ctx, res.FinalURL, res.Body)
	if err != nil {
		s.recordFetchResult(ctx, feed.ID, nil, err)
		return 0, err
	}

	ingested := s.ingestDiscoveredLinks(ctx, feed, links)
	s.recordFetchResult(ctx, feed.ID, res, nil)
	return ingested, nil
}

// ingestDiscoveredLinks ingests the not-yet-seen discovered links, capped at
// maxItemsPerPoll — the scrape counterpart to processItems. Most discovered
// links carry no publish date (only cards with a parseable <time> element
// do, see candidateLink), so overflow ordering is just page (DOM) order
// rather than newest-first.
func (s *FeedService) ingestDiscoveredLinks(
	ctx context.Context,
	feed models.Feed,
	links []discoveredLink,
) int {
	guids := make([]string, 0, len(links))
	byGUID := make(map[string]discoveredLink, len(links))
	for _, link := range links {
		canonical, err := canonicalURL(link.URL)
		if err != nil {
			continue
		}
		if _, exists := byGUID[canonical]; exists {
			continue
		}
		guids = append(guids, canonical)
		byGUID[canonical] = link
	}

	newGUIDs, err := s.items.FilterNewGUIDs(ctx, feed.ID, guids)
	if err != nil {
		s.logger.WarnContext(ctx, "scrape feed guid filter failed",
			"feedID", feed.ID, "error", err)
		return 0
	}

	ingested := 0
	for i, guid := range newGUIDs {
		if ctx.Err() != nil {
			return ingested
		}
		if i >= maxItemsPerPoll {
			s.markSeenError(ctx, feed.ID, guid, "skipped: over per-poll cap")
			continue
		}
		if s.ingestDiscoveredLink(ctx, feed, byGUID[guid], guid) {
			ingested++
		}
	}
	return ingested
}

// ingestDiscoveredLink fetches and readability-extracts one discovered
// post's content, using the discovered anchor text as its title (always
// non-empty, per discoverPostLinks' minPostLinkTextLen filter — extraction
// can't improve on it, since extractReadable itself falls back to the URL
// when a page has no <title>). Unlike an RSS item there is no feed-supplied
// description to fall back to for content, so a failed fetch/extraction
// drops the item (marked seen via markSeenError, never retried by polling —
// same as ingestItem's error path).
func (s *FeedService) ingestDiscoveredLink(
	ctx context.Context,
	feed models.Feed,
	link discoveredLink,
	guid string,
) bool {
	title := link.Title
	body := s.fetchLinkedPageHTML(ctx, guid, &title)
	if body == "" {
		s.markSeenError(ctx, feed.ID, guid, "content fetch failed")
		return false
	}

	publishedAt := time.Now()
	if !link.PublishedAt.IsZero() {
		publishedAt = link.PublishedAt
	}

	//nolint:exhaustruct // read/dismissed/bookmarked/ingest_error start empty
	item := models.Item{
		FeedID:      feed.ID,
		GUID:        guid,
		Title:       title,
		SourceURL:   guid,
		ContentHTML: body,
		PublishedAt: publishedAt,
	}
	if err := s.items.Insert(ctx, item); err != nil {
		s.logger.WarnContext(ctx, "scrape feed item store failed",
			"feedID", feed.ID, "guid", guid, "error", err)
		return false
	}
	return true
}
