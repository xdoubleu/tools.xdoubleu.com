package main

import (
	"context"
	"errors"
	"log/slog"

	"connectrpc.com/connect"

	observabilityv1 "tools.xdoubleu.com/gen/observability/v1"
	"tools.xdoubleu.com/internal/github"
)

// GetProjectIssuesByStatus surfaces GitHub Projects (v2) board status (issue
// #1357) — the separate GitHub MCP server tooling can't resolve custom
// fields on a personal (user-owned) project board, so an admin-authenticated
// agent has no other way to answer "which issues are in the Ready column".
func (h *obsConnectHandler) GetProjectIssuesByStatus(
	ctx context.Context,
	req *connect.Request[observabilityv1.GetProjectIssuesByStatusRequest],
) (*connect.Response[observabilityv1.GetProjectIssuesByStatusResponse], error) {
	if err := requireAdmin(ctx); err != nil {
		return nil, err
	}
	return connect.NewResponse(h.projectIssuesByStatus(
		ctx, req.Msg.GetProjectNumber(), req.Msg.GetStatus(),
	)), nil
}

func (h *obsConnectHandler) projectIssuesByStatus(
	ctx context.Context, projectNumber int32, status string,
) *observabilityv1.GetProjectIssuesByStatusResponse {
	resp := &observabilityv1.GetProjectIssuesByStatusResponse{
		Issues:     []*observabilityv1.ProjectIssue{},
		Configured: true,
	}

	issues, err := h.app.githubClient.ListProjectIssuesByStatus(
		ctx, int64(projectNumber), status,
	)
	if err != nil {
		if errors.Is(err, github.ErrNotConfigured) {
			resp.Configured = false
		} else {
			h.app.logger.WarnContext(ctx, "project issues unavailable",
				slog.Any("error", err))
		}
		return resp
	}

	protoIssues := make([]*observabilityv1.ProjectIssue, len(issues))
	for i, is := range issues {
		protoIssues[i] = &observabilityv1.ProjectIssue{
			Number: is.Number,
			Title:  is.Title,
			Url:    is.URL,
			Status: is.Status,
		}
	}
	resp.Issues = protoIssues
	return resp
}
