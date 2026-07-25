// Package observability provides shared instrumentation: a job wrapper that
// sends Sentry Cron Monitor check-ins for every run and a request-usage
// recorder backing the admin dashboard.
package observability

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/getsentry/sentry-go"
	essentialogger "github.com/xdoubleu/essentia/v4/pkg/logging"
	"github.com/xdoubleu/essentia/v4/pkg/threading"
)

// TrackedJob decorates a threading.Job so every run — including ones the
// JobQueue would silently discard the error of — sends a Sentry Cron Monitor
// check-in (in_progress, then ok/error) and failures are logged at Error
// level (which the Sentry log handler forwards). Panics are captured as
// failed runs instead of killing the worker. The inner job's ID is
// preserved, so progress WebSocket topics and ForceRun keep working.
type TrackedJob struct {
	inner threading.Job
}

var _ threading.Job = (*TrackedJob)(nil)

func NewTrackedJob(inner threading.Job) *TrackedJob {
	return &TrackedJob{inner: inner}
}

func (j *TrackedJob) ID() string {
	return j.inner.ID()
}

func (j *TrackedJob) RunEvery() time.Duration {
	return j.inner.RunEvery()
}

func (j *TrackedJob) Run(ctx context.Context, logger *slog.Logger) (err error) {
	start := time.Now()
	slug := j.inner.ID()
	//nolint:exhaustruct // margin/timeout/timezone/thresholds use Sentry defaults
	monitorConfig := &sentry.MonitorConfig{
		Schedule: sentry.IntervalSchedule(
			int64(j.inner.RunEvery()/time.Minute), sentry.MonitorScheduleUnitMinute,
		),
	}

	hub := hubFromContext(ctx)
	//nolint:exhaustruct // ID/Duration are unset for the in_progress check-in
	checkInID := hub.CaptureCheckIn(&sentry.CheckIn{
		MonitorSlug: slug,
		Status:      sentry.CheckInStatusInProgress,
	}, monitorConfig)

	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("job panicked: %v", r)
		}

		status := sentry.CheckInStatusOK
		if err != nil {
			status = sentry.CheckInStatusError
			logger.ErrorContext(
				ctx,
				"job failed",
				slog.String("job", slug),
				essentialogger.ErrAttr(err),
			)
		}

		// checkInID is nil when no Sentry client is configured (e.g. local
		// dev without SENTRY_DSN) — the in_progress check-in above is then a
		// no-op, so leaving CheckIn.ID zero here just gets the same no-op.
		//nolint:exhaustruct // ID is set conditionally below
		checkIn := &sentry.CheckIn{
			MonitorSlug: slug,
			Status:      status,
			Duration:    time.Since(start),
		}
		if checkInID != nil {
			checkIn.ID = *checkInID
		}
		hub.CaptureCheckIn(checkIn, monitorConfig)
	}()

	return j.inner.Run(ctx, logger)
}

// hubFromContext returns the Sentry hub threaded onto ctx (see
// sentry.SetHubOnContext in cmd/api/main.go), falling back to the current
// hub if the context carries none.
func hubFromContext(ctx context.Context) *sentry.Hub {
	if hub := sentry.GetHubFromContext(ctx); hub != nil {
		return hub
	}
	return sentry.CurrentHub()
}
