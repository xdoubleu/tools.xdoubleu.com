package main

import (
	"context"
	"errors"
	"log/slog"
	"sort"
	"time"

	"connectrpc.com/connect"

	observabilityv1 "tools.xdoubleu.com/gen/observability/v1"
	"tools.xdoubleu.com/internal/github"
	"tools.xdoubleu.com/internal/models"
	"tools.xdoubleu.com/internal/sentryapi"
)

// currentSlowTransactionsLimit caps how many transactions GetSlowTransactions'
// "current" (live) section returns — the slowest ones are what the dashboard
// card actually shows.
const currentSlowTransactionsLimit = 20

// These handlers surface the three external observability signals. Each GUARDS
// its source: an unset token yields configured=false and an upstream failure is
// logged and downgraded to an empty section, so one broken source never fails
// the whole response.

func (h *obsConnectHandler) GetFailingPullRequests(
	ctx context.Context,
	_ *connect.Request[observabilityv1.GetFailingPullRequestsRequest],
) (*connect.Response[observabilityv1.GetFailingPullRequestsResponse], error) {
	if err := requireAdmin(ctx); err != nil {
		return nil, err
	}
	return connect.NewResponse(h.failingPullRequests(ctx)), nil
}

func (h *obsConnectHandler) failingPullRequests(
	ctx context.Context,
) *observabilityv1.GetFailingPullRequestsResponse {
	resp := &observabilityv1.GetFailingPullRequestsResponse{
		PullRequests: []*observabilityv1.FailingPullRequest{},
		Configured:   true,
		FailingCount: 0,
	}

	prs, err := h.app.githubClient.ListFailingPullRequests(ctx)
	if err != nil {
		if errors.Is(err, github.ErrNotConfigured) {
			resp.Configured = false
		} else {
			h.app.logger.WarnContext(ctx, "failing pull requests unavailable",
				slog.Any("error", err))
		}
		return resp
	}

	protoPRs := make([]*observabilityv1.FailingPullRequest, len(prs))
	for i, pr := range prs {
		checks := make([]*observabilityv1.FailingCheck, len(pr.FailingChecks))
		for j, chk := range pr.FailingChecks {
			checks[j] = &observabilityv1.FailingCheck{
				Name:       chk.Name,
				Conclusion: chk.Conclusion,
				Url:        chk.URL,
			}
		}
		protoPRs[i] = &observabilityv1.FailingPullRequest{
			Number:        pr.Number,
			Title:         pr.Title,
			Url:           pr.URL,
			Author:        pr.Author,
			UpdatedAt:     pr.UpdatedAt.Format(time.RFC3339),
			FailingChecks: checks,
		}
	}
	resp.PullRequests = protoPRs
	resp.FailingCount = int32(len(prs)) //nolint:gosec // count fits int32
	return resp
}

func (h *obsConnectHandler) GetWorkflowRuns(
	ctx context.Context,
	_ *connect.Request[observabilityv1.GetWorkflowRunsRequest],
) (*connect.Response[observabilityv1.GetWorkflowRunsResponse], error) {
	if err := requireAdmin(ctx); err != nil {
		return nil, err
	}
	return connect.NewResponse(h.workflowRuns(ctx)), nil
}

func (h *obsConnectHandler) workflowRuns(
	ctx context.Context,
) *observabilityv1.GetWorkflowRunsResponse {
	resp := &observabilityv1.GetWorkflowRunsResponse{
		Runs:       []*observabilityv1.WorkflowRun{},
		Configured: true,
	}

	runs, err := h.app.githubClient.ListWorkflowRuns(ctx)
	if err != nil {
		if errors.Is(err, github.ErrNotConfigured) {
			resp.Configured = false
		} else {
			h.app.logger.WarnContext(ctx, "workflow runs unavailable",
				slog.Any("error", err))
		}
		return resp
	}

	protoRuns := make([]*observabilityv1.WorkflowRun, len(runs))
	for i, run := range runs {
		protoRuns[i] = &observabilityv1.WorkflowRun{
			Id:         run.ID,
			Name:       run.Name,
			Event:      run.Event,
			Branch:     run.Branch,
			Status:     run.Status,
			Conclusion: run.Conclusion,
			Url:        run.URL,
			StartedAt:  run.StartedAt.Format(time.RFC3339),
			DurationMs: run.DurationMs,
			FailedJobs: h.failedJobNames(ctx, run),
		}
	}
	resp.Runs = protoRuns
	return resp
}

