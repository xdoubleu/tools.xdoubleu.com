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

// storageScanRunner is the slice of *books.Books TriggerStorageScan needs.
type storageScanRunner interface {
	RunStorageScanNow(ctx context.Context) error
}

// unhealthyFeedLister is the slice of *feeds.Feeds GetUnhealthyFeeds needs.
// Unlike jobs.unhealthyFeedLister, it returns feeds.UnhealthyFeed directly.
type unhealthyFeedLister interface {
	ListUnhealthy(ctx context.Context) ([]feeds.UnhealthyFeed, error)
}

var _ observabilityv1connect.ObservabilityServiceHandler = (*obsConnectHandler)(nil)

const defaultWindowDays = 30

// recentRunsLimit caps job runs returned for the timeline.
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

// jobStats builds the job-stats response for both the Connect handler and the
// MCP tool; auth is the caller's job.
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

// unusedApps returns registered apps with no usage rows in the window.
func (h *obsConnectHandler) unusedApps(entries []models.UsageEntry) []string {
	used := make(map[string]bool, len(entries))
	for _, e := range entries {
		used[e.App] = true
	}

	unused := make([]string, 0, len(*h.app.apps))
	for _, a := range *h.app.apps {
		// dashboard never logs usage under its own name.
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

	// Scan inline (fast, admin-only) so GetStorageStats shows live data right
	// after.
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
		ScannedAt:              s.ScannedAt.Format(time.RFC3339),
		TotalSizeBytes:         s.TotalSizeBytes,
		ObjectCount:            s.ObjectCount,
		OrphanSizeBytes:        s.OrphanSizeBytes,
		OrphanCount:            s.OrphanCount,
		StaleUploadSizeBytes:   s.StaleUploadSizeBytes,
		StaleUploadCount:       s.StaleUploadCount,
		PrefixBreakdown:        breakdown,
		OrphanKeys:             s.OrphanKeys,
		DeletedOrphanSizeBytes: s.DeletedOrphanSizeBytes,
		DeletedOrphanCount:     s.DeletedOrphanCount,
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

// databaseStats is a live size snapshot; history lives in Prometheus.
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
