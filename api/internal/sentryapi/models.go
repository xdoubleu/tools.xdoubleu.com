package sentryapi

import (
	"strconv"
	"time"
)

// Issue is the normalised representation of an unresolved Sentry issue.
type Issue struct {
	ID        string
	Title     string
	Culprit   string
	Permalink string
	Count     int64
	LastSeen  time.Time
	Level     string
	// Project is the slug of the configured project this issue came from —
	// set by the caller (fetch loops per project), not present on the wire.
	Project string
}

// issueWire is the subset of the Sentry issues API payload that is decoded.
// Sentry serialises the event count as a string (e.g. "42"), so it is parsed
// into an int64 by toIssue.
type issueWire struct {
	ID        string    `json:"id"`
	Title     string    `json:"title"`
	Culprit   string    `json:"culprit"`
	Permalink string    `json:"permalink"`
	Count     string    `json:"count"`
	LastSeen  time.Time `json:"lastSeen"`
	Level     string    `json:"level"`
}

// toIssue maps a wire issue to the exported Issue. A malformed count is
// treated as zero rather than an error — the count is informational.
func (w issueWire) toIssue() Issue {
	count, _ := strconv.ParseInt(w.Count, 10, 64)
	return Issue{
		ID:        w.ID,
		Title:     w.Title,
		Culprit:   w.Culprit,
		Permalink: w.Permalink,
		Count:     count,
		LastSeen:  w.LastSeen,
		Level:     w.Level,
		// Project is filled in by the caller (fetchAll loops per project).
		Project: "",
	}
}
