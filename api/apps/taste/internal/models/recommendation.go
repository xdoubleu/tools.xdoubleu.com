package models

import (
	"time"

	"tools.xdoubleu.com/internal/consumption"
)

const (
	TrackMore    = "more"
	TrackBroaden = "broaden"
)

// IsTrack reports whether t is a known track.
func IsTrack(t string) bool {
	return t == TrackMore || t == TrackBroaden
}

// Feedback on a pick; FeedbackNone clears it.
const (
	FeedbackNone          = ""
	FeedbackHit           = "hit"
	FeedbackMiss          = "miss"
	FeedbackNotInterested = "not_interested"
	FeedbackKnown         = "known"
)

// IsFeedback reports whether f is a known feedback value, including none.
func IsFeedback(f string) bool {
	switch f {
	case FeedbackNone, FeedbackHit, FeedbackMiss, FeedbackNotInterested, FeedbackKnown:
		return true
	}
	return false
}

// Source is a web page the generator cited for a pick.
type Source struct {
	URL   string `json:"url"`
	Title string `json:"title"`
}

// Recommendation is a verified, scored pick.
type Recommendation struct {
	ID          string
	Domain      string
	Track       string
	Title       string
	Creator     string
	Year        int
	CatalogRef  string
	URL         string
	ImageURL    string
	Why         string
	Sources     []Source
	Score       int
	GeneratedAt time.Time
	Feedback    string
	Saved       bool
	Accepted    bool
	// OnMyServices marks a movie or series streaming on the user's services.
	OnMyServices bool
}

// HasBacklog reports whether accepting a pick in domain can add it to a
// backlog: books, movies and series.
func HasBacklog(domain string) bool {
	switch consumption.Domain(domain) { //nolint:exhaustive // the rest have none
	case consumption.DomainBooks, consumption.DomainMovies, consumption.DomainSeries:
		return true
	}
	return false
}

// CanAccept reports whether accepting r adds it to its app's backlog, rather
// than saving it.
func (r Recommendation) CanAccept() bool {
	return HasBacklog(r.Domain) && r.CatalogRef != ""
}

// HitRate counts how a domain and track's picks landed.
type HitRate struct {
	Domain string
	Track  string
	Hits   int
	Misses int
}

// PicksStatus is the state of a user's latest picks run.
type PicksStatus struct {
	// GeneratedAt is when the latest picks were made; nil before the first.
	GeneratedAt *time.Time
	// Error is the last run's note; empty when it succeeded.
	Error string
	// RequestedAt is when picks were last asked for; nil when never.
	RequestedAt *time.Time
	// Pending means a requested run hasn't started yet or is running.
	Pending bool
}
