package services

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"log/slog"
	"sort"

	"github.com/google/uuid"
	"github.com/mmcdole/gofeed"

	"tools.xdoubleu.com/apps/reading/internal/models"
	"tools.xdoubleu.com/apps/reading/internal/repositories"
	"tools.xdoubleu.com/apps/reading/pkg/arxiv"
	"tools.xdoubleu.com/apps/reading/pkg/webfetch"
)

// ErrInvalidFeed is returned when a subscribed URL does not parse as RSS/Atom.
var ErrInvalidFeed = errors.New("url is not a valid RSS/Atom feed")

// ErrEmailFeedsNotConfigured is returned by CreateEmail when
// EMAIL_INBOUND_DOMAIN is unset — minting an inbound address without a
// receiving domain would produce one that can never receive mail.
var ErrEmailFeedsNotConfigured = errors.New(
	"email feeds are not configured (EMAIL_INBOUND_DOMAIN unset)",
)

// emailTokenBytes is the number of random bytes for an email feed's inbound
// alias token. Mirrors koboTokenBytes (kobo.go) — same high-entropy,
// hash-before-storing pattern.
const emailTokenBytes = 32

// maxItemsPerPoll caps how many new items one poll ingests per feed (newest
// first); older overflow is marked seen without ingesting.
const maxItemsPerPoll = 20

// FeedService manages RSS/Atom subscriptions and email-relay newsletter
// subscriptions (issue #595), ingesting their items into the library
// (category "rss") via IngestService.
type FeedService struct {
	logger        *slog.Logger
	feeds         *repositories.FeedsRepository
	ingest        *IngestService
	books         *BookService
	webFetch      webfetch.Client
	inboundDomain string
}

// NewFeedService constructs a FeedService. inboundDomain is the
// EMAIL_INBOUND_DOMAIN used to build email feeds' inbound addresses; empty
// disables CreateEmail (see ErrEmailFeedsNotConfigured).
func NewFeedService(
	logger *slog.Logger,
	feeds *repositories.FeedsRepository,
	ingest *IngestService,
	books *BookService,
	webFetchClient webfetch.Client,
	inboundDomain string,
) *FeedService {
	return &FeedService{
		logger:        logger,
		feeds:         feeds,
		ingest:        ingest,
		books:         books,
		webFetch:      webFetchClient,
		inboundDomain: inboundDomain,
	}
}

// List returns the user's feeds.
func (s *FeedService) List(
	ctx context.Context,
	userID string,
) ([]models.Feed, error) {
	return s.feeds.List(ctx, userID)
}

// ListItemBooks returns which feed each of the user's ingested library books
// came from, for the ad hoc feed-reader view (issue #476).
func (s *FeedService) ListItemBooks(
	ctx context.Context,
	userID string,
) ([]models.FeedItemBook, error) {
	return s.feeds.ListItemBooks(ctx, userID)
}

// Create validates the URL by fetching and parsing it and stores the feed
// (with its self-reported title), then imports the feed's current contents
// as a first batch in the background. Returns the feed as soon as it is
// stored — the initial import (up to maxItemsPerPoll items, each possibly
// fetching its linked page and running it through Calibre) can comfortably
// exceed the server's write timeout, so it must not block the request; the
// same items land within seconds via the detached import, or within the hour
// via the poll-feeds job if the process restarts mid-import.
func (s *FeedService) Create(
	ctx context.Context,
	userID, rawURL string,
) (*models.Feed, error) {
	canonical, err := canonicalURL(rawURL)
	if err != nil {
		return nil, err
	}

	res, err := s.webFetch.Get(ctx, canonical, fetchOptions(0, ""))
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrInvalidFeed, err)
	}
	parsed, err := gofeed.NewParser().Parse(bytes.NewReader(res.Body))
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrInvalidFeed, err)
	}

	//nolint:exhaustruct // fetch state starts empty; ids are DB-owned
	feed, err := s.feeds.Insert(ctx, models.Feed{
		UserID: userID,
		URL:    canonical,
		Title:  parsed.Title,
	})
	if err != nil {
		return nil, err
	}

	// ponytail: detached goroutine, not a job-queue task — mirrors the
	// existing KEPUB-conversion pattern (connect_files.go). A process
	// restart mid-import can drop it; the hourly poll-feeds job backfills.
	importFeed := *feed
	go func() {
		importCtx := context.WithoutCancel(ctx)
		s.processItems(importCtx, importFeed, parsed.Items)
		s.recordFetchResult(importCtx, importFeed.ID, res, nil)
	}()
	return feed, nil
}

