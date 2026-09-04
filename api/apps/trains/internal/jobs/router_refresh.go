package jobs

import (
	"context"
	"log/slog"
	"time"
)

// RouterRefreshJob periodically rebuilds the in-memory CSA router index so
// its rolling window slides forward and picks up the latest static import
// (issue #1391). The startup run is desirable — a fresh replica starts with
// no index built at all. refresh is services.JourneyService.Refresh,
// injected as a func to avoid this package depending on the csa.Index
// return type.
type RouterRefreshJob struct {
	refresh func(ctx context.Context) error
}

func NewRouterRefreshJob(refresh func(ctx context.Context) error) *RouterRefreshJob {
	return &RouterRefreshJob{refresh: refresh}
}

func (j *RouterRefreshJob) ID() string { return "trains-router-refresh" }

func (j *RouterRefreshJob) RunEvery() time.Duration {
	const refreshEvery = 6 * time.Hour
	return refreshEvery
}

func (j *RouterRefreshJob) Run(ctx context.Context, _ *slog.Logger) error {
	return j.refresh(ctx)
}