// failedJobNames names the jobs that failed within run, fetched from GitHub
// only for a failed push-to-main run (see WorkflowRun.failed_jobs' doc
// comment) — every other run returns nil without an extra API call. A fetch
// failure here is logged and swallowed rather than degrading the whole
// response: the run itself still shows as failed, just without job detail.
func (h *obsConnectHandler) failedJobNames(
	ctx context.Context, run github.WorkflowRun,
) []string {
	if run.Event != "push" || run.Branch != "main" || run.Conclusion != "failure" {
		return nil
	}

	jobs, err := h.app.githubClient.ListWorkflowRunJobs(ctx, run.ID)
	if err != nil {
		h.app.logger.WarnContext(ctx, "workflow run jobs unavailable",
			slog.Any("error", err), slog.Int64("run_id", run.ID))
		return nil
	}

	names := make([]string, 0, len(jobs))
	for _, job := range jobs {
		if job.Conclusion == "failure" {
			names = append(names, job.Name)
		}
	}
	return names
}

func (h *obsConnectHandler) GetWorkflowRunStats(
	ctx context.Context,
	req *connect.Request[observabilityv1.GetWorkflowRunStatsRequest],
) (*connect.Response[observabilityv1.GetWorkflowRunStatsResponse], error) {
	if err := requireAdmin(ctx); err != nil {
		return nil, err
	}
	resp, err := h.workflowRunStats(ctx, req.Msg.GetWindowDays())
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	return connect.NewResponse(resp), nil
}

// workflowRunStats reports aggregated CI duration/failure history rather
// than a raw run list (issue #1217): main_failures should always be empty —
// deploys run straight off a passing push to main — and the two duration
// breakdowns answer "how long does CI normally take" and "which job is
// slow" without dumping every recorded run.
func (h *obsConnectHandler) workflowRunStats(
	ctx context.Context, windowDays int32,
) (*observabilityv1.GetWorkflowRunStatsResponse, error) {
	if windowDays <= 0 {
		windowDays = defaultWindowDays
	}
	since := time.Now().Add(-time.Duration(windowDays) * 24 * time.Hour)

	failures, err := h.app.workflowRunsRepo.MainFailures(ctx, "main", "failure", since)
	if err != nil {
		return nil, err
	}
	protoFailures := make([]*observabilityv1.MainBranchFailure, len(failures))
	for i, f := range failures {
		protoFailures[i] = &observabilityv1.MainBranchFailure{
			RunId:        f.RunID,
			WorkflowName: f.WorkflowName,
			Url:          f.URL,
			CompletedAt:  f.CompletedAt.Format(time.RFC3339),
		}
	}

	workflowStats, err := h.app.workflowRunsRepo.WorkflowDurationStats(ctx, since)
	if err != nil {
		return nil, err
	}
	protoWorkflowStats := make(
		[]*observabilityv1.WorkflowDurationStat, len(workflowStats),
	)
	for i, s := range workflowStats {
		protoWorkflowStats[i] = &observabilityv1.WorkflowDurationStat{
			WorkflowName:  s.WorkflowName,
			AvgDurationMs: s.AvgDurationMs,
			P95DurationMs: s.P95DurationMs,
			RunCount:      s.RunCount,
		}
	}

	jobStats, err := h.app.workflowRunsRepo.JobDurationStats(ctx, since)
	if err != nil {
		return nil, err
	}
	protoJobStats := make([]*observabilityv1.JobDurationStat, len(jobStats))
	for i, s := range jobStats {
		protoJobStats[i] = &observabilityv1.JobDurationStat{
			JobName:       s.JobName,
			AvgDurationMs: s.AvgDurationMs,
			P95DurationMs: s.P95DurationMs,
			RunCount:      s.RunCount,
		}
	}

	return &observabilityv1.GetWorkflowRunStatsResponse{
		MainFailures:          protoFailures,
		WorkflowDurationStats: protoWorkflowStats,
		JobDurationStats:      protoJobStats,
	}, nil
}