// CreateEmail mints a per-feed inbound email alias and stores the feed
// (source_type "email"). Returns the feed plus the plaintext inbound
// address — the only time it is ever available in plaintext, since only its
// hash is persisted (mirrors the Kobo device-token pattern, kobo.go).
// Unlike Create, there is no background import: content only arrives as
// mail is received via the Resend webhook.
func (s *FeedService) CreateEmail(
	ctx context.Context,
	userID string,
	title string,
) (*models.Feed, string, error) {
	if s.inboundDomain == "" {
		return nil, "", ErrEmailFeedsNotConfigured
	}

	if title == "" {
		title = "Email newsletter"
	}

	raw := make([]byte, emailTokenBytes)
	if _, err := rand.Read(raw); err != nil {
		return nil, "", err
	}
	// lowercase hex, not base64: some mail relays lowercase the recipient
	// local-part in transit, which silently breaks a mixed-case token
	// (issue #661) — hex has no case ambiguity to mangle in the first place.
	token := hex.EncodeToString(raw)
	h := sha256.Sum256([]byte(token))
	hash := hex.EncodeToString(h[:])

	//nolint:exhaustruct // fetch state starts empty; ids are DB-owned
	feed, err := s.feeds.Insert(ctx, models.Feed{
		UserID:       userID,
		Title:        title,
		SourceType:   models.FeedSourceEmail,
		InboundToken: &hash,
	})
	if err != nil {
		return nil, "", err
	}

	address := token + "@" + s.inboundDomain
	return feed, address, nil
}

// GetByInboundTokenHash resolves an email feed by its inbound alias token's
// SHA-256 hash, for the unauthenticated Resend inbound-webhook handler.
func (s *FeedService) GetByInboundTokenHash(
	ctx context.Context,
	hash string,
) (*models.Feed, error) {
	return s.feeds.GetByInboundTokenHash(ctx, hash)
}

// IngestEmail ingests one inbound email as a library item for the given
// email feed (issue #595) — the webhook-push counterpart to ingestItem for
// polled RSS items. messageID is Resend's email id, used to build a stable
// dedup SourceURL alongside the feed ID; BaseURL is left empty since an
// email has no fetchable page to resolve relative image links against (most
// newsletter HTML already uses absolute CDN image URLs — localizeImages
// simply drops any that don't resolve, ingest_images.go). Best-effort: any
// ingest failure is recorded on the feed (visible in the UI's last-error)
// rather than returned, since the caller must still ack the webhook so
// Resend doesn't retry a permanently-broken email forever.
func (s *FeedService) IngestEmail(
	ctx context.Context,
	feed models.Feed,
	messageID, subject, htmlBody string,
) {
	content := ArticleContent{ //nolint:exhaustruct // no byline/excerpt/cover
		SourceURL: "mailto:" + feed.ID.String() + "/" + messageID,
		BaseURL:   "",
		Category:  models.CategoryRSS,
		Title:     subject,
		HTML:      htmlBody,
	}

	_, err := s.ingest.IngestArticleContent(ctx, feed.UserID, content)
	if err != nil {
		s.logger.WarnContext(ctx, "email feed ingest failed",
			"feedID", feed.ID, "messageID", messageID, "error", err)
		errStr := err.Error()
		if setErr := s.feeds.SetFetchResult(
			ctx, feed.ID, nil, nil, &errStr,
		); setErr != nil {
			s.logger.WarnContext(ctx, "email feed fetch-result update failed",
				"feedID", feed.ID, "error", setErr)
		}
		return
	}

	if setErr := s.feeds.SetFetchResult(ctx, feed.ID, nil, nil, nil); setErr != nil {
		s.logger.WarnContext(ctx, "email feed fetch-result update failed",
			"feedID", feed.ID, "error", setErr)
	}
}

// RecordEmailFetchFailure persists a fetch failure for an email feed when
// the inbound webhook itself couldn't retrieve the email body (e.g.
// Resend's receiving API erroring) — before IngestEmail is ever reached.
// Mirrors IngestEmail's own error branch so this failure is visible in the
// UI's last-error the same way an ingest failure already is.
func (s *FeedService) RecordEmailFetchFailure(
	ctx context.Context,
	feedID uuid.UUID,
	fetchErr error,
) {
	errStr := fetchErr.Error()
	if err := s.feeds.SetFetchResult(ctx, feedID, nil, nil, &errStr); err != nil {
		s.logger.WarnContext(ctx, "email feed fetch-result update failed",
			"feedID", feedID, "error", err)
	}
}

