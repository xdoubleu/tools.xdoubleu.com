package services

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"html"
	"log/slog"
	"math"
	"net/url"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"
	"github.com/mmcdole/gofeed"

	"tools.xdoubleu.com/apps/feeds/pkg/webfetch"
	"tools.xdoubleu.com/apps/podcasts/internal/models"
)

const (
	// addPollTimeout keeps adding a show under kamal-proxy's response timeout.
	addPollTimeout  = 10 * time.Second
	showPollTimeout = 45 * time.Second
	recordTimeout   = 5 * time.Second
	// maxFeedBytes bounds a feed document; podcast feeds with years of
	// episodes run to a few MiB.
	maxFeedBytes = int64(10 << 20)
	// maxEpisodesPerPoll keeps the newest episodes of a feed with thousands.
	maxEpisodesPerPoll = 100
	maxSummaryRunes    = 300
	feedAccept         = "application/rss+xml, application/xml, text/xml;q=0.9, */*;q=0.5"
)

var tagPattern = regexp.MustCompile(`<[^>]*>`)

// PollShow fetches a show's feed and stores its newest episodes. The outcome
// is recorded on the show either way; the error is for the caller to log.
func (s *ShowService) PollShow(ctx context.Context, show models.Show) error {
	opts := webfetch.Options{
		ETag: deref(show.ETag), LastModified: deref(show.LastModified),
		MaxBytes: maxFeedBytes, Accept: feedAccept,
	}
	res, err := s.fetcher.Get(ctx, show.FeedURL, opts)
	if err != nil {
		return s.fail(ctx, show, fetchErrorMessage(err), err)
	}
	if res.NotModified {
		return s.repo.RecordFetch(ctx, show.ID, show.ETag, show.LastModified, "")
	}

	parsed, err := gofeed.NewParser().Parse(bytes.NewReader(res.Body))
	if err != nil {
		return s.fail(ctx, show, "feed isn't valid RSS", err)
	}
	err = s.episodes.Upsert(ctx, show.ID, toEpisodes(show, parsed.Items))
	if err != nil {
		return s.fail(ctx, show, "couldn't store the episodes", err)
	}
	return s.repo.RecordFetch(
		ctx, show.ID, nonEmpty(res.ETag), nonEmpty(res.LastModified), "",
	)
}

// fail records a fetch problem, keeping the validators of the last good fetch.
func (s *ShowService) fail(
	ctx context.Context,
	show models.Show,
	message string,
	cause error,
) error {
	// Detached, so a deadline that cut the poll short still records why.
	recordCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), recordTimeout)
	defer cancel()
	if err := s.repo.RecordFetch(
		recordCtx, show.ID, show.ETag, show.LastModified, message,
	); err != nil {
		return errors.Join(cause, err)
	}
	return cause
}

// PollAll polls every favourite, least recently fetched first. One show's
// failure never stops the rest.
func (s *ShowService) PollAll(ctx context.Context, logger *slog.Logger) error {
	shows, err := s.repo.All(ctx)
	if err != nil {
		return err
	}
	for _, show := range shows {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		showCtx, cancel := context.WithTimeout(ctx, showPollTimeout)
		err = s.PollShow(showCtx, show)
		cancel()
		if err != nil {
			logger.WarnContext(ctx, "podcast feed poll failed",
				slog.String("show_id", show.ID.String()), slog.Any("error", err))
		}
	}
	return nil
}

func fetchErrorMessage(err error) string {
	var status *webfetch.StatusError
	switch {
	case errors.As(err, &status):
		return fmt.Sprintf("feed answered HTTP %d", status.Code)
	case errors.Is(err, webfetch.ErrTooLarge):
		return "feed is too large"
	default:
		return "couldn't reach the feed"
	}
}

// toEpisodes converts the newest maxEpisodesPerPoll items; feeds list
// either end first, and undated items count as oldest.
func toEpisodes(show models.Show, items []*gofeed.Item) []models.Episode {
	items = slices.Clone(items)
	slices.SortStableFunc(items, func(a, b *gofeed.Item) int {
		switch {
		case a.PublishedParsed == nil && b.PublishedParsed == nil:
			return 0
		case a.PublishedParsed == nil:
			return 1
		case b.PublishedParsed == nil:
			return -1
		default:
			return b.PublishedParsed.Compare(*a.PublishedParsed)
		}
	})

	out := make([]models.Episode, 0, min(len(items), maxEpisodesPerPoll))
	for _, item := range items {
		if len(out) == maxEpisodesPerPoll {
			break
		}
		guid := itemGUID(item)
		if guid == "" {
			continue
		}
		out = append(out, models.Episode{
			ID:              uuid.Nil,
			ShowID:          show.ID,
			GUID:            guid,
			Title:           strings.TrimSpace(item.Title),
			Summary:         summarize(item.Description),
			Link:            webURL(item.Link),
			AudioURL:        webURL(audioURL(item)),
			DurationSeconds: duration(item),
			PublishedAt:     item.PublishedParsed,
		})
	}
	return out
}

// itemGUID is the feed's own ID, else the audio URL, link, or title.
func itemGUID(item *gofeed.Item) string {
	for _, c := range []string{item.GUID, audioURL(item), item.Link, item.Title} {
		if c = strings.TrimSpace(c); c != "" {
			return c
		}
	}
	return ""
}

// webURL keeps only http(s) URLs: feed links end up in hrefs.
func webURL(raw string) string {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") {
		return ""
	}
	return u.String()
}

func audioURL(item *gofeed.Item) string {
	for _, e := range item.Enclosures {
		if e != nil && e.URL != "" {
			return e.URL
		}
	}
	return ""
}

// duration reads iTunes' "seconds", "mm:ss" or "hh:mm:ss"; unparseable is nil.
func duration(item *gofeed.Item) *int {
	if item.ITunesExt == nil {
		return nil
	}
	raw := strings.TrimSpace(item.ITunesExt.Duration)
	if raw == "" {
		return nil
	}
	total := 0
	for part := range strings.SplitSeq(raw, ":") {
		n, err := strconv.Atoi(part)
		if err != nil || n < 0 {
			return nil
		}
		total = total*60 + n //nolint:mnd // sexagesimal
		if total > math.MaxInt32 {
			return nil
		}
	}
	return &total
}

// summarize turns a feed's HTML description into short plain text.
func summarize(description string) string {
	text := html.UnescapeString(tagPattern.ReplaceAllString(description, " "))
	text = strings.Join(strings.Fields(text), " ")
	if utf8.RuneCountInString(text) <= maxSummaryRunes {
		return text
	}
	runes := []rune(text)
	return strings.TrimSpace(string(runes[:maxSummaryRunes])) + "…"
}

func deref(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

func nonEmpty(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}