func (h *obsConnectHandler) GetSecurityAlerts(
	ctx context.Context,
	_ *connect.Request[observabilityv1.GetSecurityAlertsRequest],
) (*connect.Response[observabilityv1.GetSecurityAlertsResponse], error) {
	if err := requireAdmin(ctx); err != nil {
		return nil, err
	}
	return connect.NewResponse(h.securityAlerts(ctx)), nil
}

func (h *obsConnectHandler) securityAlerts(
	ctx context.Context,
) *observabilityv1.GetSecurityAlertsResponse {
	resp := &observabilityv1.GetSecurityAlertsResponse{
		Alerts:     []*observabilityv1.SecurityAlert{},
		Configured: true,
		AlertCount: 0,
	}

	alerts, err := h.app.githubClient.ListSecurityAlerts(ctx)
	if err != nil {
		if errors.Is(err, github.ErrNotConfigured) {
			resp.Configured = false
		} else {
			h.app.logger.WarnContext(ctx, "security alerts unavailable",
				slog.Any("error", err))
		}
		return resp
	}

	protoAlerts := make([]*observabilityv1.SecurityAlert, len(alerts))
	for i, a := range alerts {
		protoAlerts[i] = &observabilityv1.SecurityAlert{
			Number:      a.Number,
			PackageName: a.PackageName,
			Ecosystem:   a.Ecosystem,
			Severity:    a.Severity,
			Summary:     a.Summary,
			Url:         a.URL,
			CreatedAt:   a.CreatedAt.Format(time.RFC3339),
			AlertType:   securityAlertTypeToProto(a.Type),
			RuleId:      a.RuleID,
			FilePath:    a.FilePath,
			Line:        int32(a.Line), //nolint:gosec // line numbers fit int32
			SecretType:  a.SecretTypeDisplayName,
		}
	}
	resp.Alerts = protoAlerts
	resp.AlertCount = int32(len(alerts)) //nolint:gosec // count fits int32
	return resp
}

// securityAlertTypeToProto maps a github.SecurityAlertType to its proto enum
// value, defaulting to unspecified for an unrecognized/zero-value type.
func securityAlertTypeToProto(
	t github.SecurityAlertType,
) observabilityv1.SecurityAlertType {
	switch t {
	case github.SecurityAlertTypeDependabot:
		return observabilityv1.SecurityAlertType_SECURITY_ALERT_TYPE_DEPENDABOT
	case github.SecurityAlertTypeCodeScanning:
		return observabilityv1.SecurityAlertType_SECURITY_ALERT_TYPE_CODE_SCANNING
	case github.SecurityAlertTypeSecretScanning:
		return observabilityv1.SecurityAlertType_SECURITY_ALERT_TYPE_SECRET_SCANNING
	default:
		return observabilityv1.SecurityAlertType_SECURITY_ALERT_TYPE_UNSPECIFIED
	}
}

func (h *obsConnectHandler) DismissSecurityAlert(
	ctx context.Context,
	req *connect.Request[observabilityv1.DismissSecurityAlertRequest],
) (*connect.Response[observabilityv1.DismissSecurityAlertResponse], error) {
	if err := requireAdmin(ctx); err != nil {
		return nil, err
	}
	resp, err := h.dismissSecurityAlert(
		ctx,
		securityAlertTypeFromProto(req.Msg.GetAlertType()),
		req.Msg.GetAlertNumber(),
		req.Msg.GetReason(),
	)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(resp), nil
}

