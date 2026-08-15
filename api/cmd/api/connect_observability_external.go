package main

import (
	"context"
	"errors"
	"log/slog"
	"sort"
	"time"

	"connectrpc.com/connect"

	observabilityv1 "tools.xdoubleu.com/gen/observability/v1"
	"tools.xdoubleu.com/internal/digitalocean"
	"tools.xdoubleu.com/internal/github"
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
			h.app.logger.WarnContext(ctx, "slow transactions unavailable",
				slog.Any("error", err))
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

func (h *obsConnectHandler) GetDeployStatus(
	ctx context.Context,
	_ *connect.Request[observabilityv1.GetDeployStatusRequest],
) (*connect.Response[observabilityv1.GetDeployStatusResponse], error) {
	if err := requireAdmin(ctx); err != nil {
		return nil, err
	}
	return connect.NewResponse(h.deployStatus(ctx)), nil
}

func (h *obsConnectHandler) deployStatus(
	ctx context.Context,
) *observabilityv1.GetDeployStatusResponse {
	resp := &observabilityv1.GetDeployStatusResponse{
		Configured:   true,
		Phase:        "",
		Cause:        "",
		CreatedAt:    "",
		UpdatedAt:    "",
		DeploymentId: "",
	}

	deployment, err := h.app.doClient.LatestDeployment(ctx)
	if err != nil {
		if errors.Is(err, digitalocean.ErrNotConfigured) {
			resp.Configured = false
		} else {
			h.app.logger.WarnContext(ctx, "deploy status unavailable",
				slog.Any("error", err))
		}
		return resp
	}

	if deployment == nil {
		return resp // configured, but no deployment yet
	}

	resp.Phase = deployment.Phase
	resp.Cause = deployment.Cause
	resp.CreatedAt = deployment.CreatedAt.Format(time.RFC3339)
	resp.UpdatedAt = deployment.UpdatedAt.Format(time.RFC3339)
	resp.DeploymentId = deployment.ID
	return resp
}

// deployLogsMsg is a short alias for the long name buf's standard naming
// lint forces on GetDeployLogs' streamed message (observability.proto),
// purely to keep parameter lines under the linter's line length.
type deployLogsMsg = observabilityv1.ObservabilityServiceGetDeployLogsResponse

func (h *obsConnectHandler) GetDeployLogs(
	ctx context.Context,
	req *connect.Request[observabilityv1.GetDeployLogsRequest],
	stream *connect.ServerStream[deployLogsMsg],
) error {
	if err := requireAdmin(ctx); err != nil {
		return err
	}
	return h.deployLogsStream(
		ctx, req.Msg.GetDeploymentId(), int(req.Msg.GetTailLines()), stream,
	)
}

// deployLogsStream is GetDeployLogs' handler body: it resolves deploymentID
// to the latest deployment when empty, then streams each of its
// BUILD/DEPLOY/RUN/RUN_RESTARTED component logs to the client as soon as it
// resolves, rather than assembling one response after every component
// finishes (issue #672, second pass — see routes.go's deployLogsCtxTimeout
// comment for why: DigitalOcean App Platform's edge resets the connection
// ~25s after the request starts if no byte has been written yet, a ceiling
// no server-side deadline above it can beat). Streaming means the first
// component's log reaches the client well under that, regardless of how long
// slower components take. Guards its source the same way deployStatus does:
// an unset token ends the stream with nothing sent, and an upstream failure
// is logged and the stream ends with whatever was already sent — never a
// hard RPC error, so one broken source never breaks the dialog.
func (h *obsConnectHandler) deployLogsStream(
	ctx context.Context,
	deploymentID string,
	tailLines int,
	stream *connect.ServerStream[deployLogsMsg],
) error {
	if deploymentID == "" {
		latest, err := h.app.doClient.LatestDeployment(ctx)
		if err != nil {
			h.logDeployLogsErr(ctx, err)
			return nil
		}
		if latest == nil {
			return nil // configured, but no deployment yet
		}
		deploymentID = latest.ID
	}

	err := h.app.doClient.DeploymentLogsStream(ctx, deploymentID, tailLines,
		func(log digitalocean.ComponentLog) error {
			return stream.Send(&deployLogsMsg{
				Log: &observabilityv1.DeployComponentLog{
					Component:    log.Component,
					LogType:      string(log.Type),
					DeploymentId: log.DeploymentID,
					Content:      log.Content,
					Truncated:    log.Truncated,
				},
			})
		},
	)
	if err != nil {
		h.logDeployLogsErr(ctx, err)
	}
	return nil
}

// logDeployLogsErr logs a genuine upstream failure, mirroring
// degradeDeployLogs's convention. An unset token is a normal degraded state,
// not something worth logging.
func (h *obsConnectHandler) logDeployLogsErr(ctx context.Context, err error) {
	if errors.Is(err, digitalocean.ErrNotConfigured) {
		return
	}
	h.app.logger.ErrorContext(ctx, "deploy logs unavailable", slog.Any("error", err))
}

// deployLogs resolves deploymentID to the latest deployment when empty, then
// fetches its BUILD/DEPLOY/RUN/RUN_RESTARTED component logs — plus the
// runtime logs of the app's active deployment when that is a different one.
// tailLines bounds the live backlog replayed per component; 0 takes the
// client's default. Guards its source the same way deployStatus does: an
// unset token yields configured=false and an upstream failure is logged and
// downgraded to an empty section.
func (h *obsConnectHandler) deployLogs(
	ctx context.Context, deploymentID string, tailLines int,
) *observabilityv1.GetDeployLogsResponse {
	resp := &observabilityv1.GetDeployLogsResponse{
		Configured:   true,
		DeploymentId: deploymentID,
		Logs:         []*observabilityv1.DeployComponentLog{},
	}

	if deploymentID == "" {
		resolved, ok := h.resolveLatestDeploymentID(ctx, resp)
		if !ok {
			return resp
		}
		deploymentID = resolved
		resp.DeploymentId = deploymentID
	}

	logs, err := h.app.doClient.DeploymentLogs(ctx, deploymentID, tailLines)
	if err != nil {
		h.degradeDeployLogs(ctx, resp, err)
		return resp
	}

	protoLogs := make([]*observabilityv1.DeployComponentLog, len(logs))
	for i, l := range logs {
		protoLogs[i] = &observabilityv1.DeployComponentLog{
			Component:    l.Component,
			LogType:      string(l.Type),
			DeploymentId: l.DeploymentID,
			Content:      l.Content,
			Truncated:    l.Truncated,
		}
	}
	resp.Logs = protoLogs
	return resp
}

// resolveLatestDeploymentID looks up the latest deployment's ID for a
// deploymentID-less GetDeployLogs call. ok is false when resp has already
// been finalized — either degraded (upstream error) or left at its
// zero-logs default (configured, but no deployment yet).
func (h *obsConnectHandler) resolveLatestDeploymentID(
	ctx context.Context, resp *observabilityv1.GetDeployLogsResponse,
) (string, bool) {
	latest, err := h.app.doClient.LatestDeployment(ctx)
	if err != nil {
		h.degradeDeployLogs(ctx, resp, err)
		return "", false
	}
	if latest == nil {
		return "", false // configured, but no deployment yet
	}
	return latest.ID, true
}

// degradeDeployLogs downgrades resp on an upstream digitalocean error,
// mirroring deployStatus's degrade-independently convention.
func (h *obsConnectHandler) degradeDeployLogs(
	ctx context.Context, resp *observabilityv1.GetDeployLogsResponse, err error,
) {
	if errors.Is(err, digitalocean.ErrNotConfigured) {
		resp.Configured = false
		return
	}
	h.logDeployLogsErr(ctx, err)
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
		Deploy: h.deployStatus(ctx),
	}), nil
}
