package jobs

import (
	"context"
	"log/slog"
	"time"
)

// RouterRefreshJob rebuilds the CSA index so its rolling window slides
// forward. refresh is injected as a func to avoid depending on csa.Index.
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
