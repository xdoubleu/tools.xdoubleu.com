package services_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"tools.xdoubleu.com/apps/taste/internal/services"
	"tools.xdoubleu.com/internal/consumption"
)

type fakeSource struct {
	name   string
	events []consumption.Event
	err    error
}

func (f fakeSource) GetName() string { return f.name }

func all(string) bool { return true }

func (f fakeSource) ListConsumption(
	context.Context, string,
) ([]consumption.Event, error) {
	return f.events, f.err
}

func ev(
	domain consumption.Domain,
	title string,
	at time.Time,
	signal consumption.Signal,
) consumption.Event {
	//nolint:exhaustruct // only what the aggregation reads
	return consumption.Event{Domain: domain, Title: title, At: at, Signal: signal}
}

func TestEvents_MergesNewestFirstUndatedLast(t *testing.T) {
	jan := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	feb := time.Date(2026, 2, 1, 0, 0, 0, 0, time.UTC)
	log := services.NewLogService([]services.Source{
		fakeSource{name: "src", events: []consumption.Event{
			ev(consumption.DomainBooks, "Jan", jan, 1),
			ev(consumption.DomainBooks, "B undated", time.Time{}, 1),
		}, err: nil},
		fakeSource{name: "src", events: []consumption.Event{
			ev(consumption.DomainGames, "Feb", feb, 1),
			ev(consumption.DomainGames, "A undated", time.Time{}, 1),
		}, err: nil},
	})

	events, err := log.Events(context.Background(), "u", all)
	require.NoError(t, err)
	titles := make([]string, len(events))
	for i, e := range events {
		titles[i] = e.Title
	}
	assert.Equal(t, []string{"Feb", "Jan", "A undated", "B undated"}, titles)
}

func TestEvents_SourceErrorFails(t *testing.T) {
	log := services.NewLogService([]services.Source{
		fakeSource{name: "src", events: nil, err: errors.New("boom")},
	})
	_, err := log.Events(context.Background(), "u", all)
	assert.ErrorContains(t, err, "src: boom")
}

func TestEvents_SkipsExcludedSources(t *testing.T) {
	log := services.NewLogService([]services.Source{
		fakeSource{name: "hidden", events: nil, err: errors.New("never called")},
	})
	events, err := log.Events(context.Background(), "u",
		func(name string) bool { return name != "hidden" })
	require.NoError(t, err)
	assert.Empty(t, events)
}

func TestFilterAndIsDomain(t *testing.T) {
	events := []consumption.Event{
		ev(consumption.DomainBooks, "b", time.Time{}, 0),
		ev(consumption.DomainGames, "g", time.Time{}, 0),
	}
	assert.Len(t, services.Filter(events, ""), 2)
	got := services.Filter(events, consumption.DomainGames)
	require.Len(t, got, 1)
	assert.Equal(t, "g", got[0].Title)
	assert.Len(t, events, 2, "the input is left intact")

	assert.True(t, services.IsDomain(consumption.DomainSeries))
	assert.False(t, services.IsDomain("music"))
}

func TestBuildStats(t *testing.T) {
	now := time.Date(2026, 10, 15, 12, 0, 0, 0, time.UTC)
	thisMonth := time.Date(2026, 10, 2, 0, 0, 0, 0, time.UTC)
	tooOld := time.Date(2025, 9, 30, 0, 0, 0, 0, time.UTC)
	book := ev(consumption.DomainBooks, "Book", thisMonth, consumption.StrongPositive)
	book.Tags = []string{"Sci-Fi", "sci-fi ", ""}
	book.Creator = "Le Guin"
	movie := ev(consumption.DomainMovies, "Movie", tooOld, consumption.Positive)
	movie.Tags = []string{"sci-fi"}
	disliked := ev(consumption.DomainMovies, "Bad", time.Time{}, consumption.Negative)
	disliked.Tags = []string{"Horror"}
	disliked.Creator = "Nobody"

	stats := services.BuildStats([]consumption.Event{book, movie, disliked}, now)

	require.Len(t, stats.Domains, len(consumption.Domains()))
	assert.Equal(t, services.DomainStats{
		Domain: consumption.DomainBooks, Consumed: 1, Positive: 1, Negative: 0,
		LastAt: thisMonth,
	}, stats.Domains[0])
	movies := stats.Domains[3]
	assert.Equal(t, consumption.DomainMovies, movies.Domain)
	assert.Equal(t, 2, movies.Consumed)
	assert.Equal(t, 1, movies.Positive)
	assert.Equal(t, 1, movies.Negative)
	assert.Equal(t, tooOld, movies.LastAt)

	require.Len(t, stats.Months, 12)
	assert.Equal(t, "2025-11", stats.Months[0].Month)
	assert.Equal(t, "2026-10", stats.Months[11].Month)
	assert.Equal(t, 1, stats.Months[11].Counts[consumption.DomainBooks])
	for _, m := range stats.Months[:11] {
		assert.Empty(t, m.Counts)
	}

	require.Len(t, stats.TopTags, 1)
	assert.Equal(t, 2, stats.TopTags[0].Positive)
	assert.Equal(t, []consumption.Domain{
		consumption.DomainBooks, consumption.DomainMovies,
	}, stats.TopTags[0].Domains)
	assert.Equal(t, "Sci-Fi", stats.TopTags[0].Name)
	require.Len(t, stats.TopCreators, 1)
	assert.Equal(t, "Le Guin", stats.TopCreators[0].Name)
}

func TestBuildStats_RanksAndCaps(t *testing.T) {
	var events []consumption.Event
	for i := range 20 {
		for range i + 1 {
			e := ev(consumption.DomainFeeds, "x", time.Time{}, 1)
			e.Tags = []string{string(rune('a' + i))}
			events = append(events, e)
		}
	}
	tie := ev(consumption.DomainFeeds, "x", time.Time{}, 1)
	tie.Tags = []string{"T"}
	events = append(events, tie, tie)

	tags := services.BuildStats(events, time.Now()).TopTags
	require.Len(t, tags, 15)
	// "T" merges into "t", whose spelling is the more common one.
	assert.Equal(t, "t", tags[0].Name)
	assert.Equal(t, 22, tags[0].Positive)
	assert.Equal(t, "s", tags[1].Name)
}
