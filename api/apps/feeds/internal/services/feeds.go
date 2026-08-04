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
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/mmcdole/gofeed"

	"tools.xdoubleu.com/apps/feeds/internal/models"
	"tools.xdoubleu.com/apps/feeds/internal/repositories"
	"tools.xdoubleu.com/apps/feeds/pkg/webfetch"
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
// alias token.
const emailTokenBytes = 32

// maxItemsPerPoll caps how many new items one poll ingests per feed (newest
// first); older overflow is marked seen without ingesting.
const maxItemsPerPoll = 20

// FeedService manages RSS/Atom subscriptions and email-relay newsletter
// subscriptions (issue #595), ingesting their items directly as feeds.items
// rows — items are self-contained and never reference another app's schema.
type FeedService struct {
	logger        *slog.Logger
	feeds         *repositories.FeedsRepository
	items         *repositories.ItemsRepository
	webFetch      webfetch.Client
	inboundDomain string
}

// NewFeedService constructs a FeedService. inboundDomain is the
// EMAIL_INBOUND_DOMAIN used to build email feeds' inbound addresses; empty
// disables CreateEmail (see ErrEmailFeedsNotConfigured).
func NewFeedService(
	logger *slog.Logger,
	feeds *repositories.FeedsRepository,
	items *repositories.ItemsRepository,
	webFetchClient webfetch.Client,
	inboundDomain string,
) *FeedService {
	return &FeedService{
		logger:        logger,
		feeds:         feeds,
		items:         items,
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

// ListItems returns a page of items ingested by any of the user's feeds.
func (s *FeedService) ListItems(
	ctx context.Context,
	userID string,
	limit, offset int32,
	unreadOnly bool,
) ([]models.Item, bool, error) {
	return s.items.ListByUser(ctx, userID, limit, offset, unreadOnly)
}

// Create validates the URL by fetching and parsing it and stores the feed
// (with its self-reported title), then imports the feed's current contents
// as a first batch in the background. Returns the feed as soon as it is
// stored — the initial import can comfortably exceed the server's write
// timeout, so it must not block the request; the same items land within
// seconds via the detached import, or within the hour via the poll-feeds job
// if the process restarts mid-import.
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

	// ponytail: detached goroutine, not a job-queue task — a process restart
	// mid-import can drop it; the hourly poll-feeds job backfills.
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
// hash is persisted. Unlike Create, there is no background import: content
// only arrives as mail is received via the Resend webhook.
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
	// local-part in transit, which silently breaks a mixed-case token — hex
	// has no case ambiguity to mangle in the first place.
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

// IngestEmail ingests one inbound email as an item for the given email feed
// (issue #595) — the webhook-push counterpart to ingestItem for polled RSS
// items. messageID is Resend's email id, used to build a stable dedup guid
// alongside the feed ID. Best-effort: any ingest failure is recorded on the
// feed (visible in the UI's last-error) rather than returned, since the
// caller must still ack the webhook so Resend doesn't retry a
// permanently-broken email forever.
func (s *FeedService) IngestEmail(
	ctx context.Context,
	feed models.Feed,
	messageID, subject, htmlBody string,
) {
	guid := "mailto:" + feed.ID.String() + "/" + messageID
	//nolint:exhaustruct // read/dismissed/favourite/ingest_error start empty
	item := models.Item{
		FeedID:      feed.ID,
		GUID:        guid,
		Title:       titleOrDefault(subject, "Email newsletter"),
		SourceURL:   guid,
		ContentHTML: htmlBody,
		PublishedAt: time.Now(),
	}

	if err := s.items.Insert(ctx, item); err != nil {
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

// RecordEmailFetchFailure persists a fetch failure for an email feed when the
// inbound webhook itself couldn't retrieve the email body (e.g. Resend's
// receiving API erroring) — before IngestEmail is ever reached. Mirrors
// IngestEmail's own error branch so this failure is visible in the UI's
// last-error the same way an ingest failure already is.
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

// UpdateItem partially updates an item's read/dismissed/favourite state,
// scoped to userID. nil fields are left unchanged.
func (s *FeedService) UpdateItem(
	ctx context.Context,
	userID string,
	itemID uuid.UUID,
	read, dismissed, favourite *bool,
) (*models.Item, error) {
	return s.items.Update(ctx, userID, itemID, read, dismissed, favourite)
}

// Delete removes the subscription and every item it ingested (cascade via
// the feeds.items FK) — see FeedsRepository.Delete for why this no longer
// preserves engaged-with items the way reading's FeedService once did.
func (s *FeedService) Delete(
	ctx context.Context,
	userID string,
	id uuid.UUID,
) error {
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

// pollFeed fetches one feed (conditional GET) and ingests its new items,
// dispatching on source type — scrape feeds have no RSS/Atom body to parse.
func (s *FeedService) pollFeed(
	ctx context.Context,
	feed models.Feed,
) (int, error) {
	if feed.SourceType == models.FeedSourceScrape {
		return s.pollScrapeFeed(ctx, feed)
	}

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
// huge backlog never floods the reader.
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

	newGUIDs, err := s.items.FilterNewGUIDs(ctx, feed.ID, guids)
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
			// backlog is browsable up to the cap each poll.
			s.markSeenError(ctx, feed.ID, guid, "skipped: over per-poll cap")
			continue
		}
		if s.ingestItem(ctx, feed, byGUID[guid], guid) {
			ingested++
		}
	}
	return ingested
}

// ingestItem ingests one feed item; reports whether it produced a stored
// item. The guid is marked seen regardless of outcome — RefreshFeed is the
// only retry path, and it re-parses the live feed rather than replaying a
// single guid.
func (s *FeedService) ingestItem(
	ctx context.Context,
	feed models.Feed,
	item *gofeed.Item,
	guid string,
) bool {
	built, err := s.buildItem(ctx, feed, item, guid)
	if err != nil {
		s.logger.WarnContext(ctx, "feed item ingest failed",
			"feedID", feed.ID, "guid", guid, "error", err)
		s.markSeenError(ctx, feed.ID, guid, err.Error())
		return false
	}

	if err = s.items.Insert(ctx, *built); err != nil {
		s.logger.WarnContext(ctx, "feed item store failed",
			"feedID", feed.ID, "guid", guid, "error", err)
		return false
	}
	return true
}

// buildItem resolves one feed item's content. Content preference: embedded
// full content → fetch + readability-extract the linked page → RSS
// description.
func (s *FeedService) buildItem(
	ctx context.Context,
	feed models.Feed,
	item *gofeed.Item,
	guid string,
) (*models.Item, error) {
	if item.Link == "" {
		return nil, errors.New("feed item has no link")
	}
	canonical, err := canonicalURL(item.Link)
	if err != nil {
		return nil, err
	}

	title := item.Title
	html := feedItemHTML(item)
	if html == "" {
		html = s.fetchLinkedPageHTML(ctx, canonical, &title)
	}
	if html == "" {
		// Last resort: the RSS description as the item body.
		html = item.Description
	}
	title = titleOrDefault(title, canonical)

	var publishedAt time.Time
	if item.PublishedParsed != nil {
		publishedAt = *item.PublishedParsed
	}

	//nolint:exhaustruct // read/dismissed/favourite/ingest_error start empty
	return &models.Item{
		FeedID:      feed.ID,
		GUID:        guid,
		Title:       title,
		SourceURL:   canonical,
		ContentHTML: html,
		PublishedAt: publishedAt,
	}, nil
}

// fetchLinkedPageHTML fetches and readability-extracts the item's linked
// page. Best-effort: any failure returns "" (the description fallback
// applies afterwards); on success it may also fill in a missing title.
func (s *FeedService) fetchLinkedPageHTML(
	ctx context.Context,
	sourceURL string,
	title *string,
) string {
	res, err := s.webFetch.Get(
		ctx, sourceURL,
		fetchOptions(maxArticleBytes, "text/html,application/xhtml+xml"),
	)
	if err != nil {
		s.logger.WarnContext(ctx, "feed item content fetch failed",
			"url", sourceURL, "error", err)
		return ""
	}
	if !isHTMLContentType(res.ContentType) {
		s.logger.WarnContext(ctx, "feed item content fetch returned non-HTML",
			"url", sourceURL, "contentType", res.ContentType)
		return ""
	}
	art, err := extractReadable(res.FinalURL, res.Body)
	if err != nil {
		s.logger.WarnContext(ctx, "feed item readability extraction failed",
			"url", sourceURL, "error", err)
		return ""
	}

	if strings.TrimSpace(*title) == "" {
		*title = art.Title
	}
	return art.HTML
}

// titleOrDefault trims title and returns fallback if it is empty or
// whitespace-only.
func titleOrDefault(title, fallback string) string {
	if t := strings.TrimSpace(title); t != "" {
		return t
	}
	return fallback
}

func (s *FeedService) markSeenError(
	ctx context.Context,
	feedID uuid.UUID,
	guid, ingestErr string,
) {
	//nolint:exhaustruct // this is an error-only seen marker, no content
	item := models.Item{
		FeedID:      feedID,
		GUID:        guid,
		IngestError: &ingestErr,
	}
	if err := s.items.Insert(ctx, item); err != nil {
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
