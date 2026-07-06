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

// UsageEntry is one (day, app, endpoint) request counter.
type UsageEntry struct {
	Day      time.Time
	App      string
	Endpoint string
	Count    int64
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
