package services

import (
	"bytes"
	"context"
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

// maxItemsPerPoll caps how many new items one poll ingests per feed (newest
// first); older overflow is marked seen without ingesting.
const maxItemsPerPoll = 20

// FeedService manages RSS/Atom subscriptions and ingests their items into
// the library (category "rss") via IngestService.
type FeedService struct {
	logger     *slog.Logger
	feeds      *repositories.FeedsRepository
	ingest     *IngestService
	books      *BookService
	conversion *ConversionService
	webFetch   webfetch.Client
}

// NewFeedService constructs a FeedService.
func NewFeedService(
	logger *slog.Logger,
	feeds *repositories.FeedsRepository,
	ingest *IngestService,
	books *BookService,
	conversion *ConversionService,
	webFetchClient webfetch.Client,
) *FeedService {
	return &FeedService{
		logger:     logger,
		feeds:      feeds,
		ingest:     ingest,
		books:      books,
		conversion: conversion,
		webFetch:   webFetchClient,
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
	koboSync bool,
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
		UserID:   userID,
		URL:      canonical,
		Title:    parsed.Title,
		KoboSync: koboSync,
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

// Update changes the feed's title and kobo-sync flag.
func (s *FeedService) Update(
	ctx context.Context,
	userID string,
	id uuid.UUID,
	title string,
	koboSync bool,
) error {
	return s.feeds.Update(ctx, userID, id, title, koboSync)
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
func (s *FeedService) Refresh(
	ctx context.Context,
	userID string,
	id uuid.UUID,
) (int, error) {
	feed, err := s.feeds.GetByID(ctx, userID, id)
	if err != nil {
		return 0, err
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

	if feed.KoboSync {
		s.autoEnableKoboSync(ctx, feed.UserID, ub.BookID)
	}
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
		HTML:      item.Content,
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

	if feed.KoboSync {
		return s.ingest.IngestArticleContent(ctx, feed.UserID, content)
	}
	// Feed not opted into Kobo sync: skip the EPUB build (Calibre subprocess)
	// and just track the item; Add-by-URL or a later kobo_sync toggle can
	// convert it if needed.
	return s.ingestMetadataOnly(ctx, feed.UserID, content)
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
	if err != nil || !isHTMLContentType(res.ContentType) {
		return
	}
	art, err := extractReadable(res.FinalURL, res.Body)
	if err != nil {
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

// autoEnableKoboSync opts a freshly ingested item into Kobo sync and eagerly
// converts its EPUB to KEPUB (we are already on a background worker).
// Failures are logged; the standard retriable failed-kepub state applies.
func (s *FeedService) autoEnableKoboSync(
	ctx context.Context,
	userID string,
	bookID uuid.UUID,
) {
	if err := s.books.EnableKoboSync(ctx, userID, bookID); err != nil {
		s.logger.WarnContext(ctx, "feed kobo-sync enable failed",
			"bookID", bookID, "error", err)
		return
	}
	if _, err := s.conversion.EnsureKEPUB(ctx, userID, bookID); err != nil {
		s.logger.WarnContext(ctx, "feed kepub conversion failed",
			"bookID", bookID, "error", err)
	}
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

func itemAuthor(item *gofeed.Item) string {
	for _, a := range item.Authors {
		if a != nil && a.Name != "" {
			return a.Name
		}
	}
	return ""
}
