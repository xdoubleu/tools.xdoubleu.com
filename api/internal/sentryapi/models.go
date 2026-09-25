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
	// Project is set by the caller, not present on the wire.
	Project string
}

// issueWire.Count is a string on the wire; toIssue parses it.
type issueWire struct {
	ID        string    `json:"id"`
	Title     string    `json:"title"`
	Culprit   string    `json:"culprit"`
	Permalink string    `json:"permalink"`
	Count     string    `json:"count"`
	LastSeen  time.Time `json:"lastSeen"`
	Level     string    `json:"level"`
}

// toIssue treats a malformed count as zero; it's informational.
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
		Project:   "",
	}
}

// TransactionStat is one transaction's 24h p95 and request count from
// Discover; a broad sample, not only the slowest.
type TransactionStat struct {
	Transaction   string
	Project       string
	P95DurationMs float64
	RequestCount  int64
}

type transactionStatWire struct {
	Transaction string  `json:"transaction"`
	Project     string  `json:"project"`
	P95Duration float64 `json:"p95(span.duration)"`
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
