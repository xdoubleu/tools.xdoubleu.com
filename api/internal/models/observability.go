package models

import "time"

// JobRun is one execution of a background job.
type JobRun struct {
	JobID      string
	StartedAt  time.Time
	DurationMs int64
	Success    bool
	// Error is empty when the run succeeded.
	Error string
}

// JobStats aggregates the runs of one job over a time window.
type JobStats struct {
	JobID         string
	TotalRuns     int64
	FailedRuns    int64
	AvgDurationMs int64
	LastRunAt     time.Time
}

// UsageEntry is one (day, app, endpoint) request counter with bytes served.
type UsageEntry struct {
	Day      time.Time
	App      string
	Endpoint string
	Count    int64
	// Bytes is response bytes served, a proxy for (not a measure of) database
	// egress.
	Bytes int64
}

// PrefixStat aggregates object-store usage under one top-level key prefix.
type PrefixStat struct {
	Prefix    string `json:"prefix"`
	SizeBytes int64  `json:"size_bytes"`
	Count     int64  `json:"count"`
}

// StorageSnapshot is the result of one full object-store bucket scan.
type StorageSnapshot struct {
	ScannedAt            time.Time
	TotalSizeBytes       int64
	ObjectCount          int64
	OrphanSizeBytes      int64
	OrphanCount          int64
	StaleUploadSizeBytes int64
	StaleUploadCount     int64
	PrefixBreakdown      []PrefixStat
	// OrphanKeys is a capped sample (see maxOrphanKeys); OrphanCount counts all.
	OrphanKeys []string
	// DeletedOrphanSizeBytes/DeletedOrphanCount are the orphans this scan deleted
	// (older than orphanGracePeriod), a subset of OrphanSizeBytes/OrphanCount.
	DeletedOrphanSizeBytes int64
	DeletedOrphanCount     int64
}

// SchemaStats is the on-disk size of one database schema.
type SchemaStats struct {
	Name       string
	SizeBytes  int64
	TableCount int64
}

// LogEntry is a log line from api or web stored in global.log_entries.
type LogEntry struct {
	OccurredAt time.Time
	Source     string // "api" | "web"
	Level      string
	Message    string
	// AttrsJSON is the record's attributes as JSON; nil when there were none.
	AttrsJSON []byte
}

// TransactionTrend flags a transaction whose p95 regressed between two
// adjacent windows of global.transaction_latency_daily.
type TransactionTrend struct {
	Transaction    string
	Project        string
	PriorAvgP95Ms  float64
	RecentAvgP95Ms float64
	PctChange      float64
}

// OutboxMessage is one queued notification in global.notification_outbox.
type OutboxMessage struct {
	ID int64
	// To is empty for the fixed recipient configured on the mailer.
	To      string
	Subject string
	Body    string
	// Attempts counts sends started, including the current one.
	Attempts int
}

// OutboxDepth counts outbox rows by state.
type OutboxDepth struct {
	Pending int
	Failed  int
}
