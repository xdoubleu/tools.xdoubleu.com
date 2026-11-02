package services

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"

	"tools.xdoubleu.com/apps/feeds/internal/models"
)

func rankItem(
	feed uuid.UUID,
	age time.Duration,
	now time.Time,
	cats ...string,
) models.Item {
	//nolint:exhaustruct // only the scored fields matter
	return models.Item{
		ID:          uuid.New(),
		FeedID:      feed,
		Categories:  cats,
		PublishedAt: now.Add(-age),
	}
}

func TestScoreItemPrefersWellReadFeed(t *testing.T) {
	now := time.Now()
	loved, ignored := uuid.New(), uuid.New()
	feeds := map[string]models.ReadStat{
		loved.String():   {Items: 20, Read: 18, Bookmarked: 0},
		ignored.String(): {Items: 20, Read: 1, Bookmarked: 0},
	}
	a := scoreItem(rankItem(loved, time.Hour, now), feeds, nil, now)
	b := scoreItem(rankItem(ignored, time.Hour, now), feeds, nil, now)
	assert.Greater(t, a, b)
}

func TestScoreItemPrefersReadAndBookmarkedCategories(t *testing.T) {
	now := time.Now()
	feed := uuid.New()
	cats := map[string]models.ReadStat{
		"go":    {Items: 10, Read: 9, Bookmarked: 5},
		"noise": {Items: 10, Read: 0, Bookmarked: 0},
	}
	good := scoreItem(rankItem(feed, time.Hour, now, "Go"), nil, cats, now)
	bad := scoreItem(rankItem(feed, time.Hour, now, "Noise"), nil, cats, now)
	assert.Greater(t, good, bad)
}

func TestScoreItemDecaysWithAge(t *testing.T) {
	now := time.Now()
	feed := uuid.New()
	fresh := scoreItem(rankItem(feed, time.Hour, now), nil, nil, now)
	old := scoreItem(rankItem(feed, 96*time.Hour, now), nil, nil, now)
	assert.Greater(t, fresh, old)
	assert.InDelta(t, fresh/4, old, fresh/20)
}

func TestScoreItemBoostsLastChance(t *testing.T) {
	now := time.Now()
	feed := uuid.New()
	plain := rankItem(feed, 100*time.Hour, now)
	fading := plain
	soon := now.Add(time.Hour)
	fading.ExpiresAt = &soon
	later := now.Add(5 * 24 * time.Hour)
	distant := plain
	distant.ExpiresAt = &later

	assert.Greater(t, scoreItem(fading, nil, nil, now), scoreItem(plain, nil, nil, now))
	assert.InDelta(
		t,
		scoreItem(plain, nil, nil, now),
		scoreItem(distant, nil, nil, now),
		1e-9,
	)
}

func TestRankItemsReturnsTopNBestFirst(t *testing.T) {
	now := time.Now()
	feed := uuid.New()
	items := []models.Item{
		rankItem(feed, 90*time.Hour, now),
		rankItem(feed, time.Hour, now),
		rankItem(feed, 30*time.Hour, now),
	}
	newest, mid := items[1].ID, items[2].ID

	top := rankItems(items, nil, nil, now, 2)

	assert.Len(t, top, 2)
	assert.Equal(t, newest, top[0].ID)
	assert.Equal(t, mid, top[1].ID)
}