// Update changes the feed's title.
func (s *FeedService) Update(
	ctx context.Context,
	userID string,
	id uuid.UUID,
	title string,
) error {
	return s.feeds.Update(ctx, userID, id, title)
}

// Delete removes the subscription and the library items it ingested, except
// any the user engaged with (read or favourited), which are kept. The removable
// book IDs are collected before the feed is deleted, while the feed_items links
// still exist.
func (s *FeedService) Delete(
	ctx context.Context,
	userID string,
	id uuid.UUID,
) error {
	bookIDs, err := s.feeds.ListRemovableBookIDs(ctx, userID, id)
	if err != nil {
		return err
	}
	for _, bookID := range bookIDs {
		if err = s.books.RemoveFromLibrary(ctx, userID, bookID); err != nil {
			return err
		}
	}
	return s.feeds.Delete(ctx, userID, id)
}

// Refresh polls one feed synchronously. Returns how many items it ingested.
// Email feeds are push-only (Resend webhook), so this is a no-op for them.
func (s *FeedService) Refresh(
	ctx context.Context,
	userID string,
	id uuid.UUID,
) (int, error) {
	feed, err := s.feeds.GetByID(ctx, userID, id)
	if err != nil {
		return 0, err
	}
	if feed.SourceType == models.FeedSourceEmail {
		return 0, nil
	}
	return s.pollFeed(ctx, *feed)
}

// PollAll polls every feed of every user; per-feed failures are recorded on
// the feed and never abort the run. Called by the background job.
func (s *FeedService) PollAll(
	ctx context.Context,
	logger *slog.Logger,
	onProgress func(processed, total int),
) error {
	feeds, err := s.feeds.ListAll(ctx)
	if err != nil {
		return err
	}

	for i, feed := range feeds {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		if _, pollErr := s.pollFeed(ctx, feed); pollErr != nil {
			logger.WarnContext(ctx, "feed poll failed",
				"feedID", feed.ID, "url", feed.URL, "error", pollErr)
		}
		if onProgress != nil {
			onProgress(i+1, len(feeds))
		}
	}
	return nil
}

// pollFeed fetches one feed (conditional GET) and ingests its new items.
func (s *FeedService) pollFeed(
	ctx context.Context,
	feed models.Feed,
) (int, error) {
	opts := fetchOptions(0, "")
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

	parsed, err := gofeed.NewParser().Parse(bytes.NewReader(res.Body))
	if err != nil {
		wrapped := fmt.Errorf("%w: %w", ErrInvalidFeed, err)
		s.recordFetchResult(ctx, feed.ID, nil, wrapped)
		return 0, wrapped
	}

	ingested := s.processItems(ctx, feed, parsed.Items)
	s.recordFetchResult(ctx, feed.ID, res, nil)
	return ingested, nil
}

// processItems ingests the feed's not-yet-seen items, newest first, capped at
// maxItemsPerPoll per run; the overflow is marked seen without ingesting so a
// huge backlog never floods the library.
func (s *FeedService) processItems(
	ctx context.Context,
	feed models.Feed,
	items []*gofeed.Item,
) int {
	// Newest first: published desc, unparsed dates last (kept in feed order —
	// most feeds list newest first anyway).
	ordered := make([]*gofeed.Item, len(items))
	copy(ordered, items)
	sort.SliceStable(ordered, func(i, j int) bool {
		a, b := ordered[i].PublishedParsed, ordered[j].PublishedParsed
		if a == nil || b == nil {
			return false
		}
		return a.After(*b)
	})

	guids := make([]string, 0, len(ordered))
	byGUID := make(map[string]*gofeed.Item, len(ordered))
	for _, item := range ordered {
		guid := itemGUID(item)
		if guid == "" || byGUID[guid] != nil {
			continue
		}
		guids = append(guids, guid)
		byGUID[guid] = item
	}

	newGUIDs, err := s.feeds.FilterNewGUIDs(ctx, feed.ID, guids)
	if err != nil {
		s.logger.WarnContext(ctx, "feed guid filter failed",
			"feedID", feed.ID, "error", err)
		return 0
	}

	ingested := 0
	for i, guid := range newGUIDs {
		if ctx.Err() != nil {
			return ingested
		}
		if i >= maxItemsPerPoll {
			// Mark the overflow seen so it is never ingested later — the
			// backlog is browsable on the site, and Add-by-URL backfills
			// specific posts on demand.
			s.markSeen(ctx, feed.ID, guid, nil, "skipped: over per-poll cap")
			continue
		}
		if s.ingestItem(ctx, feed, byGUID[guid], guid) {
			ingested++
		}
	}
	return ingested
}

