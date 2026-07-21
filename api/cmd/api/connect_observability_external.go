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

func (h *obsConnectHandler) GetGithubIssues(
	ctx context.Context,
	_ *connect.Request[observabilityv1.GetGithubIssuesRequest],
) (*connect.Response[observabilityv1.GetGithubIssuesResponse], error) {
	if err := requireAdmin(ctx); err != nil {
		return nil, err
	}
	return connect.NewResponse(h.githubIssues(ctx)), nil
}

func (h *obsConnectHandler) githubIssues(
	ctx context.Context,
) *observabilityv1.GetGithubIssuesResponse {
	resp := &observabilityv1.GetGithubIssuesResponse{
		Issues:     []*observabilityv1.GithubIssue{},
		Configured: true,
		OpenCount:  0,
	}

	issues, err := h.app.githubClient.ListOpenIssues(ctx)
	if err != nil {
		if errors.Is(err, github.ErrNotConfigured) {
			resp.Configured = false
		} else {
			h.app.logger.WarnContext(ctx, "github issues unavailable",
				slog.Any("error", err))
		}
		return resp
	}

	protoIssues := make([]*observabilityv1.GithubIssue, len(issues))
	for i, is := range issues {
		protoIssues[i] = &observabilityv1.GithubIssue{
			Number:    is.Number,
			Title:     is.Title,
			Url:       is.URL,
			State:     is.State,
			CreatedAt: is.CreatedAt.Format(time.RFC3339),
			Labels:    is.Labels,
		}
	}
	resp.Issues = protoIssues
	resp.OpenCount = int32(len(issues)) //nolint:gosec // issue count fits int32
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

func (h *obsConnectHandler) GetHealthOverview(
	ctx context.Context,
	_ *connect.Request[observabilityv1.GetHealthOverviewRequest],
) (*connect.Response[observabilityv1.GetHealthOverviewResponse], error) {
	if err := requireAdmin(ctx); err != nil {
		return nil, err
	}

	return connect.NewResponse(&observabilityv1.GetHealthOverviewResponse{
		Github: h.githubIssues(ctx),
		Sentry: h.sentryIssues(ctx),
		Deploy: h.deployStatus(ctx),
	}), nil
}
