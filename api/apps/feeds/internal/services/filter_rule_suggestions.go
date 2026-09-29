package services

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"

	"tools.xdoubleu.com/apps/feeds/internal/models"
)

// A feed category is suggested for a rule once it has at least
// SuggestionMinItems items published within SuggestionWindow, of which at
// most SuggestionMaxReadPct percent are read.
const (
	SuggestionMinItems   = 10
	SuggestionMaxReadPct = 10
	SuggestionWindow     = 90 * 24 * time.Hour
)

// GetFilterRuleSuggestions returns the feed categories the user rarely
// reads that no category rule covers yet and the user hasn't dismissed.
func (s *FeedService) GetFilterRuleSuggestions(
	ctx context.Context,
	userID string,
) ([]models.FilterRuleSuggestion, error) {
	return s.filterRules.ListSuggestions(
		ctx, userID, time.Now().Add(-SuggestionWindow),
		SuggestionMinItems, SuggestionMaxReadPct,
	)
}

// DismissFilterRuleSuggestion stops suggesting feedID's category, compared
// ignoring case and with whitespace collapsed like stored categories.
func (s *FeedService) DismissFilterRuleSuggestion(
	ctx context.Context,
	userID string,
	feedID uuid.UUID,
	category string,
) error {
	category = strings.Join(strings.Fields(category), " ")
	if category == "" || len(category) > maxFilterRuleValueLen {
		return fmt.Errorf(
			"%w: category must be 1-%d characters", ErrInvalidFilterRule,
			maxFilterRuleValueLen,
		)
	}
	return s.filterRules.DismissSuggestion(ctx, userID, feedID, category)
}
