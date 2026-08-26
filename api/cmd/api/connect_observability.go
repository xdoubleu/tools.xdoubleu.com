package main

import (
	"context"
	"sort"
	"time"

	"connectrpc.com/connect"

	"tools.xdoubleu.com/apps/feeds"
	observabilityv1 "tools.xdoubleu.com/gen/observability/v1"
	"tools.xdoubleu.com/gen/observability/v1/observabilityv1connect"
	"tools.xdoubleu.com/internal/models"
)

type obsConnectHandler struct {
	app *Application
}

// storageScanRunner is the slice of *books.Books TriggerStorageScan
// needs, narrow so tests can substitute a stub instead of depending on a
// real R2 bucket.
type storageScanRunner interface {
	RunStorageScanNow(ctx context.Context) error
}

// unhealthyFeedLister is the slice of *feeds.Feeds GetUnhealthyFeeds needs,
// narrow so tests can substitute a stub. Distinct from
// jobs.unhealthyFeedLister (different return type — this returns
// feeds.UnhealthyFeed directly rather than jobs.UnhealthyFeed) since this
// handler reuses the feeds app's own type instead of going through
// main.go's feedsHealthAdapter, which exists only to keep the jobs package
// from importing apps/feeds.
type unhealthyFeedLister interface {
	ListUnhealthy(ctx context.Context) ([]feeds.UnhealthyFeed, error)
}

var _ observabilityv1connect.ObservabilityServiceHandler = (*obsConnectHandler)(nil)

// defaultWindowDays is used when a stats request omits window_days.
const defaultWindowDays = 30

// recentRunsLimit caps how many individual job runs are returned for the
// timeline / failure list.
const recentRunsLimit = 100

func windowSince(windowDays int32) time.Time {
	days := int(windowDays)
	if days <= 0 {
		days = defaultWindowDays
	}
	return time.Now().AddDate(0, 0, -days)
}

func (h *obsConnectHandler) GetJobStats(
	ctx context.Context,
	req *connect.Request[observabilityv1.GetJobStatsRequest],
) (*connect.Response[observabilityv1.GetJobStatsResponse], error) {
	if err := requireAdmin(ctx); err != nil {
		return nil, err
	}

	resp, err := h.jobStats(ctx, req.Msg.WindowDays)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	return connect.NewResponse(resp), nil
}

// jobStats runs the job-stats query and builds the response. It is shared by
// the Connect handler above and the MCP tool; neither the admin check nor the
// connect wrapping lives here.
func (h *obsConnectHandler) jobStats(
	ctx context.Context,
	windowDays int32,
) (*observabilityv1.GetJobStatsResponse, error) {
	since := windowSince(windowDays)

	stats, err := h.app.jobRunsRepo.Stats(ctx, since)
	if err != nil {
		return nil, err
	}
	runs, err := h.app.jobRunsRepo.ListRecent(ctx, since, recentRunsLimit)
	if err != nil {
		return nil, err
	}

	protoStats := make([]*observabilityv1.JobStat, len(stats))
	for i, s := range stats {
		protoStats[i] = &observabilityv1.JobStat{
			JobId:         s.JobID,
			TotalRuns:     s.TotalRuns,
			FailedRuns:    s.FailedRuns,
			AvgDurationMs: s.AvgDurationMs,
			LastRunAt:     s.LastRunAt.Format(time.RFC3339),
		}
	}

	protoRuns := make([]*observabilityv1.JobRun, len(runs))
	for i, r := range runs {
		protoRuns[i] = &observabilityv1.JobRun{
			JobId:      r.JobID,
			StartedAt:  r.StartedAt.Format(time.RFC3339),
			DurationMs: r.DurationMs,
			Success:    r.Success,
			Error:      r.Error,
		}
	}

	return &observabilityv1.GetJobStatsResponse{
		Stats:      protoStats,
		RecentRuns: protoRuns,
	}, nil
}

func (h *obsConnectHandler) GetUsageStats(
	ctx context.Context,
	req *connect.Request[observabilityv1.GetUsageStatsRequest],
) (*connect.Response[observabilityv1.GetUsageStatsResponse], error) {
	if err := requireAdmin(ctx); err != nil {
		return nil, err
	}

	resp, err := h.usageStats(ctx, req.Msg.WindowDays)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	return connect.NewResponse(resp), nil
}

func (h *obsConnectHandler) usageStats(
	ctx context.Context,
	windowDays int32,
) (*observabilityv1.GetUsageStatsResponse, error) {
	entries, err := h.app.usageRepo.GetDaily(ctx, windowSince(windowDays))
	if err != nil {
		return nil, err
	}

	protoEntries := make([]*observabilityv1.UsageDay, len(entries))
	for i, e := range entries {
		protoEntries[i] = &observabilityv1.UsageDay{
			Day:      e.Day.Format(time.DateOnly),
			App:      e.App,
			Endpoint: e.Endpoint,
			Count:    e.Count,
			Bytes:    e.Bytes,
		}
	}

	return &observabilityv1.GetUsageStatsResponse{
		Entries:    protoEntries,
		UnusedApps: h.unusedApps(entries),
	}, nil
}

