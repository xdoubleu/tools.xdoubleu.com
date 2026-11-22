// Package protomap maps the feeds domain models to their proto messages, for
// the Connect and MCP adapters.
package protomap

import (
	"time"

	"tools.xdoubleu.com/apps/feeds/internal/models"
	feedsv1 "tools.xdoubleu.com/gen/feeds/v1"
)

func Feed(f models.Feed) *feedsv1.Feed {
	lastFetched := ""
	if f.LastFetchedAt != nil {
		lastFetched = f.LastFetchedAt.Format(time.RFC3339)
	}
	lastError := ""
	if f.LastError != nil {
		lastError = *f.LastError
	}
	etag := ""
	if f.ETag != nil {
		etag = *f.ETag
	}
	lastModified := ""
	if f.LastModified != nil {
		lastModified = *f.LastModified
	}
	notifiedAt := ""
	if f.NotifiedAt != nil {
		notifiedAt = f.NotifiedAt.Format(time.RFC3339)
	}
	//nolint:gosec // failure count never approaches int32 range
	consecutiveFailures := int32(f.ConsecutiveFailures)
	return &feedsv1.Feed{
		Id:                  f.ID.String(),
		Url:                 f.URL,
		Title:               f.Title,
		LastFetchedAt:       lastFetched,
		LastError:           lastError,
		CreatedAt:           f.CreatedAt.Format(time.RFC3339),
		SourceType:          f.SourceType,
		Etag:                etag,
		LastModified:        lastModified,
		ConsecutiveFailures: consecutiveFailures,
		ExpiryDays:          int32(f.ExpiryDays), //nolint:gosec // capped at 3650
		NotifiedAt:          notifiedAt,
	}
}

// Item maps the model to its wire message.
func Item(item models.Item) *feedsv1.Item {
	readAt := ""
	if item.ReadAt != nil {
		readAt = item.ReadAt.Format(time.RFC3339)
	}
	ingestError := ""
	if item.IngestError != nil {
		ingestError = *item.IngestError
	}
	publishedAt := ""
	if !item.PublishedAt.IsZero() {
		publishedAt = item.PublishedAt.Format(time.RFC3339)
	}
	var filterRule *feedsv1.FilterRule
	if item.FilterRule != nil {
		filterRule = FilterRule(*item.FilterRule)
	}
	return &feedsv1.Item{
		Id:              item.ID.String(),
		FeedId:          item.FeedID.String(),
		Title:           item.Title,
		SourceUrl:       item.SourceURL,
		ContentHtml:     item.ContentHTML,
		PublishedAt:     publishedAt,
		ReadAt:          readAt,
		Dismissed:       item.Dismissed,
		Bookmarked:      item.Bookmarked,
		IngestError:     ingestError,
		CreatedAt:       item.CreatedAt.Format(time.RFC3339),
		ReadProgressPct: int32(item.ReadProgressPct), //nolint:gosec // clamped [0,100]
		HasContent:      item.HasContent,
		Categories:      item.Categories,
		FilteredAt:      formatOptionalTime(item.FilteredAt),
		FilterRule:      filterRule,
		RestoredAt:      formatOptionalTime(item.RestoredAt),
		ExpiredAt:       formatOptionalTime(item.ExpiredAt),
		ExpiresAt:       formatOptionalTime(item.ExpiresAt),
	}
}

// formatOptionalTime formats t as RFC3339, or "" when nil.
func formatOptionalTime(t *time.Time) string {
	if t == nil {
		return ""
	}
	return t.Format(time.RFC3339)
}

// FilterRule maps the model to its wire message.
func FilterRule(r models.FilterRule) *feedsv1.FilterRule {
	kind := feedsv1.FilterRuleKind_FILTER_RULE_KIND_TITLE
	if r.Kind == models.FilterRuleKindCategory {
		kind = feedsv1.FilterRuleKind_FILTER_RULE_KIND_CATEGORY
	}
	feedID := ""
	if r.FeedID != nil {
		feedID = r.FeedID.String()
	}
	return &feedsv1.FilterRule{
		Id:            r.ID.String(),
		FeedId:        feedID,
		Kind:          kind,
		Value:         r.Value,
		CreatedAt:     r.CreatedAt.Format(time.RFC3339),
		FilteredCount: int32(r.FilteredCount), //nolint:gosec // item counts fit int32
	}
}

// FilterRuleSuggestion maps the model to its wire message.
func FilterRuleSuggestion(
	s models.FilterRuleSuggestion,
) *feedsv1.FilterRuleSuggestion {
	return &feedsv1.FilterRuleSuggestion{
		FeedId:    s.FeedID.String(),
		FeedTitle: s.FeedTitle,
		FeedUrl:   s.FeedURL,
		Category:  s.Category,
		ItemCount: int32(s.ItemCount), //nolint:gosec // item counts fit int32
		ReadCount: int32(s.ReadCount), //nolint:gosec // item counts fit int32
		ReadRate:  float64(s.ReadCount) / float64(s.ItemCount),
	}
}
