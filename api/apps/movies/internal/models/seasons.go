package models

import "time"

// Season is a series season with the user's watches of it. Number 0 is
// TMDB's "Specials"; a nil WatchedAt element is a watch with an unknown date.
type Season struct {
	Number       int
	Name         string
	AirDate      *time.Time
	EpisodeCount int
	WatchedAt    []*time.Time
}

// Counts reports whether the season takes part in the series status: TMDB's
// "Specials" never do.
func (s Season) Counts() bool {
	return s.Number > 0
}

// Aired reports whether the season had aired by now; an undated season has
// not.
func (s Season) Aired(now time.Time) bool {
	return s.AirDate != nil && !s.AirDate.After(now)
}

func (s Season) Ticked() bool {
	return len(s.WatchedAt) > 0
}

// SeriesStatus derives a series status from its season checklist: every aired
// season ticked is watched, any ticked season is watching, else want.
func SeriesStatus(seasons []Season, now time.Time) string {
	aired, airedTicked, ticked := 0, 0, 0
	for _, s := range seasons {
		if !s.Counts() {
			continue
		}
		if s.Ticked() {
			ticked++
		}
		if s.Aired(now) {
			aired++
			if s.Ticked() {
				airedTicked++
			}
		}
	}
	switch {
	case aired > 0 && airedTicked == aired:
		return StatusWatched
	case ticked > 0:
		return StatusWatching
	default:
		return StatusWant
	}
}

// WatchDate turns a picked day into its stored instant, 12:00 UTC, so the day
// reads the same in every timezone; nil is an unknown date.
func WatchDate(day *time.Time) *time.Time {
	if day == nil {
		return nil
	}
	d := time.Date(day.Year(), day.Month(), day.Day(), 12, 0, 0, 0, time.UTC)
	return &d
}
