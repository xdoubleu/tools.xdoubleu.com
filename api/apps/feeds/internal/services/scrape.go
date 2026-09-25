package services

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"net/url"
	"regexp"
	"slices"
	"strings"
	"time"

	"golang.org/x/net/html"

	"tools.xdoubleu.com/apps/feeds/internal/models"
	"tools.xdoubleu.com/apps/feeds/pkg/webfetch"
)

// ErrNoPostsFound is returned when discoverPostLinks finds no plausible post
// links on a page.
var ErrNoPostsFound = errors.New("no post links found on page")

// maxDiscoveredLinks caps candidate links per index page against noise
// floods; not a discovery ceiling, since pagination merges every page.
const maxDiscoveredLinks = 30

// minPostLinkTextLen is the minimum trimmed anchor-text length to look
// title-like rather than a nav/utility link ("Home", "More", "Sign in").
const minPostLinkTextLen = 15

// minBarePostLinkTextLen is the higher title-length bar for an anchor with no
// nested <time>/heading, filtering out short topic/category filter pills.
const minBarePostLinkTextLen = 25

// excludedPathSegments mark a link as a listing/utility page, not a post.
//
//nolint:gochecknoglobals // static lookup table, read-only after init
var excludedPathSegments = map[string]bool{
	"tag": true, "tags": true, "category": true, "categories": true,
	"author": true, "authors": true, "page": true, "search": true,
	"rss": true, "feed": true, "login": true, "signup": true,
	"subscribe": true,
}

// skippedContainerTags are site-chrome elements never scanned for post links.
//
//nolint:gochecknoglobals // static lookup table, read-only after init
var skippedContainerTags = map[string]bool{
	"nav": true, "header": true, "footer": true, "aside": true,
	"script": true, "style": true,
}

// headingTags: a nested heading is preferred as the post title over the
// anchor's full text, which may include date/category/excerpt.
//
//nolint:gochecknoglobals // static lookup table, read-only after init
var headingTags = map[string]bool{
	"h1": true, "h2": true, "h3": true, "h4": true, "h5": true, "h6": true,
}

// datePublishedLayout parses a card's <time> text (e.g. "Jun 16, 2026").
const datePublishedLayout = "Jan 2, 2006"

// discoveredLink is one candidate post on an index page, in DOM order.
type discoveredLink struct {
	URL   string
	Title string
	// PublishedAt is zero when the card had no parseable <time>.
	PublishedAt time.Time
}

// discoverPostLinks heuristically finds post-like links on a page with no
// RSS/Atom feed (see candidateLink). Best-effort, no per-site configuration.
func discoverPostLinks(pageURL string, body []byte) ([]discoveredLink, error) {
	return discoverPostLinksWithLocaleBase(pageURL, pageURL, body)
}

// discoverPostLinksWithLocaleBase is discoverPostLinks with localeBaseURL as
// the isLocaleAlternate base: paginated pages must be judged against the
// first page's URL, since extra segments (/page/2) break its segment check.
func discoverPostLinksWithLocaleBase(
	pageURL, localeBaseURL string,
	body []byte,
) ([]discoveredLink, error) {
	base, err := url.Parse(pageURL)
	if err != nil {
		return nil, fmt.Errorf("%w: bad page url", ErrNoPostsFound)
	}
	localeBase, err := url.Parse(localeBaseURL)
	if err != nil {
		return nil, fmt.Errorf("%w: bad locale base url", ErrNoPostsFound)
	}

	doc, err := html.Parse(bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrNoPostsFound, err)
	}

	out := []discoveredLink{}
	seen := make(map[string]bool)
	collectPostLinks(doc, base, localeBase, seen, &out)

	if len(out) > maxDiscoveredLinks {
		out = out[:maxDiscoveredLinks]
	}
	if len(out) == 0 {
		return nil, ErrNoPostsFound
	}
	return out, nil
}