// dismissSecurityAlert is the second deliberate mutation in this otherwise
// read-only observability surface, alongside resolveSentryIssue below —
// see api/CLAUDE.md's "Apps MCP Server" section. Takes the internal
// github.SecurityAlertType (rather than the proto enum) so the MCP tool,
// whose alert_type input already matches those string values, can call it
// directly without a round trip through the proto enum.
func (h *obsConnectHandler) dismissSecurityAlert(
	ctx context.Context,
	alertType github.SecurityAlertType,
	alertNumber int64,
	reason string,
) (*observabilityv1.DismissSecurityAlertResponse, error) {
	err := h.app.githubClient.DismissSecurityAlert(ctx, alertType, alertNumber, reason)
	if err != nil {
		if errors.Is(err, github.ErrInvalidDismissReason) {
			return nil, connect.NewError(connect.CodeInvalidArgument, err)
		}
		if errors.Is(err, github.ErrNotConfigured) {
			return nil, connect.NewError(connect.CodeFailedPrecondition, err)
		}
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	return &observabilityv1.DismissSecurityAlertResponse{}, nil
}

// securityAlertTypeFromProto is the inverse of securityAlertTypeToProto.
func securityAlertTypeFromProto(
	t observabilityv1.SecurityAlertType,
) github.SecurityAlertType {
	switch t {
	case observabilityv1.SecurityAlertType_SECURITY_ALERT_TYPE_DEPENDABOT:
		return github.SecurityAlertTypeDependabot
	case observabilityv1.SecurityAlertType_SECURITY_ALERT_TYPE_CODE_SCANNING:
		return github.SecurityAlertTypeCodeScanning
	case observabilityv1.SecurityAlertType_SECURITY_ALERT_TYPE_SECRET_SCANNING:
		return github.SecurityAlertTypeSecretScanning
	case observabilityv1.SecurityAlertType_SECURITY_ALERT_TYPE_UNSPECIFIED:
		return ""
	default:
		return ""
	}
}

func (h *obsConnectHandler) GetSentryIssues(
	ctx context.Context,
	_ *connect.Request[observabilityv1.GetSentryIssuesRequest],
) (*connect.Response[observabilityv1.GetSentryIssuesResponse], error) {
	if err := requireAdmin(ctx); err != nil {
		return nil, err
	}
	return connect.NewResponse(h.sentryIssues(ctx)), nil
}

func (h *obsConnectHandler) sentryIssues(
	ctx context.Context,
) *observabilityv1.GetSentryIssuesResponse {
	resp := &observabilityv1.GetSentryIssuesResponse{
		Issues:          []*observabilityv1.SentryIssue{},
		Configured:      true,
		UnresolvedCount: 0,
	}

	issues, err := h.app.sentryClient.ListUnresolvedIssues(ctx)
	if err != nil {
		if errors.Is(err, sentryapi.ErrNotConfigured) {
			resp.Configured = false
		} else {
			h.app.logger.WarnContext(ctx, "sentry issues unavailable",
				slog.Any("error", err))
		}
		return resp
	}

	protoIssues := make([]*observabilityv1.SentryIssue, len(issues))
	for i, is := range issues {
		protoIssues[i] = &observabilityv1.SentryIssue{
			Id:        is.ID,
			Title:     is.Title,
			Culprit:   is.Culprit,
			Permalink: is.Permalink,
			Count:     is.Count,
			LastSeen:  is.LastSeen.Format(time.RFC3339),
			Level:     is.Level,
			Project:   is.Project,
		}
	}
	resp.Issues = protoIssues
	resp.UnresolvedCount = int32(len(issues)) //nolint:gosec // count fits int32
	return resp
}

func (h *obsConnectHandler) ResolveSentryIssue(
	ctx context.Context,
	req *connect.Request[observabilityv1.ResolveSentryIssueRequest],
) (*connect.Response[observabilityv1.ResolveSentryIssueResponse], error) {
	if err := requireAdmin(ctx); err != nil {
		return nil, err
	}
	resp, err := h.resolveSentryIssue(ctx, req.Msg.GetIssueId())
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(resp), nil
}

func (h *obsConnectHandler) resolveSentryIssue(
	ctx context.Context, issueID string,
) (*observabilityv1.ResolveSentryIssueResponse, error) {
	if err := h.app.sentryClient.ResolveIssue(ctx, issueID); err != nil {
		if errors.Is(err, sentryapi.ErrReauthRequired) {
			// Distinct from ErrNotConfigured/CodeFailedPrecondition below: the
			// web UI uses this code to prompt an explicit reconnect instead of
			// silently treating the connection as unconfigured (issue #791).
			return nil, connect.NewError(connect.CodeUnauthenticated, err)
		}
		if errors.Is(err, sentryapi.ErrNotConfigured) {
			return nil, connect.NewError(connect.CodeFailedPrecondition, err)
		}
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	return &observabilityv1.ResolveSentryIssueResponse{}, nil
}

func (h *obsConnectHandler) GetSlowTransactions(
	ctx context.Context,
	_ *connect.Request[observabilityv1.GetSlowTransactionsRequest],
) (*connect.Response[observabilityv1.GetSlowTransactionsResponse], error) {
	if err := requireAdmin(ctx); err != nil {
		return nil, err
	}
	resp, err := h.slowTransactions(ctx)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	return connect.NewResponse(resp), nil
}

// slowTransactions combines two independent sections: current is live from
// Sentry (guarded the same way sentryIssues/failingPullRequests are — an
// unset token yields configured=false, an upstream failure is logged and
// downgraded to an empty section), and trending is computed from stored
// history (global.transaction_latency_daily), fetched regardless of whether
// Sentry is currently reachable so past regressions stay visible.
func (h *obsConnectHandler) slowTransactions(
	ctx context.Context,
) (*observabilityv1.GetSlowTransactionsResponse, error) {
	resp := &observabilityv1.GetSlowTransactionsResponse{
		Current:    []*observabilityv1.SlowTransaction{},
		Configured: true,
		Trending:   []*observabilityv1.TransactionTrend{},
	}

	stats, err := h.app.sentryClient.ListTransactionStats(ctx)
	if err != nil {
		if errors.Is(err, sentryapi.ErrNotConfigured) {
			resp.Configured = false
		} else {
			return nil, err
		}
	} else {
		resp.Current = protoSlowTransactions(stats)
	}

	trends, err := h.app.transactionLatencyRepo.Trends(ctx)
	if err != nil {
		return nil, err
	}

	protoTrends := make([]*observabilityv1.TransactionTrend, len(trends))
	for i, t := range trends {
		protoTrends[i] = &observabilityv1.TransactionTrend{
			Transaction:    t.Transaction,
			Project:        t.Project,
			PriorAvgP95Ms:  t.PriorAvgP95Ms,
			RecentAvgP95Ms: t.RecentAvgP95Ms,
			PctChange:      t.PctChange,
		}
	}
	resp.Trending = protoTrends

	return resp, nil
}

func (h *obsConnectHandler) GetTransactionLatencyHistory(
	ctx context.Context,
	req *connect.Request[observabilityv1.GetTransactionLatencyHistoryRequest],
) (*connect.Response[observabilityv1.GetTransactionLatencyHistoryResponse], error) {
	if err := requireAdmin(ctx); err != nil {
		return nil, err
	}
	resp, err := h.transactionLatencyHistory(ctx, req.Msg.GetWindowDays())
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	return connect.NewResponse(resp), nil
}

// transactionLatencyHistory returns every stored (project, transaction)
// series over the window flat and unfiltered — the client pivots and picks
// which series to plot.
func (h *obsConnectHandler) transactionLatencyHistory(
	ctx context.Context,
	windowDays int32,
) (*observabilityv1.GetTransactionLatencyHistoryResponse, error) {
	points, err := h.app.transactionLatencyRepo.History(ctx, windowSince(windowDays))
	if err != nil {
		return nil, err
	}

	protoPoints := make([]*observabilityv1.TransactionLatencyPoint, len(points))
	for i, p := range points {
		protoPoints[i] = &observabilityv1.TransactionLatencyPoint{
			Day:           p.Day.Format("2006-01-02"),
			Project:       p.Project,
			Transaction:   p.Transaction,
			P95DurationMs: p.P95DurationMs,
			RequestCount:  p.RequestCount,
		}
	}
	return &observabilityv1.GetTransactionLatencyHistoryResponse{
		Points: protoPoints,
	}, nil
}

// protoSlowTransactions sorts stats slowest-first and caps it to
// currentSlowTransactionsLimit — ListTransactionStats returns a broad
// sample (not necessarily pre-sorted after project filtering), this is the
// "current" view's own ordering.
func protoSlowTransactions(
	stats []sentryapi.TransactionStat,
) []*observabilityv1.SlowTransaction {
	sorted := make([]sentryapi.TransactionStat, len(stats))
	copy(sorted, stats)
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].P95DurationMs > sorted[j].P95DurationMs
	})
	if len(sorted) > currentSlowTransactionsLimit {
		sorted = sorted[:currentSlowTransactionsLimit]
	}

	protoStats := make([]*observabilityv1.SlowTransaction, len(sorted))
	for i, s := range sorted {
		protoStats[i] = &observabilityv1.SlowTransaction{
			Transaction:   s.Transaction,
			Project:       s.Project,
			P95DurationMs: s.P95DurationMs,
			RequestCount:  s.RequestCount,
		}
	}
	return protoStats
}

