package models_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"tools.xdoubleu.com/apps/movies/internal/models"
)

//nolint:gochecknoglobals //fixed clock
var now = time.Date(2026, 5, 1, 12, 0, 0, 0, time.UTC)

func season(number int, airDate *time.Time, watches int) models.Season {
	return models.Season{
		Number: number, Name: "", AirDate: airDate, EpisodeCount: 0,
		WatchedAt: make([]*time.Time, watches),
	}
}

func at(d time.Duration) *time.Time {
	t := now.Add(d)
	return &t
}

func TestSeason_Counts(t *testing.T) {
	assert.False(t, season(0, nil, 0).Counts())
	assert.True(t, season(1, nil, 0).Counts())
}

func TestSeason_Aired(t *testing.T) {
	assert.False(t, season(1, nil, 0).Aired(now))
	assert.True(t, season(1, at(0), 0).Aired(now), "airing today")
	assert.True(t, season(1, at(-time.Hour), 0).Aired(now))
	assert.False(t, season(1, at(time.Hour), 0).Aired(now))
}

func TestSeason_Ticked(t *testing.T) {
	assert.False(t, season(1, nil, 0).Ticked())
	assert.True(t, season(1, nil, 1).Ticked(), "an unknown-date watch ticks")
}

func TestSeriesStatus(t *testing.T) {
	past, future := at(-time.Hour), at(time.Hour)
	cases := map[string]struct {
		seasons []models.Season
		want    string
	}{
		"no seasons": {nil, models.StatusWant},
		"nothing ticked": {
			[]models.Season{season(1, past, 0)}, models.StatusWant,
		},
		"only specials ticked": {
			[]models.Season{season(0, past, 1), season(1, past, 0)},
			models.StatusWant,
		},
		"specials alone never complete a series": {
			[]models.Season{season(0, past, 1)}, models.StatusWant,
		},
		"some aired ticked": {
			[]models.Season{season(1, past, 1), season(2, past, 0)},
			models.StatusWatching,
		},
		"every aired ticked": {
			[]models.Season{
				season(0, past, 0), season(1, past, 1), season(2, past, 2),
				season(3, future, 0),
			},
			models.StatusWatched,
		},
		"only an unaired season ticked": {
			[]models.Season{season(1, future, 1)}, models.StatusWatching,
		},
		"unaired ticked, aired not": {
			[]models.Season{season(1, past, 0), season(2, future, 1)},
			models.StatusWatching,
		},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			assert.Equal(t, c.want, models.SeriesStatus(c.seasons, now))
		})
	}
}

func TestWatchDate(t *testing.T) {
	assert.Nil(t, models.WatchDate(nil))

	day := time.Date(2020, 3, 4, 23, 30, 0, 0, time.FixedZone("CET", 3600))
	assert.Equal(t,
		time.Date(2020, 3, 4, 12, 0, 0, 0, time.UTC), *models.WatchDate(&day))
}
