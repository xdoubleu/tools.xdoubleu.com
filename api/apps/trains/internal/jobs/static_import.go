// Package jobs holds the trains app's background jobs.
package jobs

import (
	"context"
	"log/slog"
	"time"
)

// staticImporter is the slice of services.StaticImportService the job needs.
type staticImporter interface {
	Import(ctx context.Context) error
}

// StaticImportJob refreshes the SNCB GTFS static timetable once a day. The
// startup run is desirable (a fresh replica has an empty schema) and a
// conditional GET makes an unchanged feed nearly free.
type StaticImportJob struct {
	svc staticImporter
}

func NewStaticImportJob(svc staticImporter) *StaticImportJob {
	return &StaticImportJob{svc: svc}
}

func (j *StaticImportJob) ID() string { return "trains-static-import" }

func (j *StaticImportJob) RunEvery() time.Duration {
	const hoursInDay = 24
	return hoursInDay * time.Hour
}

func (j *StaticImportJob) Run(ctx context.Context, _ *slog.Logger) error {
	return j.svc.Import(ctx)
}