// hostMetricsHistoryPoint is the shared shape behind cpu/memory/disk_history
// — factored out so protoHostMetricsResponse doesn't repeat itself building
// three near-identical slices.
type hostMetricPointSource func(models.HostMetricSample) float64

func (h *obsConnectHandler) GetHostMetrics(
	ctx context.Context,
	req *connect.Request[observabilityv1.GetHostMetricsRequest],
) (*connect.Response[observabilityv1.GetHostMetricsResponse], error) {
	if err := requireAdmin(ctx); err != nil {
		return nil, err
	}

	resp, err := h.hostMetrics(ctx, req.Msg.GetSince())
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	return connect.NewResponse(resp), nil
}

func (h *obsConnectHandler) hostMetrics(
	ctx context.Context, since string,
) (*observabilityv1.GetHostMetricsResponse, error) {
	sinceTime := time.Now().Add(-defaultWindowDays * 24 * time.Hour)
	if since != "" {
		if parsed, err := time.Parse(time.RFC3339, since); err == nil {
			sinceTime = parsed
		}
	}

	samples, err := h.app.hostMetricsRepo.Since(ctx, sinceTime)
	if err != nil {
		return nil, err
	}

	resp := &observabilityv1.GetHostMetricsResponse{
		CpuHistory: hostMetricHistory(samples, func(s models.HostMetricSample) float64 {
			return s.CPUPercent
		}),
		MemoryHistory: hostMetricHistory(
			samples,
			func(s models.HostMetricSample) float64 {
				return s.MemoryPercent
			},
		),
		DiskHistory: hostMetricHistory(
			samples,
			func(s models.HostMetricSample) float64 {
				return s.DiskPercent
			},
		),
	}
	if len(samples) > 0 {
		latest := samples[len(samples)-1]
		resp.CpuPercent = latest.CPUPercent
		resp.MemoryPercent = latest.MemoryPercent
		resp.DiskPercent = latest.DiskPercent
	}
	return resp, nil
}

