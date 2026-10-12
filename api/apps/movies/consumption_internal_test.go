package movies

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"tools.xdoubleu.com/apps/movies/internal/models"
	"tools.xdoubleu.com/internal/consumption"
)

// entry builds an Entry with only what entrySignal reads.
func entry(status string, rating *int, watched []*time.Time) models.Entry {
	//nolint:exhaustruct // only what entrySignal reads
	return models.Entry{Status: status, Rating: rating, WatchedAt: watched}
}

func TestEntrySignal(t *testing.T) {
	two := 2
	day := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	tests := []struct {
		name   string
		entry  models.Entry
		signal consumption.Signal
		reason string
		ok     bool
	}{
		{"want", entry(models.StatusWant, nil, nil), 0, "", false},
		{
			"rating wins", entry(models.StatusDropped, &two, nil),
			consumption.Negative, "rated 2★", true,
		},
		{
			"dropped", entry(models.StatusDropped, nil, nil),
			consumption.Negative, "dropped", true,
		},
		{
			"watching", entry(models.StatusWatching, nil, nil),
			consumption.Neutral, "watching", true,
		},
		{
			"rewatched",
			entry(models.StatusWatched, nil, []*time.Time{&day, nil}),
			consumption.StrongPositive, "rewatched", true,
		},
		{
			"watched", entry(models.StatusWatched, nil, nil),
			consumption.Positive, "watched", true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			signal, reason, ok := entrySignal(tt.entry)
			assert.Equal(t, tt.signal, signal)
			assert.Equal(t, tt.reason, reason)
			assert.Equal(t, tt.ok, ok)
		})
	}
}

func TestEntryEvent_DatesFromLatestWatch(t *testing.T) {
	early := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	late := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	//nolint:exhaustruct // only the fields entryEvent reads
	e, ok := entryEvent(models.Entry{
		Status:    models.StatusWatched,
		Title:     models.Title{MediaType: "movie", TMDBID: 7, Title: "T"},
		WatchedAt: []*time.Time{&late, nil, &early},
		UpdatedAt: early,
	})
	assert.True(t, ok)
	assert.Equal(t, late, e.At)
	assert.Equal(t, "movie:7", e.ItemID)

	_, ok = entryEvent(entry(models.StatusWant, nil, nil))
	assert.False(t, ok)
}
