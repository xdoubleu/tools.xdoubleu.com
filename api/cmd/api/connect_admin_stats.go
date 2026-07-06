package main

import (
	"context"
	"time"

	"connectrpc.com/connect"

	adminv1 "tools.xdoubleu.com/gen/admin/v1"
	"tools.xdoubleu.com/internal/models"
)

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

func (h *adminConnectHandler) GetJobStats(
	ctx context.Context,
	req *connect.Request[adminv1.GetJobStatsRequest],
) (*connect.Response[adminv1.GetJobStatsResponse], error) {
	if err := h.requireAdmin(ctx); err != nil {
		return nil, err
	}

	since := windowSince(req.Msg.WindowDays)

	stats, err := h.app.jobRunsRepo.Stats(ctx, since)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	runs, err := h.app.jobRunsRepo.ListRecent(ctx, since, recentRunsLimit)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	protoStats := make([]*adminv1.JobStat, len(stats))
	for i, s := range stats {
		protoStats[i] = &adminv1.JobStat{
			JobId:         s.JobID,
			TotalRuns:     s.TotalRuns,
			FailedRuns:    s.FailedRuns,
			AvgDurationMs: s.AvgDurationMs,
			LastRunAt:     s.LastRunAt.Format(time.RFC3339),
		}
	}

	protoRuns := make([]*adminv1.JobRun, len(runs))
	for i, r := range runs {
		protoRuns[i] = &adminv1.JobRun{
			JobId:      r.JobID,
			StartedAt:  r.StartedAt.Format(time.RFC3339),
			DurationMs: r.DurationMs,
			Success:    r.Success,
			Error:      r.Error,
		}
	}

	return connect.NewResponse(&adminv1.GetJobStatsResponse{
		Stats:      protoStats,
		RecentRuns: protoRuns,
	}), nil
}

func (h *adminConnectHandler) GetUsageStats(
	ctx context.Context,
	req *connect.Request[adminv1.GetUsageStatsRequest],
) (*connect.Response[adminv1.GetUsageStatsResponse], error) {
	if err := h.requireAdmin(ctx); err != nil {
		return nil, err
	}

	entries, err := h.app.usageRepo.GetDaily(ctx, windowSince(req.Msg.WindowDays))
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	protoEntries := make([]*adminv1.UsageDay, len(entries))
	for i, e := range entries {
		protoEntries[i] = &adminv1.UsageDay{
			Day:      e.Day.Format(time.DateOnly),
			App:      e.App,
			Endpoint: e.Endpoint,
			Count:    e.Count,
		}
	}

	return connect.NewResponse(&adminv1.GetUsageStatsResponse{
		Entries: protoEntries,
	}), nil
}

func (h *adminConnectHandler) GetStorageStats(
	ctx context.Context,
	_ *connect.Request[adminv1.GetStorageStatsRequest],
) (*connect.Response[adminv1.GetStorageStatsResponse], error) {
	if err := h.requireAdmin(ctx); err != nil {
		return nil, err
	}

	latest, err := h.app.storageRepo.Latest(ctx)
	if err != nil {
		// No snapshot yet is not an error — the scan has not run.
		latest = nil
	}

	history, err := h.app.storageRepo.History(ctx, windowSince(defaultWindowDays))
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	protoHistory := make([]*adminv1.StorageSnapshot, len(history))
	for i, s := range history {
		snap := s
		protoHistory[i] = protoStorageSnapshot(&snap)
	}

	return connect.NewResponse(&adminv1.GetStorageStatsResponse{
		Latest:  protoStorageSnapshot(latest),
		History: protoHistory,
	}), nil
}

func protoStorageSnapshot(s *models.StorageSnapshot) *adminv1.StorageSnapshot {
	if s == nil {
		return nil
	}

	breakdown := make([]*adminv1.PrefixStat, len(s.PrefixBreakdown))
	for i, p := range s.PrefixBreakdown {
		breakdown[i] = &adminv1.PrefixStat{
			Prefix:    p.Prefix,
			SizeBytes: p.SizeBytes,
			Count:     p.Count,
		}
	}

	return &adminv1.StorageSnapshot{
		ScannedAt:            s.ScannedAt.Format(time.RFC3339),
		TotalSizeBytes:       s.TotalSizeBytes,
		ObjectCount:          s.ObjectCount,
		OrphanSizeBytes:      s.OrphanSizeBytes,
		OrphanCount:          s.OrphanCount,
		StaleUploadSizeBytes: s.StaleUploadSizeBytes,
		StaleUploadCount:     s.StaleUploadCount,
		PrefixBreakdown:      breakdown,
	}
}

func (h *adminConnectHandler) GetDatabaseStats(
	ctx context.Context,
	_ *connect.Request[adminv1.GetDatabaseStatsRequest],
) (*connect.Response[adminv1.GetDatabaseStatsResponse], error) {
	if err := h.requireAdmin(ctx); err != nil {
		return nil, err
	}

	total, err := h.app.dbStatsRepo.TotalSize(ctx)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	schemas, err := h.app.dbStatsRepo.SchemaSizes(ctx)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	protoSchemas := make([]*adminv1.SchemaStat, len(schemas))
	for i, s := range schemas {
		protoSchemas[i] = &adminv1.SchemaStat{
			Name:       s.Name,
			SizeBytes:  s.SizeBytes,
			TableCount: s.TableCount,
		}
	}

	return connect.NewResponse(&adminv1.GetDatabaseStatsResponse{
		TotalSizeBytes: total,
		Schemas:        protoSchemas,
	}), nil
}