// ingestItem ingests one feed item; reports whether it produced a library
// entry. The guid is marked seen regardless of outcome (Add-by-URL is the
// retry path).
func (s *FeedService) ingestItem(
	ctx context.Context,
	feed models.Feed,
	item *gofeed.Item,
	guid string,
) bool {
	ub, err := s.ingestItemContent(ctx, feed, item)
	if err != nil {
		s.logger.WarnContext(ctx, "feed item ingest failed",
			"feedID", feed.ID, "guid", guid, "error", err)
		s.markSeen(ctx, feed.ID, guid, nil, err.Error())
		return false
	}

	s.markSeen(ctx, feed.ID, guid, &ub.BookID, "")
	return true
}

// ingestItemContent builds the library entry for one feed item. Content
// preference: embedded full content → fetch + readability-extract the linked
// page → RSS description.
func (s *FeedService) ingestItemContent(
	ctx context.Context,
	feed models.Feed,
	item *gofeed.Item,
) (*models.UserBook, error) {
	if item.Link == "" {
		return nil, errors.New("feed item has no link")
	}
	// arXiv items are ingested as papers (metadata + PDF from the arXiv API),
	// not readability-extracted rss articles — so an arXiv feed yields papers.
	if id, ok := arxivIDFromItem(item); ok {
		return s.ingest.IngestArxivByID(ctx, feed.UserID, id)
	}
	canonical, err := canonicalURL(item.Link)
	if err != nil {
		return nil, err
	}

	content := ArticleContent{ //nolint:exhaustruct // cover/excerpt set below
		SourceURL: canonical,
		BaseURL:   canonical,
		Category:  models.CategoryRSS,
		Title:     item.Title,
		Byline:    itemAuthor(item),
		HTML:      feedItemHTML(item),
	}
	if item.Description != "" {
		content.Excerpt = item.Description
	}
	if item.Image != nil {
		content.CoverURL = item.Image.URL
	}

	if content.HTML == "" {
		s.enrichFromLinkedPage(ctx, &content)
	}
	if content.HTML == "" {
		// Last resort: the RSS description as the article body.
		content.HTML = item.Description
	}
	if content.Title == "" {
		content.Title = canonical
	}
	if content.HTML == "" {
		// No content anywhere: track the item metadata-only (no file).
		// Add-by-URL later rebuilds the file if the page becomes readable.
		return s.ingestMetadataOnly(ctx, feed.UserID, content)
	}

	// RSS items never sync to Kobo (#640), so skip the EPUB build here and
	// just track the item; it's still readable in-app via content_html
	// below, and Add-by-URL can build a file later if needed.
	ub, err := s.ingestMetadataOnly(ctx, feed.UserID, content)
	if err != nil {
		return nil, err
	}

	// Persist the extracted HTML for in-app reading regardless of whether an
	// EPUB was built — the EPUB pipeline only ever used it transiently.
	if setErr := s.books.SetContentHTML(ctx, ub.BookID, content.HTML); setErr != nil {
		s.logger.WarnContext(ctx, "failed to store article content html",
			"bookID", ub.BookID, "error", setErr)
	}
	return ub, nil
}

// enrichFromLinkedPage fills the missing content fields by fetching and
// readability-extracting the item's linked page. Best-effort: any failure
// leaves content untouched (the description fallback applies afterwards).
func (s *FeedService) enrichFromLinkedPage(
	ctx context.Context,
	content *ArticleContent,
) {
	res, err := s.webFetch.Get(
		ctx, content.SourceURL,
		fetchOptions(maxArticleBytes, "text/html,application/xhtml+xml"),
	)
	if err != nil {
		s.logger.WarnContext(ctx, "feed item content fetch failed",
			"url", content.SourceURL, "error", err)
		return
	}
	if !isHTMLContentType(res.ContentType) {
		s.logger.WarnContext(ctx, "feed item content fetch returned non-HTML",
			"url", content.SourceURL, "contentType", res.ContentType)
		return
	}
	art, err := extractReadable(res.FinalURL, res.Body)
	if err != nil {
		s.logger.WarnContext(ctx, "feed item readability extraction failed",
			"url", content.SourceURL, "error", err)
		return
	}

	content.HTML = art.HTML
	content.BaseURL = res.FinalURL
	if content.Title == "" {
		content.Title = art.Title
	}
	if content.Byline == "" {
		content.Byline = art.Byline
	}
	if content.Excerpt == "" {
		content.Excerpt = art.Excerpt
	}
	if content.CoverURL == "" {
		content.CoverURL = art.ImageURL
	}
}

