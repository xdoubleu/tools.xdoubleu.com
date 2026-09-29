package services

import (
	"context"
	"strings"

	"github.com/google/uuid"
)

// normalizeCategories collapses whitespace, drops empties and removes
// case-insensitive duplicates, keeping the first spelling.
func normalizeCategories(raw []string) []string {
	var out []string
	seen := make(map[string]bool, len(raw))
	for _, c := range raw {
		c = strings.Join(strings.Fields(c), " ")
		key := strings.ToLower(c)
		if c == "" || seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, c)
	}
	return out
}

// backfillCategories writes freshly parsed categories onto the guids that
// are already stored (in guids but not newGUIDs) and still have none. It
// never fetches content.
func (s *FeedService) backfillCategories(
	ctx context.Context,
	feedID uuid.UUID,
	guids, newGUIDs []string,
	categoriesOf func(guid string) []string,
) {
	isNew := make(map[string]bool, len(newGUIDs))
	for _, g := range newGUIDs {
		isNew[g] = true
	}

	stored := map[string][]string{}
	for _, g := range guids {
		if isNew[g] {
			continue
		}
		if categories := categoriesOf(g); len(categories) > 0 {
			stored[g] = categories
		}
	}

	if err := s.items.FillMissingCategories(ctx, feedID, stored); err != nil {
		s.logger.WarnContext(ctx, "feed category backfill failed",
			"feedID", feedID, "error", err)
	}
}
