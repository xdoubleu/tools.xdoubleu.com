// Package consumption is the shared shape apps export their consumed items
// in, for the taste app's cross-domain log.
package consumption

import (
	"fmt"
	"time"
)

type Domain string

const (
	DomainBooks    Domain = "books"
	DomainFeeds    Domain = "feeds"
	DomainGames    Domain = "games"
	DomainMovies   Domain = "movies"
	DomainSeries   Domain = "series"
	DomainPodcasts Domain = "podcasts"
)

// Domains lists every domain in display order.
func Domains() []Domain {
	return []Domain{
		DomainBooks, DomainFeeds, DomainGames,
		DomainMovies, DomainSeries, DomainPodcasts,
	}
}

// Signal is engagement normalized within its domain.
type Signal int

const (
	StrongNegative Signal = -2
	Negative       Signal = -1
	Neutral        Signal = 0
	Positive       Signal = 1
	StrongPositive Signal = 2
)

// Event is one consumed item. A zero At means the date is unknown.
type Event struct {
	Domain  Domain
	ItemID  string
	Title   string
	Creator string
	Tags    []string
	At      time.Time
	Signal  Signal
	// Reason says why the item got its signal, e.g. "rated 5★".
	Reason string
}

// RatingSignal maps 1-5 stars onto the signal scale, 3 being neutral.
func RatingSignal(stars int) Signal {
	const neutralStars, maxStars = 3, 5
	return Signal(min(max(stars, 1), maxStars) - neutralStars)
}

// RatingReason is the Event.Reason for a star rating.
func RatingReason(stars int) string {
	return fmt.Sprintf("rated %d★", stars)
}

// Latest returns the latest non-nil time, or the zero time.
func Latest(times []*time.Time) time.Time {
	var latest time.Time
	for _, t := range times {
		if t != nil && t.After(latest) {
			latest = *t
		}
	}
	return latest
}
