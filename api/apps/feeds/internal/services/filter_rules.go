package services

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"

	"tools.xdoubleu.com/apps/feeds/internal/models"
)

// ErrInvalidFilterRule is returned for an unknown kind or a blank or
// overlong value.
var ErrInvalidFilterRule = errors.New("invalid filter rule")

// maxFilterRuleValueLen bounds a rule's value in bytes.
const maxFilterRuleValueLen = 200

// ListFilterRules returns the user's rules, oldest first, with how many
// items each filters.
func (s *FeedService) ListFilterRules(
	ctx context.Context,
	userID string,
) ([]models.FilterRule, error) {
	return s.filterRules.ListByUser(ctx, userID)
}

// CreateFilterRule stores a rule for one feed (feedID) or all the user's
// feeds (nil) and filters the matching unread, unbookmarked, undismissed
// items already stored; FilteredCount is how many. The value's whitespace is
// collapsed, like stored categories.
func (s *FeedService) CreateFilterRule(
	ctx context.Context,
	userID string,
	feedID *uuid.UUID,
	kind, value string,
) (*models.FilterRule, error) {
	if kind != models.FilterRuleKindCategory && kind != models.FilterRuleKindTitle {
		return nil, fmt.Errorf("%w: unknown kind %q", ErrInvalidFilterRule, kind)
	}
	value = strings.Join(strings.Fields(value), " ")
	if value == "" || len(value) > maxFilterRuleValueLen {
		return nil, fmt.Errorf(
			"%w: value must be 1-%d characters", ErrInvalidFilterRule,
			maxFilterRuleValueLen,
		)
	}
	return s.filterRules.Create(ctx, userID, feedID, kind, value)
}

// DeleteFilterRule removes a rule; the items it filtered stay filtered.
func (s *FeedService) DeleteFilterRule(
	ctx context.Context,
	userID string,
	id uuid.UUID,
) error {
	return s.filterRules.Delete(ctx, userID, id)
}

// feedFilterRules loads the rules applying to feed. A lookup failure is
// logged and returns none, so ingest proceeds unfiltered: a poll has
// already stored the feed's validators and a webhook can't be retried, so
// skipping ingest would lose the items.
func (s *FeedService) feedFilterRules(
	ctx context.Context,
	feed models.Feed,
) []models.FilterRule {
	rules, err := s.filterRules.ListForFeed(ctx, feed.UserID, feed.ID)
	if err != nil {
		s.logger.WarnContext(ctx, "feed filter rules lookup failed",
			"feedID", feed.ID, "error", err)
		return nil
	}
	return rules
}

// pollFilterRules is feedFilterRules, skipping the query when nothing is new.
func (s *FeedService) pollFilterRules(
	ctx context.Context,
	feed models.Feed,
	newGUIDs []string,
) []models.FilterRule {
	if len(newGUIDs) == 0 {
		return nil
	}
	return s.feedFilterRules(ctx, feed)
}

// markFiltered records rule on item; a nil rule leaves it unfiltered.
func markFiltered(item *models.Item, rule *models.FilterRule) {
	if rule == nil {
		return
	}
	now := time.Now()
	item.FilteredAt = &now
	item.FilteredRuleID = &rule.ID
}

// matchFilterRule returns the oldest rule applying to feedID (its own or a
// global one) that matches the item, or nil. The retroactive SQL in
// FilterRulesRepository.Create must match the same way.
func matchFilterRule(
	rules []models.FilterRule,
	feedID uuid.UUID,
	title string,
	categories []string,
) *models.FilterRule {
	var match *models.FilterRule
	for i := range rules {
		r := &rules[i]
		if r.FeedID != nil && *r.FeedID != feedID {
			continue
		}
		if !filterRuleMatches(*r, title, categories) {
			continue
		}
		if match == nil || r.CreatedAt.Before(match.CreatedAt) {
			match = r
		}
	}
	return match
}

func filterRuleMatches(r models.FilterRule, title string, categories []string) bool {
	switch r.Kind {
	case models.FilterRuleKindCategory:
		for _, c := range categories {
			if strings.EqualFold(c, r.Value) {
				return true
			}
		}
		return false
	case models.FilterRuleKindTitle:
		return strings.Contains(strings.ToLower(title), strings.ToLower(r.Value))
	default:
		return false
	}
}
