package services

import (
	"context"
	"math"
	"sort"
	"strings"
	"time"

	"tools.xdoubleu.com/apps/feeds/internal/models"
)

const (
	// TopItemsCount is the length of the ranked "Today's top" list.
	TopItemsCount = 10

	day               = 24 * time.Hour
	rankingHistory    = 90 * day
	rankingCandidates = 300
	// Laplace prior: a feed or category with no history reads as priorRate.
	priorRate   = 0.3
	priorWeight = 2.0
	// halfLife halves an item's score every 48h of age.
	halfLife = 2 * day
	half     = 0.5
	// Weights of the feed read rate, the category read rate and the
	// category bookmark rate.
	weightFeed     = 0.4
	weightCategory = 0.4
	weightBookmark = 0.4
	// lastChanceBoost lifts items about to expire, undecayed.
	lastChanceBoost = 0.15
)

// smoothedRate is part/total pulled towards priorRate for small samples.
func smoothedRate(part, total int) float64 {
	return (float64(part) + priorRate*priorWeight) / (float64(total) + priorWeight)
}

// scoreItem scores one candidate from the user's own history: the feed's and
// the item's categories' read-through rates, a boost for categories the user
// bookmarks, decayed by age, plus a flat bump when it is about to expire.
func scoreItem(
	item models.Item,
	feeds map[string]models.ReadStat,
	cats map[string]models.ReadStat,
	now time.Time,
) float64 {
	feed := feeds[item.FeedID.String()]
	feedRate := smoothedRate(feed.Read, feed.Items)

	catRate, catBookmark := feedRate, smoothedRate(feed.Bookmarked, feed.Items)-priorRate
	if len(item.Categories) > 0 {
		var rateSum, bmSum float64
		for _, c := range item.Categories {
			st := cats[strings.ToLower(c)]
			rateSum += smoothedRate(st.Read, st.Items)
			bmSum += smoothedRate(st.Bookmarked, st.Items) - priorRate
		}
		n := float64(len(item.Categories))
		catRate, catBookmark = rateSum/n, bmSum/n
	}

	base := weightFeed*feedRate + weightCategory*catRate +
		weightBookmark*math.Max(catBookmark, 0)
	age := math.Max(now.Sub(item.PublishedAt).Hours(), 0)
	score := base * math.Pow(half, age/halfLife.Hours())
	if item.ExpiresAt != nil && item.ExpiresAt.Sub(now) <= LastChanceWindow {
		score += lastChanceBoost
	}
	return score
}

// rankItems returns the top n items by score, best first.
func rankItems(
	items []models.Item,
	feeds map[string]models.ReadStat,
	cats map[string]models.ReadStat,
	now time.Time,
	n int,
) []models.Item {
	scores := make(map[string]float64, len(items))
	for _, it := range items {
		scores[it.ID.String()] = scoreItem(it, feeds, cats, now)
	}
	sort.SliceStable(items, func(a, b int) bool {
		return scores[items[a].ID.String()] > scores[items[b].ID.String()]
	})
	if len(items) > n {
		items = items[:n]
	}
	return items
}

// ListTopItems returns today's best TopItemsCount items: the unread ones
// plus those read since the start of the UTC day (so the list ends instead
// of refilling), ranked by scoreItem.
func (s *FeedService) ListTopItems(
	ctx context.Context,
	userID string,
) ([]models.Item, error) {
	now := time.Now().UTC()
	dayStart := now.Truncate(day)
	items, err := s.items.ListRankingCandidates(
		ctx, userID, dayStart, rankingCandidates,
	)
	if err != nil {
		return nil, err
	}
	feedStats, catStats, err := s.items.ReadStats(ctx, userID, now.Add(-rankingHistory))
	if err != nil {
		return nil, err
	}
	feeds := make(map[string]models.ReadStat, len(feedStats))
	for id, st := range feedStats {
		feeds[id.String()] = st
	}
	return rankItems(items, feeds, catStats, now, TopItemsCount), nil
}
