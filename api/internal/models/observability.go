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
	// OrphanKeys is a capped sample of the orphaned object keys — see
	// maxOrphanKeys in apps/books/internal/jobs/storage_scan.go. OrphanCount
	// tallies every orphan found even when this list is truncated.
	OrphanKeys []string
	// DeletedOrphanSizeBytes/DeletedOrphanCount cover orphans this same scan
	// actually deleted (past orphanGracePeriod, so an object whose book_files
	// row hasn't committed yet during an in-flight upload is never wrongly
	// removed) — a subset of OrphanSizeBytes/OrphanCount, which still counts
	// every orphan seen regardless of age or delete outcome.
	DeletedOrphanSizeBytes int64
	DeletedOrphanCount     int64
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

// WorkflowRunSample is one recorded GitHub Actions workflow run, persisted so
// duration/failure history survives past github.Client's 45s in-memory
// cache (issue #1217).
type WorkflowRunSample struct {
	RunID        int64
	WorkflowName string
	Branch       string
	Event        string
	Conclusion   string
	URL          string
	DurationMs   int64
	StartedAt    time.Time
	CompletedAt  time.Time
}

// WorkflowJobSample is one job within a recorded workflow run — the
// "specific actions" duration breakdown.
type WorkflowJobSample struct {
	RunID       int64
	JobName     string
	Conclusion  string
	DurationMs  int64
	StartedAt   time.Time
	CompletedAt time.Time
}

// WorkflowDurationStat aggregates a workflow's recorded run durations over
// the retention window.
type WorkflowDurationStat struct {
	WorkflowName  string
	AvgDurationMs float64
	P95DurationMs float64
	RunCount      int64
}

// JobDurationStat aggregates one job name's recorded durations across every
// workflow run over the retention window — the per-action breakdown.
type JobDurationStat struct {
	JobName       string
	AvgDurationMs float64
	P95DurationMs float64
	RunCount      int64
}

// AlertState is one threshold rule's breach/recovery state
// (jobs.ThresholdAlertJob, issue #1283). Unlike the one-shot dedup
// global.notified_issues uses, a rule re-arms on recovery: Breaching flips
// back to false and Since is cleared, so a second incident notifies again.
// CurrentValue/Threshold are refreshed on every evaluation, not just on a
// breach/recovery transition, so a read always reflects the latest sample.
type AlertState struct {
	RuleKey        string
	Breaching      bool
	Since          *time.Time
	LastNotifiedAt *time.Time
	CurrentValue   float64
	Threshold      float64
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

// TransactionLatencyPoint is one global.transaction_latency_daily row,
// returned flat for GetTransactionLatencyHistory — the client pivots and
// selects series for its multi-line chart.
type TransactionLatencyPoint struct {
	Day           time.Time
	Project       string
	Transaction   string
	P95DurationMs float64
	RequestCount  int64
}

// TableSizeSample is one table's on-disk size at sampling time, scraped live
// from pg_total_relation_size by DBSizeSnapshotJob (issue #1282).
type TableSizeSample struct {
	SchemaName string
	TableName  string
	SizeBytes  int64
}

// TableSizeGrowth compares one table's current size against its earliest
// recorded size within a requested window: DeltaBytes/PctChange is the
// growth from EarliestSizeBytes to CurrentSizeBytes.
type TableSizeGrowth struct {
	SchemaName        string
	TableName         string
	CurrentSizeBytes  int64
	EarliestSizeBytes int64
	DeltaBytes        int64
	PctChange         float64
}

// DBSizeSnapshot is one snapshot batch's total size, summed across every table
// sampled at that instant.
type DBSizeSnapshot struct {
	SampledAt      time.Time
	TotalSizeBytes int64
}

// TableSizeHistoryPoint is one global.db_size_samples row, returned flat for
// GetDatabaseSizeHistory — the client pivots and selects series for its
// multi-line chart, same shape as TransactionLatencyPoint.
type TableSizeHistoryPoint struct {
	Day        time.Time
	SchemaName string
	TableName  string
	SizeBytes  int64
}