// BackfillContentHTML lazily fetches and stores the readability-extracted
// body for a book that has none yet — either ingested before content_html
// existed, or hit a since-fixed extraction bug. Returns the HTML it stored,
// or "" if it still couldn't get any; the caller falls back to the existing
// "no content" UI.
func (s *FeedService) BackfillContentHTML(
	ctx context.Context,
	book *models.Book,
) string {
	if book.SourceURL == nil || *book.SourceURL == "" {
		return ""
	}

	url := *book.SourceURL
	//nolint:exhaustruct // only the fields enrichFromLinkedPage reads/fills
	content := ArticleContent{SourceURL: url, BaseURL: url}
	s.enrichFromLinkedPage(ctx, &content)
	if content.HTML == "" {
		return ""
	}

	if err := s.books.SetContentHTML(ctx, book.ID, content.HTML); err != nil {
		s.logger.WarnContext(ctx, "failed to store backfilled article content html",
			"bookID", book.ID, "error", err)
	}
	return content.HTML
}

// ingestMetadataOnly creates the catalog row and user_book for a feed item
// whose content could not be fetched — the item is still tracked in the
// library, just without a stored file.
func (s *FeedService) ingestMetadataOnly(
	ctx context.Context,
	userID string,
	content ArticleContent,
) (*models.UserBook, error) {
	sourceURL := content.SourceURL
	book := models.Book{ //nolint:exhaustruct // catalog metadata only
		Title:     content.Title,
		Authors:   nil,
		Category:  content.Category,
		SourceURL: &sourceURL,
	}
	if content.Byline != "" {
		book.Authors = []string{content.Byline}
	}
	if content.Excerpt != "" {
		book.Description = &content.Excerpt
	}

	saved, err := s.ingest.booksRepo.UpsertBookBySourceURL(ctx, book)
	if err != nil {
		return nil, err
	}
	if _, err = s.ingest.ensureUserBook(ctx, userID, saved.ID); err != nil {
		return nil, err
	}
	return s.ingest.booksRepo.GetUserBook(ctx, userID, saved.ID)
}

func (s *FeedService) markSeen(
	ctx context.Context,
	feedID uuid.UUID,
	guid string,
	bookID *uuid.UUID,
	ingestErr string,
) {
	var errPtr *string
	if ingestErr != "" {
		errPtr = &ingestErr
	}
	if err := s.feeds.MarkItemSeen(ctx, feedID, guid, bookID, errPtr); err != nil {
		s.logger.WarnContext(ctx, "feed mark-seen failed",
			"feedID", feedID, "guid", guid, "error", err)
	}
}

// recordFetchResult persists poll outcome; res may be nil on failure.
func (s *FeedService) recordFetchResult(
	ctx context.Context,
	feedID uuid.UUID,
	res *webfetch.Result,
	fetchErr error,
) {
	var etag, lastModified, errStr *string
	if res != nil {
		if res.ETag != "" {
			etag = &res.ETag
		}
		if res.LastModified != "" {
			lastModified = &res.LastModified
		}
	}
	if fetchErr != nil {
		msg := fetchErr.Error()
		errStr = &msg
	}
	if err := s.feeds.SetFetchResult(
		ctx, feedID, etag, lastModified, errStr,
	); err != nil {
		s.logger.WarnContext(ctx, "feed fetch-result update failed",
			"feedID", feedID, "error", err)
	}
}

// arxivIDFromItem extracts an arXiv paper ID from a feed item's link or GUID
// (arXiv feeds put the abstract URL in either), reporting whether one matched.
func arxivIDFromItem(item *gofeed.Item) (string, bool) {
	if id, ok := arxiv.ParseID(item.Link); ok {
		return id, true
	}
	if item.GUID != "" {
		if id, ok := arxiv.ParseID(item.GUID); ok {
			return id, true
		}
	}
	return "", false
}

func itemGUID(item *gofeed.Item) string {
	if item.GUID != "" {
		return item.GUID
	}
	return item.Link
}

// feedItemHTML returns the item's embedded full content, if any. gofeed only
// maps <content:encoded> into item.Content when it resolves the "content"
// namespace prefix; feeds that declare it non-standardly still carry the
// value in item.Custom["encoded"] instead.
func feedItemHTML(item *gofeed.Item) string {
	if item.Content != "" {
		return item.Content
	}
	return item.Custom["encoded"]
}

func itemAuthor(item *gofeed.Item) string {
	for _, a := range item.Authors {
		if a != nil && a.Name != "" {
			return a.Name
		}
	}
	return ""
}
