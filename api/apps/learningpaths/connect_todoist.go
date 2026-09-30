package learningpaths

import (
	"context"
	"fmt"
	"time"

	"connectrpc.com/connect"

	learningpathsv1 "tools.xdoubleu.com/gen/learningpaths/v1"
	"tools.xdoubleu.com/gen/learningpaths/v1/learningpathsv1connect"
)

type todoistConnectHandler struct {
	app *LearningPaths
}

var _ learningpathsv1connect.TodoistServiceHandler = (*todoistConnectHandler)(nil)

func (h *todoistConnectHandler) ConnectTodoist(
	ctx context.Context,
	_ *connect.Request[learningpathsv1.ConnectTodoistRequest],
) (*connect.Response[learningpathsv1.ConnectTodoistResponse], error) {
	user := getUser(ctx)
	if user == nil {
		return nil, connect.NewError(
			connect.CodeUnauthenticated, fmt.Errorf("user not authenticated"),
		)
	}

	url := h.app.services.Todoist.AuthorizeURL(user.ID)
	return connect.NewResponse(&learningpathsv1.ConnectTodoistResponse{
		AuthorizeUrl: url,
	}), nil
}

func (h *todoistConnectHandler) DisconnectTodoist(
	ctx context.Context,
	_ *connect.Request[learningpathsv1.DisconnectTodoistRequest],
) (*connect.Response[learningpathsv1.DisconnectTodoistResponse], error) {
	user := getUser(ctx)
	if user == nil {
		return nil, connect.NewError(
			connect.CodeUnauthenticated, fmt.Errorf("user not authenticated"),
		)
	}

	if err := h.app.services.Todoist.Disconnect(ctx, user.ID); err != nil {
		return nil, mapError(err)
	}
	return connect.NewResponse(&learningpathsv1.DisconnectTodoistResponse{}), nil
}

func (h *todoistConnectHandler) GetTodoistConnectionStatus(
	ctx context.Context,
	_ *connect.Request[learningpathsv1.GetTodoistConnectionStatusRequest],
) (*connect.Response[learningpathsv1.GetTodoistConnectionStatusResponse], error) {
	user := getUser(ctx)
	if user == nil {
		return nil, connect.NewError(
			connect.CodeUnauthenticated, fmt.Errorf("user not authenticated"),
		)
	}

	connected, connectedAt, err := h.app.services.Todoist.Status(ctx, user.ID)
	if err != nil {
		return nil, mapError(err)
	}

	resp := &learningpathsv1.GetTodoistConnectionStatusResponse{
		Connected: connected,
	}
	if connected {
		resp.ConnectedAt = connectedAt.Format(time.RFC3339)
	}
	return connect.NewResponse(resp), nil
}
