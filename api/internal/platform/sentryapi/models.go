package sentryapi

import (
	"encoding/json"
	"strconv"
	"strings"
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

// maxEventSpans caps the db spans returned per event.
const maxEventSpans = 200

// Span is one db span of an Event; Offender marks spans Sentry's performance
// detector flagged.
type Span struct {
	Op          string
	Description string
	DurationMs  float64
	Offender    bool
}

// Event is an issue's latest event, reduced to its db spans.
type Event struct {
	ID          string
	Title       string
	DateCreated time.Time
	Spans       []Span
}

type eventWire struct {
	EventID     string    `json:"eventID"`
	Title       string    `json:"title"`
	DateCreated time.Time `json:"dateCreated"`
	Entries     []struct {
		Type string          `json:"type"`
		Data json.RawMessage `json:"data"`
	} `json:"entries"`
	Occurrence *struct {
		EvidenceData struct {
			OffenderSpanIDs []string `json:"offenderSpanIds"`
		} `json:"evidenceData"`
	} `json:"occurrence"`
}

type spanWire struct {
	SpanID         string  `json:"span_id"`
	Op             string  `json:"op"`
	Description    string  `json:"description"`
	StartTimestamp float64 `json:"start_timestamp"`
	Timestamp      float64 `json:"timestamp"`
}

// toEvent keeps only db.* spans; a malformed spans entry yields no spans
// since it's diagnostic detail.
func (w eventWire) toEvent() Event {
	offenders := map[string]bool{}
	if w.Occurrence != nil {
		for _, id := range w.Occurrence.EvidenceData.OffenderSpanIDs {
			offenders[id] = true
		}
	}

	spans := []Span{}
	for _, e := range w.Entries {
		if e.Type != "spans" {
			continue
		}
		var raw []spanWire
		if err := json.Unmarshal(e.Data, &raw); err != nil {
			continue
		}
		for _, sp := range raw {
			if !strings.HasPrefix(sp.Op, "db") || len(spans) >= maxEventSpans {
				continue
			}
			spans = append(spans, Span{
				Op:          sp.Op,
				Description: sp.Description,
				DurationMs:  (sp.Timestamp - sp.StartTimestamp) * 1000, //nolint:mnd // s to ms
				Offender:    offenders[sp.SpanID],
			})
		}
	}
	return Event{ID: w.EventID, Title: w.Title, DateCreated: w.DateCreated, Spans: spans}
}
