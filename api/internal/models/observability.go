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

// UsageEntry is one (day, app, endpoint) request counter, with the response
// bytes those requests served.
type UsageEntry struct {
	Day      time.Time
	App      string
	Endpoint string
	Count    int64
	// Bytes is the total response body size served for this counter. It
	// measures what left the api, which is a proxy for what the api pulled
	// out of Postgres, not a direct measure of database egress — for
	// passthrough list endpoints the two track closely, which is the case
	// that matters (issue #1027).
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
}

// SchemaStats is the on-disk size of one database schema.
type SchemaStats struct {
	Name       string
	SizeBytes  int64
	TableCount int64
}

// HostMetricSample is one point-in-time reading of host resource usage,
// scraped from node_exporter (issue #1040).
type HostMetricSample struct {
	SampledAt     time.Time
	CPUPercent    float64
	MemoryPercent float64
	DiskPercent   float64
}

// LogEntry is one application log line forwarded from api (in-process) or
// web (HTTP ingest) into global.log_entries.
type LogEntry struct {
	OccurredAt time.Time
	Source     string // "api" | "web"
	Level      string
	Message    string
	// AttrsJSON is the log record's structured attributes, stored as opaque
	// JSON — nil when there were none.
	AttrsJSON []byte
}

// TransactionTrend flags a transaction (API endpoint or frontend page)
// whose p95 duration is regressing: PriorAvgP95Ms/RecentAvgP95Ms average
// global.transaction_latency_daily rows over two adjacent windows, and
// PctChange is the increase from the former to the latter.
type TransactionTrend struct {
	Transaction    string
	Project        string
	PriorAvgP95Ms  float64
	RecentAvgP95Ms float64
	PctChange      float64
}
