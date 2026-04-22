package helper_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"tools.xdoubleu.com/apps/backlog/internal/helper"
	"tools.xdoubleu.com/apps/backlog/internal/models"
)

func TestNewAchievementsGrapher_SeedsTodayDate(t *testing.T) {
	g := helper.NewAchievementsGrapher(map[int]int{})
	labels, values := g.ToSlices()
	today := time.Now().UTC().Format(models.ProgressDateFormat)
	assert.Contains(t, labels, today)
	require.Len(t, values, len(labels))
}

func TestAddPoint_SingleGame(t *testing.T) {
	totals := map[int]int{1: 10}
	g := helper.NewAchievementsGrapher(totals)

	date := time.Now().UTC()
	g.AddPoint(date, 1)

	labels, values := g.ToSlices()
	assert.NotEmpty(t, labels)
	assert.NotEmpty(t, values)
}

func TestAddPoint_BackfillsEarlierDate(t *testing.T) {
	totals := map[int]int{1: 4}
	g := helper.NewAchievementsGrapher(totals)

	// Add a point 3 days in the past — should backfill dates between then and today
	past := time.Now().UTC().AddDate(0, 0, -3)
	g.AddPoint(past, 1)

	labels, _ := g.ToSlices()
	// Should have at least 4 dates (the 3 past days + today)
	assert.GreaterOrEqual(t, len(labels), 4)
}

func TestAddPoint_FutureDateAppendsForward(t *testing.T) {
	totals := map[int]int{1: 4}
	g := helper.NewAchievementsGrapher(totals)

	future := time.Now().UTC().AddDate(0, 0, 2)
	g.AddPoint(future, 1)

	labels, _ := g.ToSlices()
	futureStr := future.Format(models.ProgressDateFormat)
	assert.Contains(t, labels, futureStr)
}

func TestAddPoint_SameDateTwice(t *testing.T) {
	totals := map[int]int{42: 10}
	g := helper.NewAchievementsGrapher(totals)

	date := time.Now().UTC()
	g.AddPoint(date, 42)
	g.AddPoint(date, 42)

	labels, values := g.ToSlices()
	assert.NotEmpty(t, labels)
	assert.NotEmpty(t, values)
}

func TestAddPoint_MultipleGames(t *testing.T) {
	totals := map[int]int{1: 10, 2: 5, 3: 20}
	g := helper.NewAchievementsGrapher(totals)

	now := time.Now().UTC()
	g.AddPoint(now, 1)
	g.AddPoint(now, 2)
	g.AddPoint(now.AddDate(0, 0, -1), 3)

	labels, values := g.ToSlices()
	assert.Len(t, values, len(labels))
}

func TestToSlices_EmptyGrapher(t *testing.T) {
	g := helper.NewAchievementsGrapher(map[int]int{})
	labels, values := g.ToSlices()
	// Always has at least today seeded
	assert.NotEmpty(t, labels)
	assert.Len(t, values, len(labels))
}

func TestToSlices_ZeroTotalAchievements(t *testing.T) {
	// If total is 0 for a game, completionRate is NaN (0/0) → should not panic
	totals := map[int]int{99: 0}
	g := helper.NewAchievementsGrapher(totals)
	g.AddPoint(time.Now().UTC(), 99)

	labels, values := g.ToSlices()
	assert.Len(t, values, len(labels))
}
