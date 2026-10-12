package movies

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"tools.xdoubleu.com/apps/movies/internal/models"
	"tools.xdoubleu.com/internal/consumption"
)

func TestEntrySignal(t *testing.T) {
	two := 2
	tests := []struct {
		name      string
		status    string
		rating    *int
		rewatched bool
		signal    consumption.Signal
		reason    string
		ok        bool
	}{
		{"want", models.StatusWant, nil, false, 0, "", false},
		{
			"rating wins",
			models.StatusDropped,
			&two,
			true,
			consumption.Negative,
			"rated 2★",
			true,
		},
		{"dropped", models.StatusDropped, nil, false, consumption.Negative, "dropped", true},
		{"watching", models.StatusWatching, nil, true, consumption.Neutral, "watching", true},
		{
			"rewatched",
			models.StatusWatched,
			nil,
			true,
			consumption.StrongPositive,
			"rewatched",
			true,
		},
		{"watched", models.StatusWatched, nil, false, consumption.Positive, "watched", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			signal, reason, ok := entrySignal(tt.status, tt.rating, tt.rewatched)
			assert.Equal(t, tt.signal, signal)
			assert.Equal(t, tt.reason, reason)
			assert.Equal(t, tt.ok, ok)
		})
	}
}

// entry builds an Entry with only what entryEvent reads.
func entry(mediaType, status string, watched []*time.Time) models.Entry {
	//nolint:exhaustruct // only what entryEvent reads
	return models.Entry{
		Status:    status,
		Title:     models.Title{MediaType: mediaType, TMDBID: 7, Title: "T"},
		WatchedAt: watched,
		UpdatedAt: time.Now(),
	}
}

func TestEntryEvent_Movie(t *testing.T) {
	early := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	late := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	none := models.SeasonWatchSummary{LatestAt: nil, Rewatched: false}

	e, ok := entryEvent(entry("movie", models.StatusWatched,
		[]*time.Time{&late, nil, &early}), none)
	assert.True(t, ok)
	assert.Equal(t, late, e.At)
	assert.Equal(t, "movie:7", e.ItemID)
	assert.Equal(t, consumption.DomainMovies, e.Domain)
	assert.Equal(t, "rewatched", e.Reason)

	e, _ = entryEvent(entry("movie", models.StatusWatched, []*time.Time{&early}), none)
	assert.Equal(t, "watched", e.Reason, "one watch is no rewatch")

	e, ok = entryEvent(entry("movie", models.StatusDropped, nil), none)
	assert.True(t, ok)
	assert.True(t, e.At.IsZero(), "an unknown date stays undated")

	_, ok = entryEvent(entry("movie", models.StatusWant, nil), none)
	assert.False(t, ok)
}

func TestEntryEvent_SeriesUsesSeasonWatches(t *testing.T) {
	late := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	e, ok := entryEvent(entry("series", models.StatusWatched, nil),
		models.SeasonWatchSummary{LatestAt: &late, Rewatched: true})
	assert.True(t, ok)
	assert.Equal(t, consumption.DomainSeries, e.Domain)
	assert.Equal(t, late, e.At)
	assert.Equal(t, consumption.StrongPositive, e.Signal)

	e, _ = entryEvent(entry("series", models.StatusWatched, nil),
		models.SeasonWatchSummary{LatestAt: nil, Rewatched: false})
	assert.True(t, e.At.IsZero())
	assert.Equal(t, "watched", e.Reason)
}

func result(mediaType, title, original string, year int) models.SearchResult {
	release := time.Date(year, 1, 1, 0, 0, 0, 0, time.UTC)
	//nolint:exhaustruct // only what pickTitle reads
	return models.SearchResult{Title: models.Title{
		MediaType: mediaType, Title: title, OriginalTitle: original, ReleaseDate: &release,
	}}
}

func TestPickTitle(t *testing.T) {
	results := []models.SearchResult{
		result("movie", "Dune: Part Two", "Dune: Part Two", 2024),
		result("series", "Dune", "Dune", 2000),
		result("movie", "Dune", "Dune", 1984),
		result("movie", "Dune", "Dune", 2021),
		result("movie", "Spirited Away", "千と千尋の神隠し", 2001),
	}
	assert.Equal(t, 1984, releaseYear(pickTitle(results, "movie", "Dune", 0).Title),
		"the first exact match without a year")
	assert.Equal(t, 2021, releaseYear(pickTitle(results, "movie", "Dune", 2021).Title))
	assert.Equal(t, 2000, releaseYear(pickTitle(results, "series", "Dune", 2021).Title))
	assert.Equal(t, "Spirited Away",
		pickTitle(results, "movie", "千と千尋の神隠し", 0).Title.Title)
	assert.Equal(t, "Dune: Part Two",
		pickTitle(results[:1], "movie", "Dune", 0).Title.Title, "a subtitle fallback")
	assert.Nil(t, pickTitle(results, "series", "Arrival", 0))
	//nolint:exhaustruct // no release date
	assert.Zero(t, releaseYear(models.Title{}))
}