// unusedApps returns the registered apps that logged no usage rows in the
// window, so an app nobody has touched doesn't just disappear from the
// response (issue #442).
func (h *obsConnectHandler) unusedApps(entries []models.UsageEntry) []string {
	used := make(map[string]bool, len(entries))
	for _, e := range entries {
		used[e.App] = true
	}

	unused := make([]string, 0, len(*h.app.apps))
	for _, a := range *h.app.apps {
		// dashboard owns no schema or routes of its own — it only ever
		// serves through games/books/feeds' exported methods — so it never
		// logs usage under its own name and would always show as unused.
		if a.GetName() == "dashboard" {
			continue
		}
		if !used[a.GetName()] {
			unused = append(unused, a.GetName())
		}
	}
	sort.Strings(unused)

	return unused
}

func (h *obsConnectHandler) GetStorageStats(
	ctx context.Context,
	_ *connect.Request[observabilityv1.GetStorageStatsRequest],
) (*connect.Response[observabilityv1.GetStorageStatsResponse], error) {
	if err := requireAdmin(ctx); err != nil {
		return nil, err
	}

	resp, err := h.storageStats(ctx)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	return connect.NewResponse(resp), nil
}

func (h *obsConnectHandler) storageStats(
	ctx context.Context,
) (*observabilityv1.GetStorageStatsResponse, error) {
	latest, err := h.app.storageRepo.Latest(ctx)
	if err != nil {
		// No snapshot yet is not an error — the scan has not run.
		latest = nil
	}

	history, err := h.app.storageRepo.History(ctx, windowSince(defaultWindowDays))
	if err != nil {
		return nil, err
	}

	protoHistory := make([]*observabilityv1.StorageSnapshot, len(history))
	for i, s := range history {
		snap := s
		protoHistory[i] = protoStorageSnapshot(&snap)
	}

	return &observabilityv1.GetStorageStatsResponse{
		Latest:  protoStorageSnapshot(latest),
		History: protoHistory,
	}, nil
}

func (h *obsConnectHandler) TriggerStorageScan(
	ctx context.Context,
	_ *connect.Request[observabilityv1.TriggerStorageScanRequest],
) (*connect.Response[observabilityv1.TriggerStorageScanResponse], error) {
	if err := requireAdmin(ctx); err != nil {
		return nil, err
	}

	// Runs the R2 list-bucket scan inline (seconds, admin-only, low
	// frequency) rather than via the async job queue, so the caller can
	// re-fetch GetStorageStats immediately after and see live data.
	if err := h.app.booksApp.RunStorageScanNow(ctx); err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	return connect.NewResponse(&observabilityv1.TriggerStorageScanResponse{}), nil
}

func protoStorageSnapshot(s *models.StorageSnapshot) *observabilityv1.StorageSnapshot {
	if s == nil {
		return nil
	}

	breakdown := make([]*observabilityv1.PrefixStat, len(s.PrefixBreakdown))
	for i, p := range s.PrefixBreakdown {
		breakdown[i] = &observabilityv1.PrefixStat{
			Prefix:    p.Prefix,
			SizeBytes: p.SizeBytes,
			Count:     p.Count,
		}
	}

	return &observabilityv1.StorageSnapshot{
		ScannedAt:            s.ScannedAt.Format(time.RFC3339),
		TotalSizeBytes:       s.TotalSizeBytes,
		ObjectCount:          s.ObjectCount,
		OrphanSizeBytes:      s.OrphanSizeBytes,
		OrphanCount:          s.OrphanCount,
		StaleUploadSizeBytes: s.StaleUploadSizeBytes,
		StaleUploadCount:     s.StaleUploadCount,
		PrefixBreakdown:      breakdown,
		OrphanKeys:           s.OrphanKeys,
	}
}

func (h *obsConnectHandler) GetDatabaseStats(
	ctx context.Context,
	_ *connect.Request[observabilityv1.GetDatabaseStatsRequest],
) (*connect.Response[observabilityv1.GetDatabaseStatsResponse], error) {
	if err := requireAdmin(ctx); err != nil {
		return nil, err
	}

	resp, err := h.databaseStats(ctx)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	return connect.NewResponse(resp), nil
}

func (h *obsConnectHandler) databaseStats(
	ctx context.Context,
) (*observabilityv1.GetDatabaseStatsResponse, error) {
	total, err := h.app.dbStatsRepo.TotalSize(ctx)
	if err != nil {
		return nil, err
	}
	schemas, err := h.app.dbStatsRepo.SchemaSizes(ctx)
	if err != nil {
		return nil, err
	}

	protoSchemas := make([]*observabilityv1.SchemaStat, len(schemas))
	for i, s := range schemas {
		protoSchemas[i] = &observabilityv1.SchemaStat{
			Name:       s.Name,
			SizeBytes:  s.SizeBytes,
			TableCount: s.TableCount,
		}
	}

	return &observabilityv1.GetDatabaseStatsResponse{
		TotalSizeBytes: total,
		Schemas:        protoSchemas,
	}, nil
}
