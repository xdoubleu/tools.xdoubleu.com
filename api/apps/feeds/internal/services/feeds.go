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
	"tools.xdoubleu.com/internal/mailer"
	"tools.xdoubleu.com/internal/notifications"
	globalrepositories "tools.xdoubleu.com/internal/repositories"
)

// ErrInvalidFeed is returned when a subscribed URL does not parse as RSS/Atom.
var ErrInvalidFeed = errors.New("url is not a valid RSS/Atom feed")

// ErrEmailFeedsNotConfigured is returned by CreateEmail when
// EMAIL_INBOUND_DOMAIN is unset.
var ErrEmailFeedsNotConfigured = errors.New(
	"email feeds are not configured (EMAIL_INBOUND_DOMAIN unset)",
)

// emailTokenBytes is the random byte length of an inbound alias token.
const emailTokenBytes = 32

// maxItemsPerPoll caps how many new items one poll ingests per feed (newest
// first); older overflow is marked seen without ingesting.
const maxItemsPerPoll = 20

// errorNotifyThreshold is the unbroken poll failures before a problem email.
const errorNotifyThreshold = 3

// quietCheckHistory is how many recent items feed the quiet-feed heuristic.
const quietCheckHistory = 6

// statsHistoryDays bounds the items-per-day histogram window.
const statsHistoryDays = 90

// A feed is quiet once it's gone quietGapMultiplier times its average posting
// gap without a new item, floored at quietMinGap for low-volume feeds.
const quietGapMultiplier = 3

const quietMinGap = 48 * time.Hour

// FeedService manages RSS/Atom, scrape and email-relay subscriptions,
// ingesting their items as self-contained feeds.items rows.
type FeedService struct {
	logger        *slog.Logger
	feeds         *repositories.FeedsRepository
	items         *repositories.ItemsRepository
	webFetch      webfetch.Client
	inboundDomain string
	notifications *notifications.Service
	users         *globalrepositories.AppUsersRepository
	webURL        string
}

// NewFeedService constructs a FeedService. An empty inboundDomain disables
// CreateEmail.
func NewFeedService(
	logger *slog.Logger,
	feeds *repositories.FeedsRepository,
	items *repositories.ItemsRepository,
	webFetchClient webfetch.Client,
	inboundDomain string,
	notifications *notifications.Service,
	users *globalrepositories.AppUsersRepository,
	webURL string,
) *FeedService {
	return &FeedService{
		logger:        logger,
		feeds:         feeds,
		items:         items,
		webFetch:      webFetchClient,
		inboundDomain: inboundDomain,
		notifications: notifications,
		users:         users,
		webURL:        webURL,
	}
}

// List returns the user's feeds.
func (s *FeedService) List(
	ctx context.Context,
	userID string,
) ([]models.Feed, error) {
	return s.feeds.List(ctx, userID)
}

// ListItems returns a page of the user's items, optionally restricted to one
// feed or to bookmarked items.
func (s *FeedService) ListItems(
	ctx context.Context,
	userID string,
	limit, offset int32,
	unreadOnly bool,
	feedID *uuid.UUID,
	bookmarkedOnly bool,
) ([]models.Item, bool, error) {
	return s.items.ListByUser(
		ctx, userID, limit, offset, unreadOnly, feedID, bookmarkedOnly,
	)
}

// GetItem returns one of the user's items with its article body, which
// ListItems omits.
func (s *FeedService) GetItem(
	ctx context.Context,
	userID string,
	itemID uuid.UUID,
) (*models.Item, error) {
	return s.items.GetByIDForUser(ctx, userID, itemID)
}

// CountUnread returns the user's unread item count across all feeds.
func (s *FeedService) CountUnread(ctx context.Context, userID string) (int, error) {
	return s.items.CountUnread(ctx, userID)
}

// Create validates the URL by fetching and parsing it, stores the feed and
// imports its contents in the background: the import can exceed the write
// timeout, and the poll-feeds job backfills if it's dropped.
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

	// Detached, not queued: a restart can drop it; poll-feeds backfills.
	importFeed := *feed
	go func() {
		importCtx := context.WithoutCancel(ctx)
		s.processItems(importCtx, importFeed, parsed.Items)
		s.recordFetchResult(importCtx, importFeed.ID, res, nil)
	}()
	return feed, nil
}

// CreateEmail mints an inbound email alias and stores the feed. The returned
// plaintext address is never available again (only its hash is stored).
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
	// Hex, not base64: some relays lowercase the recipient local-part.
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

// GetByInboundTokenHash resolves an email feed by its token's SHA-256 hash.
func (s *FeedService) GetByInboundTokenHash(
	ctx context.Context,
	hash string,
) (*models.Feed, error) {
	return s.feeds.GetByInboundTokenHash(ctx, hash)
}