func hostMetricHistory(
	samples []models.HostMetricSample, value hostMetricPointSource,
) []*observabilityv1.HostMetricPoint {
	points := make([]*observabilityv1.HostMetricPoint, len(samples))
	for i, s := range samples {
		points[i] = &observabilityv1.HostMetricPoint{
			Timestamp: s.SampledAt.Format(time.RFC3339),
			Value:     value(s),
		}
	}
	return points
}

func (h *obsConnectHandler) GetLogs(
	ctx context.Context,
	req *connect.Request[observabilityv1.GetLogsRequest],
) (*connect.Response[observabilityv1.GetLogsResponse], error) {
	if err := requireAdmin(ctx); err != nil {
		return nil, err
	}

	resp, err := h.logs(
		ctx, req.Msg.GetSource(), req.Msg.GetMinLevel(), req.Msg.GetSince(),
	)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	return connect.NewResponse(resp), nil
}

func (h *obsConnectHandler) logs(
	ctx context.Context, source, minLevel, since string,
) (*observabilityv1.GetLogsResponse, error) {
	sinceTime := time.Now().Add(-defaultWindowDays * 24 * time.Hour)
	if since != "" {
		if parsed, err := time.Parse(time.RFC3339, since); err == nil {
			sinceTime = parsed
		}
	}

	entries, err := h.app.logsRepo.Query(ctx, sinceTime, source, minLevel)
	if err != nil {
		return nil, err
	}

	protoEntries := make([]*observabilityv1.LogEntry, len(entries))
	for i, e := range entries {
		protoEntries[i] = &observabilityv1.LogEntry{
			OccurredAt: e.OccurredAt.Format(time.RFC3339),
			Source:     e.Source,
			Level:      e.Level,
			Message:    e.Message,
			AttrsJson:  string(e.AttrsJSON),
		}
	}
	return &observabilityv1.GetLogsResponse{Entries: protoEntries}, nil
}

func (h *obsConnectHandler) GetHealthOverview(
	ctx context.Context,
	_ *connect.Request[observabilityv1.GetHealthOverviewRequest],
) (*connect.Response[observabilityv1.GetHealthOverviewResponse], error) {
	if err := requireAdmin(ctx); err != nil {
		return nil, err
	}

	return connect.NewResponse(&observabilityv1.GetHealthOverviewResponse{
		Sentry: h.sentryIssues(ctx),
	}), nil
}
