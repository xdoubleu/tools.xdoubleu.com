package main

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"connectrpc.com/connect"

	observabilityv1 "tools.xdoubleu.com/gen/observability/v1"
	"tools.xdoubleu.com/internal/digitalocean"
	"tools.xdoubleu.com/internal/github"
	"tools.xdoubleu.com/internal/sentryapi"
)

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

func (h *obsConnectHandler) GetDeployLogs(
	ctx context.Context,
	req *connect.Request[observabilityv1.GetDeployLogsRequest],
) (*connect.Response[observabilityv1.GetDeployLogsResponse], error) {
	if err := requireAdmin(ctx); err != nil {
		return nil, err
	}
	return connect.NewResponse(h.deployLogs(
		ctx, req.Msg.GetDeploymentId(), int(req.Msg.GetTailLines()),
	)), nil
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
	h.app.logger.WarnContext(ctx, "deploy logs unavailable", slog.Any("error", err))
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
