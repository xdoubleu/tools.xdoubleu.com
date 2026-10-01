package main

import (
	"context"
	"errors"
	"time"

	"connectrpc.com/connect"

	observabilityv1 "tools.xdoubleu.com/gen/observability/v1"
	"tools.xdoubleu.com/internal/github"
)

// GetProjectIssuesByStatus returns GitHub Projects v2 board status; the GitHub
// MCP server can't resolve custom fields on a user-owned board.
func (h *obsConnectHandler) GetProjectIssuesByStatus(
	ctx context.Context,
	req *connect.Request[observabilityv1.GetProjectIssuesByStatusRequest],
) (*connect.Response[observabilityv1.GetProjectIssuesByStatusResponse], error) {
	if err := requireAdmin(ctx); err != nil {
		return nil, err
	}
	resp, err := h.projectIssuesByStatus(
		ctx, req.Msg.GetProjectNumber(), req.Msg.GetStatus(),
	)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(resp), nil
}

// projectIssuesByStatus reports configured=false without a GitHub connection;
// any other failure is an error, since an empty column would read as "none".
func (h *obsConnectHandler) projectIssuesByStatus(
	ctx context.Context, projectNumber int32, status string,
) (*observabilityv1.GetProjectIssuesByStatusResponse, error) {
	resp := &observabilityv1.GetProjectIssuesByStatusResponse{
		Issues:     []*observabilityv1.ProjectIssue{},
		Configured: true,
	}

	issues, err := h.app.githubClient.ListProjectIssuesByStatus(
		ctx, int64(projectNumber), status,
	)
	if errors.Is(err, github.ErrNotConfigured) {
		resp.Configured = false
		return resp, nil
	}
	if errors.Is(err, github.ErrInsufficientScopes) {
		return nil, connect.NewError(connect.CodeFailedPrecondition, err)
	}
	if err != nil {
		return nil, connect.NewError(connect.CodeUnavailable, err)
	}

	protoIssues := make([]*observabilityv1.ProjectIssue, len(issues))
	for i, is := range issues {
		protoIssues[i] = &observabilityv1.ProjectIssue{
			Number:                is.Number,
			Title:                 is.Title,
			Url:                   is.URL,
			Status:                is.Status,
			AuthorLogin:           is.AuthorLogin,
			AuthorAssociation:     is.AuthorAssociation,
			StatusUpdatedAt:       is.StatusUpdatedAt.Format(time.RFC3339),
			BodyHasHtmlComment:    is.BodyHasHTMLComment,
			BodyEditedAfterStatus: is.BodyEditedAfterStatus,
		}
	}
	resp.Issues = protoIssues
	return resp, nil
}
