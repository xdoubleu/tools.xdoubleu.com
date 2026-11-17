package obs

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sort"
	"strings"
	"time"

	"connectrpc.com/connect/v2"

	"tools.xdoubleu.com/gen/factory/v1/factoryv1connect"
	observabilityv1 "tools.xdoubleu.com/gen/observability/v1"
	"tools.xdoubleu.com/internal/platform/github"
	"tools.xdoubleu.com/internal/platform/reposignals"
	platformrepositories "tools.xdoubleu.com/internal/platform/repositories"
	"tools.xdoubleu.com/internal/platform/sentryapi"
)

// currentSlowTransactionsLimit caps GetSlowTransactions' live section.
const currentSlowTransactionsLimit = 20

// Each external-signal handler guards its source: an unset token yields
// configured=false; an upstream failure logs and returns an empty section,
// which GetSecurityAlerts flags with error.

func (h *obsConnectHandler) GetFailingPullRequests(
	ctx context.Context,
	req *observabilityv1.GetFailingPullRequestsRequest,
) (*observabilityv1.GetFailingPullRequestsResponse, error) {
	if err := RequireAdmin(ctx); err != nil {
		return nil, err
	}
	resp, err := h.failingPullRequests(ctx, req.GetRepo())
	if err != nil {
		h.deps.Logger.WarnContext(ctx, "failing pull requests unavailable",
			slog.Any("error", err))
	}
	return resp, nil
}

// monitoredRepos lists the registered repos, none when unwired or unreadable.
func (h *obsConnectHandler) monitoredRepos(ctx context.Context) []reposignals.Repo {
	if h.deps.Repos == nil {
		return nil
	}
	repos, err := h.deps.Repos.MonitoredRepos(ctx)
	if err != nil {
		h.deps.Logger.WarnContext(ctx, "monitored repos unavailable",
			slog.Any("error", err))
		return nil
	}
	return repos
}

// failingPullRequests returns an upstream failure alongside the empty
// section, so the MCP tool can report it instead of an empty list. repo
// ("owner/name") limits it to one registered repo.
func (h *obsConnectHandler) failingPullRequests(
	ctx context.Context, repo string,
) (*observabilityv1.GetFailingPullRequestsResponse, error) {
	resp := &observabilityv1.GetFailingPullRequestsResponse{
		PullRequests: []*observabilityv1.FailingPullRequest{},
		Configured:   true,
		FailingCount: 0,
	}

	repos := h.monitoredRepos(ctx)
	if len(repos) == 0 {
		return h.failingPullRequestsOfPickedRepo(ctx, repo, resp)
	}
	if repo != "" {
		r, ok := reposignals.Find(repos, repo)
		if !ok {
			return resp, fmt.Errorf("unknown repo %q", repo)
		}
		repos = []reposignals.Repo{r}
	}

	var errs []error
	for _, r := range repos {
		prs, err := h.deps.GithubClient.ListFailingPullRequestsIn(
			ctx, r.Slug, github.PRScope{
				Label:        r.RedPRLabel,
				BranchPrefix: r.RedPRBranchPrefix,
			})
		if errors.Is(err, github.ErrNotConfigured) {
			resp.Configured = false
			return resp, nil
		}
		if err != nil {
			errs = append(errs, fmt.Errorf("%s: %w", r.Slug, err))
			continue
		}
		resp.PullRequests = append(resp.PullRequests, protoFailingPRs(prs, r.Slug)...)
	}
	resp.FailingCount = int32(len(resp.PullRequests)) //nolint:gosec // fits int32
	return resp, errors.Join(errs...)
}