// collectPostLinks appends candidate post links under n in DOM order.
func collectPostLinks(
	n *html.Node,
	base, localeBase *url.URL,
	seen map[string]bool,
	out *[]discoveredLink,
) {
	if n.Type == html.ElementNode && skippedContainerTags[n.Data] {
		return
	}
	if n.Type == html.ElementNode && n.Data == "a" {
		if link, ok := candidateLink(n, base, localeBase); ok && !seen[link.URL] {
			seen[link.URL] = true
			*out = append(*out, link)
		}
	}
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		collectPostLinks(c, base, localeBase, seen, out)
	}
}

// candidateLink checks whether an <a> looks like a post link. The title
// prefers a nested heading, else the anchor text minus any <time> (which
// becomes PublishedAt). Without a nested <time> or heading the title must
// clear minBarePostLinkTextLen instead of minPostLinkTextLen.
func candidateLink(
	n *html.Node,
	base, localeBase *url.URL,
) (discoveredLink, bool) {
	resolved, ok := candidatePostURL(n, base, localeBase)
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

// candidatePostURL resolves an <a>'s href and requires http(s), same domain,
// no listing/utility path, and not a locale alternate of localeBase.
func candidatePostURL(
	n *html.Node,
	base, localeBase *url.URL,
) (*url.URL, bool) {
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
	if isLocaleAlternate(resolved, localeBase) {
		return nil, false
	}

	return resolved, true
}

// localeSegmentPattern matches a language code segment ("en", "pt-BR").
var localeSegmentPattern = regexp.MustCompile(`(?i)^[a-z]{2}(-[a-z]{2})?$`)

// isLocaleAlternate reports whether resolved is base in another locale: same
// segment count, differing in exactly one segment, both locale-shaped. Such
// language switchers are often outside nav chrome, have title-length text,
// and carry volatile query strings that would defeat dedup.
func isLocaleAlternate(resolved, base *url.URL) bool {
	baseSegs := strings.Split(strings.Trim(base.Path, "/"), "/")
	segs := strings.Split(strings.Trim(resolved.Path, "/"), "/")
	if len(segs) != len(baseSegs) || len(segs) == 0 {
		return false
	}

	diffs := 0
	for i, seg := range segs {
		if seg == baseSegs[i] {
			continue
		}
		if !localeSegmentPattern.MatchString(seg) ||
			!localeSegmentPattern.MatchString(baseSegs[i]) {
			return false
		}
		diffs++
	}
	return diffs == 1
}

// nodeText concatenates all text within n's subtree, space-separated.
func nodeText(n *html.Node) string {
	return nodeTextExcluding(n, nil)
}

// nodeTextExcluding is nodeText skipping exclude's subtree (nil skips none).
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

// findNode returns the first node in n's subtree (n included) matching, or nil.
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

// pageTitle extracts the page's <title> text.
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

// nextPageLinkTexts mark an <a> without rel="next" as a next-page link
// (lowercased anchor text or aria-label).
//
//nolint:gochecknoglobals // static lookup table, read-only after init
var nextPageLinkTexts = map[string]bool{
	"next": true, "next page": true, "next posts": true,
	"older posts": true, "older": true, "more posts": true,
	"load more": true, "»": true, "›": true,
}

// hasRelNext reports whether n's rel attribute contains "next".
func hasRelNext(n *html.Node) bool {
	return slices.Contains(
		slices.Collect(strings.FieldsSeq(strings.ToLower(nodeAttr(n, "rel")))),
		"next",
	)
}

// isNextPageCandidate reports whether n is a <link>/<a rel="next"> or an <a>
// with pagination text.
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

// discoverNextPageURL resolves a same-domain "next page" link present on the
// page; it never guesses pagination URL patterns.
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

// fetchPaginatedPostLinks discovers post links on the first page, then
// follows "next page" links, merging and deduping. It is deliberately
// uncapped (a ceiling makes older posts unreachable) and stops when a page
// adds no new link, a URL repeats, or a fetch fails — degraded results, never
// errors. Stopping on zero new links is correct for newest-first indexes.
func (s *FeedService) fetchPaginatedPostLinks(
	ctx context.Context,
	firstPageURL string,
	firstPageBody []byte,
) ([]discoveredLink, error) {
	links, err := discoverPostLinksWithLocaleBase(
		firstPageURL, firstPageURL, firstPageBody,
	)
	if err != nil {
		return nil, err
	}

	seen := make(map[string]bool, len(links))
	out := make([]discoveredLink, 0, len(links))
	for _, link := range links {
		seen[link.URL] = true
		out = append(out, link)
	}

	visitedPages := map[string]bool{firstPageURL: true}
	pageURL, body := firstPageURL, firstPageBody
	for {
		nextURL, ok := discoverNextPageURL(pageURL, body)
		if !ok || visitedPages[nextURL] {
			break
		}
		visitedPages[nextURL] = true

		nextFinalURL, nextBody, pageLinks, ok := s.fetchScrapePage(
			ctx, nextURL, firstPageURL,
		)
		if !ok {
			break
		}

		newOnPage := 0
		for _, link := range pageLinks {
			if seen[link.URL] {
				continue
			}
			seen[link.URL] = true
			newOnPage++
			out = append(out, link)
		}
		if newOnPage == 0 {
			break
		}
		pageURL, body = nextFinalURL, nextBody
	}

	return out, nil
}

// fetchScrapePage fetches one index page and discovers its post links; ok is
// false on any failure, which ends pagination.
func (s *FeedService) fetchScrapePage(
	ctx context.Context,
	pageURL, localeBaseURL string,
) (string, []byte, []discoveredLink, bool) {
	res, err := s.webFetch.Get(
		ctx, pageURL, fetchOptions(0, "text/html,application/xhtml+xml"),
	)
	if err != nil {
		return "", nil, nil, false
	}
	links, err := discoverPostLinksWithLocaleBase(res.FinalURL, localeBaseURL, res.Body)
	if err != nil {
		return "", nil, nil, false
	}
	return res.FinalURL, res.Body, links, true
}

// CreateScrape validates the URL by finding at least one post link on the
// first page, stores the feed and imports it in the background (like Create).
// Validation stays on page 1: the full pagination walk would blow the
// request deadline.
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
	// Page-1 discovery doubles as URL validation.
	_, discErr := discoverPostLinksWithLocaleBase(
		res.FinalURL, res.FinalURL, res.Body,
	)
	if discErr != nil {
		return nil, discErr
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

	// Detached, not queued: a restart can drop it; poll-feeds backfills.
	importFeed := *feed
	go s.importScrapeFeed(ctx, importFeed, res)

	return feed, nil
}

// importScrapeFeed is CreateScrape's detached import: walk full pagination
// (partial results are imported as-is), ingest, and record the fetch result
// that arms conditional GET.
func (s *FeedService) importScrapeFeed(
	ctx context.Context,
	feed models.Feed,
	res *webfetch.Result,
) {
	importCtx := context.WithoutCancel(ctx)
	walked, _ := s.fetchPaginatedPostLinks(importCtx, res.FinalURL, res.Body)
	s.ingestDiscoveredLinks(importCtx, feed, walked)
	s.recordFetchResult(importCtx, feed.ID, res, nil)
}

// pollScrapeFeed is the scrape counterpart to pollFeedRSS: a conditional GET
// plus the full paginated walk. Only posts listed on the feed's own index
// page are ever seen, which doubles as category membership.
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

// ingestDiscoveredLinks ingests unseen links, capped at maxItemsPerPoll, in
// DOM order (most links have no date). Overflow is left unseen so the next
// poll resurfaces it; marking it seen would drop posts permanently.
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
			continue
		}
		if s.ingestDiscoveredLink(ctx, feed, byGUID[guid], guid) {
			ingested++
		}
	}
	return ingested
}

// ingestDiscoveredLink fetches and extracts one post, titled by its anchor
// text. With no feed description to fall back to, a failed fetch/extraction
// drops the item (marked seen, never retried).
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
