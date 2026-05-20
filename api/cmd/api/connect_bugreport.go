package main

import (
	"context"
	"errors"
	"strings"

	"connectrpc.com/connect"
	"github.com/xdoubleu/essentia/v4/pkg/contexttools"

	bugreportv1 "tools.xdoubleu.com/gen/bugreport/v1"
	"tools.xdoubleu.com/gen/bugreport/v1/bugreportv1connect"
	"tools.xdoubleu.com/internal/constants"
	"tools.xdoubleu.com/internal/models"
)

type bugReportConnectHandler struct {
	app *Application
}

var _ bugreportv1connect.BugReportServiceHandler = (*bugReportConnectHandler)(nil)

func (h *bugReportConnectHandler) CreateBugReport(
	ctx context.Context,
	req *connect.Request[bugreportv1.CreateBugReportRequest],
) (*connect.Response[bugreportv1.CreateBugReportResponse], error) {
	if h.app.config.GitHubToken == "" || h.app.config.GitHubRepo == "" {
		return nil, connect.NewError(
			connect.CodeUnavailable,
			errors.New("bug reporting is not configured"),
		)
	}

	if strings.TrimSpace(req.Msg.Title) == "" ||
		strings.TrimSpace(req.Msg.Description) == "" {
		return nil, connect.NewError(
			connect.CodeInvalidArgument,
			errors.New("title and description must not be empty"),
		)
	}

	user := contexttools.GetValue[models.User](ctx, constants.UserContextKey)

	body := buildIssueBody(
		req.Msg.Description,
		req.Msg.Page,
		req.Header().Get("User-Agent"),
		h.app.config.Release,
		h.app.config.Env,
		user.ID,
		h.app.requestBuffer.Get(user.ID),
		req.Msg.ConsoleLogs,
		req.Msg.WsLog,
	)

	issueURL, err := createGitHubIssue(
		ctx,
		h.app.config.GitHubToken,
		h.app.config.GitHubRepo,
		req.Msg.Title,
		body,
	)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	return connect.NewResponse(&bugreportv1.CreateBugReportResponse{
		Url: issueURL,
	}), nil
}