// failingPullRequestsOfPickedRepo is the fallback while no repo is registered:
// the admin-picked GitHub repo under the default scope.
func (h *obsConnectHandler) failingPullRequestsOfPickedRepo(
	ctx context.Context,
	repo string,
	resp *observabilityv1.GetFailingPullRequestsResponse,
) (*observabilityv1.GetFailingPullRequestsResponse, error) {
	if repo != "" {
		return resp, fmt.Errorf("unknown repo %q", repo)
	}
	prs, err := h.deps.GithubClient.ListFailingPullRequests(ctx)
	if errors.Is(err, github.ErrNotConfigured) {
		resp.Configured = false
		return resp, nil
	}
	if err != nil {
		return resp, err
	}
	resp.PullRequests = protoFailingPRs(prs, "")
	resp.FailingCount = int32(len(prs)) //nolint:gosec // count fits int32
	return resp, nil
}

func protoFailingPRs(
	prs []github.PullRequest, repo string,
) []*observabilityv1.FailingPullRequest {
	out := make([]*observabilityv1.FailingPullRequest, len(prs))
	for i, pr := range prs {
		checks := make([]*observabilityv1.FailingCheck, len(pr.FailingChecks))
		for j, chk := range pr.FailingChecks {
			checks[j] = &observabilityv1.FailingCheck{
				Name:       chk.Name,
				Conclusion: chk.Conclusion,
				Url:        chk.URL,
			}
		}
		out[i] = &observabilityv1.FailingPullRequest{
			Number:        pr.Number,
			Title:         pr.Title,
			Url:           pr.URL,
			Author:        pr.Author,
			UpdatedAt:     pr.UpdatedAt.Format(time.RFC3339),
			FailingChecks: checks,
			Repo:          repo,
		}
	}
	return out
}

func (h *obsConnectHandler) GetWorkflowRuns(
	ctx context.Context,
	_ *observabilityv1.GetWorkflowRunsRequest,
) (*observabilityv1.GetWorkflowRunsResponse, error) {
	if err := RequireAdmin(ctx); err != nil {
		return nil, err
	}
	return h.workflowRuns(ctx), nil
}