// IngestEmail ingests one inbound email as an item, deduped on Resend's
// messageID. Failures are recorded on the feed rather than returned, since
// the webhook must still be acked so Resend doesn't retry forever.
func (s *FeedService) IngestEmail(
	ctx context.Context,
	feed models.Feed,
	messageID, subject, htmlBody string,
) {
	guid := "mailto:" + feed.ID.String() + "/" + messageID
	//nolint:exhaustruct // read/dismissed/bookmarked/ingest_error start empty
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
		s.recordFetchResultRaw(ctx, feed.ID, nil, nil, &errStr)
		return
	}

	s.recordFetchResultRaw(ctx, feed.ID, nil, nil, nil)
}

// RecordEmailFetchFailure records a failure to retrieve an inbound email's
// body, surfaced like an IngestEmail failure.
func (s *FeedService) RecordEmailFetchFailure(
	ctx context.Context,
	feedID uuid.UUID,
	fetchErr error,
) {
	errStr := fetchErr.Error()
	s.recordFetchResultRaw(ctx, feedID, nil, nil, &errStr)
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

// UpdateItem partially updates an item's state; nil fields are unchanged.
// readProgressPct is clamped to [0,100] and only ever increases.
func (s *FeedService) UpdateItem(
	ctx context.Context,
	userID string,
	itemID uuid.UUID,
	read, dismissed, bookmarked *bool,
	readProgressPct *int32,
) (*models.Item, error) {
	if readProgressPct != nil {
		clamped := clampPct(*readProgressPct)
		readProgressPct = &clamped
	}
	return s.items.Update(
		ctx,
		userID,
		itemID,
		read,
		dismissed,
		bookmarked,
		readProgressPct,
	)
}

// maxPct is the upper clamp bound for a 0-100 percentage value.
const maxPct = 100

func clampPct(v int32) int32 {
	switch {
	case v < 0:
		return 0
	case v > maxPct:
		return maxPct
	default:
		return v
	}
}

// GetStats returns per-feed cadence/read stats plus an items-per-day
// histogram over the last statsHistoryDays.
func (s *FeedService) GetStats(
	ctx context.Context,
	userID string,
) ([]models.FeedStats, []models.DayCount, error) {
	stats, err := s.items.Stats(ctx, userID)
	if err != nil {
		return nil, nil, err
	}
	perDay, err := s.items.ItemsPerDay(
		ctx, userID, time.Now().AddDate(0, 0, -statsHistoryDays),
	)
	if err != nil {
		return nil, nil, err
	}
	return stats, perDay, nil
}

// Delete removes the feed and, via FK cascade, every item it ingested.
func (s *FeedService) Delete(
	ctx context.Context,
	userID string,
	id uuid.UUID,
) error {
	return s.feeds.Delete(ctx, userID, id)
}

// Refresh polls one feed synchronously and returns how many items it
// ingested; a no-op for push-only email feeds.
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

// PollAll polls every feed; per-feed failures are recorded, never abort.
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

// ListUnhealthy returns every failing feed across all users.
func (s *FeedService) ListUnhealthy(ctx context.Context) ([]models.Feed, error) {
	return s.feeds.ListUnhealthy(ctx)
}

// CountUnreadByFeed returns unread item counts per feed across all users.
func (s *FeedService) CountUnreadByFeed(
	ctx context.Context,
) ([]models.FeedUnreadCount, error) {
	return s.items.CountUnreadByFeed(ctx)
}

// pollFeed fetches one feed (conditional GET) and ingests new items.
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

// processItems ingests unseen items newest first, capped at maxItemsPerPoll;
// overflow is marked seen so a huge backlog never floods the reader.
func (s *FeedService) processItems(
	ctx context.Context,
	feed models.Feed,
	items []*gofeed.Item,
) int {
	// Newest first; unparsed dates last in feed order.
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
			s.markSeenError(ctx, feed.ID, guid, "skipped: over per-poll cap")
			continue
		}
		if s.ingestItem(ctx, feed, byGUID[guid], guid) {
			ingested++
		}
	}
	return ingested
}

// ingestItem ingests one feed item and reports whether it stored one. The
// guid is marked seen regardless; RefreshFeed re-parses the live feed.
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

// buildItem resolves item content: embedded content, else the extracted
// linked page, else the RSS description.
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
		html = item.Description
	}
	title = titleOrDefault(title, canonical)

	var publishedAt time.Time
	if item.PublishedParsed != nil {
		publishedAt = *item.PublishedParsed
	}

	//nolint:exhaustruct // read/dismissed/bookmarked/ingest_error start empty
	return &models.Item{
		FeedID:      feed.ID,
		GUID:        guid,
		Title:       title,
		SourceURL:   canonical,
		ContentHTML: html,
		PublishedAt: publishedAt,
	}, nil
}

