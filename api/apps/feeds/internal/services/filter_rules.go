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

// pollFilterRules loads the feed's rules once per poll, skipping the query
// when nothing is new. ok is false on failure: the poll then stores nothing,
// so the next one retries rather than letting filtered items through.
func (s *FeedService) pollFilterRules(
	ctx context.Context,
	feed models.Feed,
	newGUIDs []string,
) ([]models.FilterRule, bool) {
	if len(newGUIDs) == 0 {
		return nil, true
	}
	rules, err := s.filterRules.ListForFeed(ctx, feed.UserID, feed.ID)
	if err != nil {
		s.logger.WarnContext(ctx, "feed filter rules lookup failed",
			"feedID", feed.ID, "error", err)
		return nil, false
	}
	return rules, true
}

// filterEmailItem marks an inbound email filtered when a rule matches its
// title. A lookup failure ingests it unfiltered: the webhook can't retry.
func (s *FeedService) filterEmailItem(
	ctx context.Context,
	feed models.Feed,
	item *models.Item,
) {
	rules, err := s.filterRules.ListForFeed(ctx, feed.UserID, feed.ID)
	if err != nil {
		s.logger.WarnContext(ctx, "email feed filter rules lookup failed",
			"feedID", feed.ID, "error", err)
		return
	}
	markFiltered(item, matchFilterRule(rules, feed.ID, item.Title, nil))
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
