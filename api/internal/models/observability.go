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

// AutomatedAction is one run of an out-of-process routine, recorded by the
// routine or its workflow. Finish fields are empty while open.
type AutomatedAction struct {
	ID            int64
	FiredAt       time.Time
	TriggerSource string
	RoutineName   string
	FinishedAt    *time.Time
	// Outcome is "succeeded" | "failed" | "no_action_needed", empty while open.
	Outcome string
	PRURL   string
	Error   string
	// Metrics is nil unless the run was closed with them.
	Metrics *RunMetrics
}

// RunMetrics is what a routine run cost, measured from its agent transcript.
// CostUSD is the agent's estimate, not the provider's bill.
type RunMetrics struct {
	Requests          int32
	InputTokens       int64
	OutputTokens      int64
	ReasoningTokens   int64
	CacheReadTokens   int64
	CostUSD           float64
	DurationSeconds   float64
	ToolCalls         int32
	ToolErrors        int32
	RepeatedToolCalls int32
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