// fetchLinkedPageHTML extracts the linked page, returning "" on any failure;
// it may also fill in a missing title.
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

// titleOrDefault returns the trimmed title, or fallback if blank.
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
	s.recordFetchResultRaw(ctx, feedID, etag, lastModified, errStr)
}

// recordFetchResultRaw persists a poll/ingest outcome and checks whether a
// problem email is due.
func (s *FeedService) recordFetchResultRaw(
	ctx context.Context,
	feedID uuid.UUID,
	etag, lastModified, fetchErr *string,
) {
	updated, err := s.feeds.SetFetchResult(ctx, feedID, etag, lastModified, fetchErr)
	if err != nil {
		s.logger.WarnContext(ctx, "feed fetch-result update failed",
			"feedID", feedID, "error", err)
		return
	}
	s.checkFeedHealth(ctx, *updated)
}

// checkFeedHealth sends a problem email (deduped via notified_at) when a
// feed crosses the failure threshold or looks quiet, and clears notified_at
// on recovery from either, since both share the one dedup column.
func (s *FeedService) checkFeedHealth(ctx context.Context, feed models.Feed) {
	if feed.LastError != nil {
		if feed.ConsecutiveFailures >= errorNotifyThreshold && feed.NotifiedAt == nil {
			s.notifyProblem(ctx, feed, "failing to load: "+*feed.LastError)
		}
		return
	}

	times, err := s.items.RecentPublishedAt(ctx, feed.ID, quietCheckHistory)
	if err != nil {
		s.logger.WarnContext(ctx, "feed quiet-check lookup failed",
			"feedID", feed.ID, "error", err)
		return
	}

	quiet := isFeedQuiet(times, time.Now())
	switch {
	case quiet && feed.NotifiedAt == nil:
		s.notifyProblem(ctx, feed, "hasn't posted new content in longer than usual")
	case !quiet && feed.NotifiedAt != nil:
		if clearErr := s.feeds.ClearNotified(ctx, feed.ID); clearErr != nil {
			s.logger.WarnContext(ctx, "feed notified-clear failed",
				"feedID", feed.ID, "error", clearErr)
		}
	}
}

// minQuietHistory is the fewest items needed to establish a cadence.
const minQuietHistory = 3

// isFeedQuiet reports whether a feed has gone quiet relative to its own
// cadence. times need not be sorted.
func isFeedQuiet(times []time.Time, now time.Time) bool {
	if len(times) < minQuietHistory {
		return false
	}

	sorted := make([]time.Time, len(times))
	copy(sorted, times)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i].Before(sorted[j]) })

	var totalGap time.Duration
	for i := 1; i < len(sorted); i++ {
		totalGap += sorted[i].Sub(sorted[i-1])
	}
	avgGap := totalGap / time.Duration(len(sorted)-1)

	threshold := avgGap * quietGapMultiplier
	if threshold < quietMinGap {
		threshold = quietMinGap
	}

	latest := sorted[len(sorted)-1]
	return now.Sub(latest) > threshold
}

// notifyProblem queues a problem email to the feed owner. MarkNotified runs
// only after a successful send, so a failed send retries next poll;
// mailer.ErrNotConfigured degrades rather than fails.
func (s *FeedService) notifyProblem(
	ctx context.Context,
	feed models.Feed,
	reason string,
) {
	user, err := s.users.GetByID(ctx, feed.UserID)
	if err != nil {
		s.logger.WarnContext(ctx, "feed notify: user lookup failed",
			"feedID", feed.ID, "error", err)
		return
	}

	title := titleOrDefault(feed.Title, feed.URL)
	subject := fmt.Sprintf("Feed %q needs attention", title)
	body := fmt.Sprintf(
		"Your feed %q is %s.\n\nView it: %s/feeds",
		title, reason, s.webURL,
	)
	s.notifications.EnqueueTo(
		user.Email,
		subject,
		body,
		func(ctx context.Context, err error) error {
			if err != nil {
				if !errors.Is(err, mailer.ErrNotConfigured) {
					s.logger.WarnContext(ctx, "feed notify: send failed",
						"feedID", feed.ID, "error", err)
				}
				return nil
			}
			if markErr := s.feeds.MarkNotified(ctx, feed.ID); markErr != nil {
				s.logger.WarnContext(ctx, "feed notify: mark-notified failed",
					"feedID", feed.ID, "error", markErr)
			}
			return nil
		},
	)
}

func itemGUID(item *gofeed.Item) string {
	if item.GUID != "" {
		return item.GUID
	}
	return item.Link
}

// feedItemHTML returns the item's embedded full content. Feeds declaring the
// content namespace non-standardly carry it in item.Custom["encoded"].
func feedItemHTML(item *gofeed.Item) string {
	if item.Content != "" {
		return item.Content
	}
	return item.Custom["encoded"]
}
