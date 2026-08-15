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

// TransactionStat is one Sentry transaction's (an API endpoint or a
// frontend page/route) p95 duration and request count over the last 24h,
// as sampled from Sentry's org-level Events (Discover) API. Not
// necessarily one of the slowest — ListTransactionStats returns a broad
// sample; callers sort/truncate as needed for their own view (a live "top
// N slowest" list, or a full daily snapshot for trend detection).
type TransactionStat struct {
	Transaction   string
	Project       string
	P95DurationMs float64
	RequestCount  int64
}

// transactionStatWire is the subset of one row of Sentry's Discover
// events payload that is decoded — see client.go's transactionStatsFields.
type transactionStatWire struct {
	Transaction string  `json:"transaction"`
	Project     string  `json:"project"`
	P95Duration float64 `json:"p95(transaction.duration)"`
	Count       float64 `json:"count()"`
}

func (w transactionStatWire) toTransactionStat() TransactionStat {
	return TransactionStat{
		Transaction:   w.Transaction,
		Project:       w.Project,
		P95DurationMs: w.P95Duration,
		RequestCount:  int64(w.Count),
	}
}