func (h *obsConnectHandler) workflowRuns(
	ctx context.Context,
) *observabilityv1.GetWorkflowRunsResponse {
	resp := &observabilityv1.GetWorkflowRunsResponse{
		Runs:       []*observabilityv1.WorkflowRun{},
		Configured: true,
	}

	runs, err := h.deps.GithubClient.ListWorkflowRuns(ctx)
	if err != nil {
		if errors.Is(err, github.ErrNotConfigured) {
			resp.Configured = false
		} else {
			h.deps.Logger.WarnContext(ctx, "workflow runs unavailable",
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

// failedJobNames fetches failed job names for a failed PR run or a failed
// push-to-main run. Fetch errors are logged and swallowed.
func (h *obsConnectHandler) failedJobNames(
	ctx context.Context, run github.WorkflowRun,
) []string {
	if run.Conclusion != conclusionFailure {
		return nil
	}
	if run.Event != "pull_request" && (run.Event != "push" || run.Branch != "main") {
		return nil
	}

	jobs, err := h.deps.GithubClient.ListWorkflowRunJobs(ctx, run.ID)
	if err != nil {
		h.deps.Logger.WarnContext(ctx, "workflow run jobs unavailable",
			slog.Any("error", err), slog.Int64("run_id", run.ID))
		return nil
	}

	names := make([]string, 0, len(jobs))
	for _, job := range jobs {
		if jobFailed(job) {
			names = append(names, job.Name)
		}
	}
	return names
}

func (h *obsConnectHandler) GetSecurityAlerts(
	ctx context.Context,
	_ *observabilityv1.GetSecurityAlertsRequest,
) (*observabilityv1.GetSecurityAlertsResponse, error) {
	if err := RequireAdmin(ctx); err != nil {
		return nil, err
	}
	return h.securityAlerts(ctx), nil
}

func (h *obsConnectHandler) securityAlerts(
	ctx context.Context,
) *observabilityv1.GetSecurityAlertsResponse {
	resp := &observabilityv1.GetSecurityAlertsResponse{
		Alerts:     []*observabilityv1.SecurityAlert{},
		Configured: true,
		AlertCount: 0,
	}

	alerts, err := h.deps.GithubClient.ListSecurityAlerts(ctx)
	if err != nil {
		if errors.Is(err, github.ErrNotConfigured) {
			resp.Configured = false
		} else {
			h.deps.Logger.WarnContext(ctx, "security alerts unavailable",
				slog.Any("error", err))
			resp.Error = h.securityAlertsError(ctx, err)
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
			AdvisoryId:  a.AdvisoryID,
			Url:         a.URL,
			CreatedAt:   a.CreatedAt.Format(time.RFC3339),
			AlertType:   securityAlertTypeToProto(a.Type),
			RuleId:      a.RuleID,
			FilePath:    a.FilePath,
			Line:        int32(a.Line), //nolint:gosec // line numbers fit int32
			SecretType:  a.SecretTypeDisplayName,

			FirstPatchedVersion: a.FirstPatchedVersion,
		}
	}
	resp.Alerts = protoAlerts
	resp.AlertCount = int32(len(alerts)) //nolint:gosec // count fits int32
	return resp
}

// securityAlertTypeToProto maps an alert type, defaulting to unspecified.
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
	req *observabilityv1.DismissSecurityAlertRequest,
) (*observabilityv1.DismissSecurityAlertResponse, error) {
	if err := RequireAdmin(ctx); err != nil {
		return nil, err
	}
	resp, err := h.dismissSecurityAlert(
		ctx,
		securityAlertTypeFromProto(req.GetAlertType()),
		req.GetAlertNumber(),
		req.GetReason(),
	)
	if err != nil {
		return nil, err
	}
	return resp, nil
}

// dismissSecurityAlert is one of the deliberate mutations on the observability
// surface. Takes github.SecurityAlertType so the MCP tool can pass its
// alert_type string straight through.
func (h *obsConnectHandler) dismissSecurityAlert(
	ctx context.Context,
	alertType github.SecurityAlertType,
	alertNumber int64,
	reason string,
) (*observabilityv1.DismissSecurityAlertResponse, error) {
	err := h.deps.GithubClient.DismissSecurityAlert(ctx, alertType, alertNumber, reason)
	if err != nil {
		if errors.Is(err, github.ErrInvalidDismissReason) {
			return nil, connect.NewError(connect.CodeInvalidArgument, err.Error()).WithCause(err)
		}
		if errors.Is(err, github.ErrNotConfigured) {
			return nil, connect.NewError(connect.CodeFailedPrecondition, err.Error()).
				WithCause(err)
		}
		return nil, connect.NewError(connect.CodeInternal, err.Error()).WithCause(err)
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
	req *observabilityv1.GetSentryIssuesRequest,
) (*observabilityv1.GetSentryIssuesResponse, error) {
	if err := RequireAdmin(ctx); err != nil {
		return nil, err
	}
	return h.sentryIssues(ctx, req.GetRepo()), nil
}

// sentryIssues tags each issue with the repo that owns its project; repo
// ("owner/name") limits the result to that repo's issues.
func (h *obsConnectHandler) sentryIssues(
	ctx context.Context, repo string,
) *observabilityv1.GetSentryIssuesResponse {
	resp := &observabilityv1.GetSentryIssuesResponse{
		Issues:          []*observabilityv1.SentryIssue{},
		Configured:      true,
		UnresolvedCount: 0,
	}

	issues, err := h.deps.SentryClient.ListUnresolvedIssues(ctx)
	if err != nil {
		if errors.Is(err, sentryapi.ErrNotConfigured) {
			resp.Configured = false
		} else {
			h.deps.Logger.WarnContext(ctx, "sentry issues unavailable",
				slog.Any("error", err))
		}
		return resp
	}

	repos := h.monitoredRepos(ctx)
	protoIssues := make([]*observabilityv1.SentryIssue, 0, len(issues))
	for _, is := range issues {
		owner := reposignals.RepoForSentryProject(repos, is.Project)
		if repo != "" && !strings.EqualFold(owner, repo) {
			continue
		}
		protoIssues = append(protoIssues, &observabilityv1.SentryIssue{
			Id:        is.ID,
			Title:     is.Title,
			Culprit:   is.Culprit,
			Permalink: is.Permalink,
			Count:     is.Count,
			LastSeen:  is.LastSeen.Format(time.RFC3339),
			Level:     is.Level,
			Project:   is.Project,
			Repo:      owner,
		})
	}
	resp.Issues = protoIssues
	resp.UnresolvedCount = int32(len(protoIssues)) //nolint:gosec // count fits int32
	return resp
}

func (h *obsConnectHandler) GetSentryIssueEvent(
	ctx context.Context,
	req *observabilityv1.GetSentryIssueEventRequest,
) (*observabilityv1.GetSentryIssueEventResponse, error) {
	if err := RequireAdmin(ctx); err != nil {
		return nil, err
	}
	return h.sentryIssueEvent(ctx, req.GetIssueId())
}

func (h *obsConnectHandler) sentryIssueEvent(
	ctx context.Context, issueID string,
) (*observabilityv1.GetSentryIssueEventResponse, error) {
	ev, err := h.deps.SentryClient.GetLatestEvent(ctx, issueID)
	if err != nil {
		if errors.Is(err, sentryapi.ErrNotConfigured) {
			return nil, connect.NewError(connect.CodeFailedPrecondition, err.Error()).
				WithCause(err)
		}
		return nil, connect.NewError(connect.CodeInternal, err.Error()).WithCause(err)
	}

	spans := make([]*observabilityv1.SentrySpan, len(ev.Spans))
	for i, sp := range ev.Spans {
		spans[i] = &observabilityv1.SentrySpan{
			Op:          sp.Op,
			Description: sp.Description,
			DurationMs:  sp.DurationMs,
			Offender:    sp.Offender,
		}
	}
	return &observabilityv1.GetSentryIssueEventResponse{
		EventId:     ev.ID,
		Title:       ev.Title,
		DateCreated: ev.DateCreated.Format(time.RFC3339),
		Spans:       spans,
	}, nil
}

func (h *obsConnectHandler) ResolveSentryIssue(
	ctx context.Context,
	req *observabilityv1.ResolveSentryIssueRequest,
) (*observabilityv1.ResolveSentryIssueResponse, error) {
	if err := RequireAdmin(ctx); err != nil {
		return nil, err
	}
	resp, err := h.resolveSentryIssue(ctx, req.GetIssueId())
	if err != nil {
		return nil, err
	}
	return resp, nil
}

func (h *obsConnectHandler) resolveSentryIssue(
	ctx context.Context, issueID string,
) (*observabilityv1.ResolveSentryIssueResponse, error) {
	status, err := h.deps.SentryClient.ResolveIssue(ctx, issueID)
	if err != nil {
		if errors.Is(err, sentryapi.ErrReauthRequired) {
			// CodeUnauthenticated (not FailedPrecondition) makes the UI prompt a
			// reconnect.
			return nil, connect.NewError(connect.CodeUnauthenticated, err.Error()).WithCause(err)
		}
		if errors.Is(err, sentryapi.ErrNotConfigured) {
			return nil, connect.NewError(connect.CodeFailedPrecondition, err.Error()).
				WithCause(err)
		}
		return nil, connect.NewError(connect.CodeInternal, err.Error()).WithCause(err)
	}
	return &observabilityv1.ResolveSentryIssueResponse{
		IssueId: issueID,
		Status:  status,
	}, nil
}

func (h *obsConnectHandler) GetSlowTransactions(
	ctx context.Context,
	_ *observabilityv1.GetSlowTransactionsRequest,
) (*observabilityv1.GetSlowTransactionsResponse, error) {
	if err := RequireAdmin(ctx); err != nil {
		return nil, err
	}
	resp, err := h.slowTransactions(ctx)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err.Error()).WithCause(err)
	}
	return resp, nil
}

// slowTransactions combines current (live from Sentry, guarded like the other
// external sections) with trending (from stored history, fetched even when
// Sentry is unreachable).
func (h *obsConnectHandler) slowTransactions(
	ctx context.Context,
) (*observabilityv1.GetSlowTransactionsResponse, error) {
	resp := &observabilityv1.GetSlowTransactionsResponse{
		Current:    []*observabilityv1.SlowTransaction{},
		Configured: true,
		Trending:   []*observabilityv1.TransactionTrend{},
	}

	stats, err := h.deps.SentryClient.ListTransactionStats(ctx)
	if err != nil {
		if errors.Is(err, sentryapi.ErrNotConfigured) {
			resp.Configured = false
		} else {
			return nil, err
		}
	} else {
		resp.Current = protoSlowTransactions(stats)
	}

	trends, err := h.deps.TransactionLatencyRepo.Trends(ctx)
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

// longPollTransactions are the runner's deliberate 20s holds; their p95 is the
// poll interval, not slowness.
//
//nolint:gochecknoglobals //effectively const
var longPollTransactions = map[string]bool{
	"POST " + factoryv1connect.RunnerServiceClaimJobProcedure:  true,
	"POST " + factoryv1connect.RunnerServiceHeartbeatProcedure: true,
}

// protoSlowTransactions drops long-poll transactions, sorts slowest-first and caps to
// currentSlowTransactionsLimit.
func protoSlowTransactions(
	stats []sentryapi.TransactionStat,
) []*observabilityv1.SlowTransaction {
	sorted := make([]sentryapi.TransactionStat, 0, len(stats))
	for _, s := range stats {
		if !longPollTransactions[s.Transaction] {
			sorted = append(sorted, s)
		}
	}
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

func (h *obsConnectHandler) GetLogs(
	ctx context.Context,
	req *observabilityv1.GetLogsRequest,
) (*observabilityv1.GetLogsResponse, error) {
	if err := RequireAdmin(ctx); err != nil {
		return nil, err
	}

	resp, err := h.logs(ctx, req)
	if err != nil {
		return nil, err
	}
	return resp, nil
}

// maxLogEntries caps one GetLogs response.
const maxLogEntries = 1000

// logsFilter parses req's timestamps; since defaults to defaultWindowDays ago.
func logsFilter(
	req *observabilityv1.GetLogsRequest,
) (platformrepositories.LogsFilter, error) {
	filter := platformrepositories.LogsFilter{
		Since:    time.Now().Add(-defaultWindowDays * 24 * time.Hour),
		Until:    time.Time{},
		Source:   req.GetSource(),
		MinLevel: req.GetMinLevel(),
		Contains: req.GetContains(),
		Limit:    maxLogEntries,
	}
	var err error
	if since := req.GetSince(); since != "" {
		if filter.Since, err = time.Parse(time.RFC3339, since); err != nil {
			return filter, fmt.Errorf("since: %w", err)
		}
	}
	if until := req.GetUntil(); until != "" {
		if filter.Until, err = time.Parse(time.RFC3339, until); err != nil {
			return filter, fmt.Errorf("until: %w", err)
		}
	}
	return filter, nil
}

func (h *obsConnectHandler) logs(
	ctx context.Context, req *observabilityv1.GetLogsRequest,
) (*observabilityv1.GetLogsResponse, error) {
	filter, err := logsFilter(req)
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err.Error()).WithCause(err)
	}

	entries, err := h.deps.LogsRepo.Query(ctx, filter)
	if errors.Is(err, platformrepositories.ErrUnknownLogLevel) {
		return nil, connect.NewError(connect.CodeInvalidArgument, err.Error()).WithCause(err)
	}
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err.Error()).WithCause(err)
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
	_ *observabilityv1.GetHealthOverviewRequest,
) (*observabilityv1.GetHealthOverviewResponse, error) {
	if err := RequireAdmin(ctx); err != nil {
		return nil, err
	}

	return &observabilityv1.GetHealthOverviewResponse{
		Sentry: h.sentryIssues(ctx, ""),
	}, nil
}
