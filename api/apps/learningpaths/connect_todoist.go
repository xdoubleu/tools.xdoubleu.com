package learningpaths

import (
	"context"
	"time"

	"connectrpc.com/connect/v2"
	"github.com/google/uuid"

	learningpathsv1 "tools.xdoubleu.com/gen/learningpaths/v1"
	"tools.xdoubleu.com/gen/learningpaths/v1/learningpathsv1connect"
	"tools.xdoubleu.com/internal/connecttools"
)

type todoistConnectHandler struct {
	app *LearningPaths
}

var _ learningpathsv1connect.TodoistServiceHandler = (*todoistConnectHandler)(nil)

func (h *todoistConnectHandler) ConnectTodoist(
	ctx context.Context,
	_ *learningpathsv1.ConnectTodoistRequest,
) (*learningpathsv1.ConnectTodoistResponse, error) {
	user := getUser(ctx)
	if user == nil {
		return nil, connect.Errorf(
			connect.CodeUnauthenticated, "user not authenticated",
		)
	}

	url := h.app.services.Todoist.AuthorizeURL(user.ID)
	return &learningpathsv1.ConnectTodoistResponse{
		AuthorizeUrl: url,
	}, nil
}

func (h *todoistConnectHandler) DisconnectTodoist(
	ctx context.Context,
	_ *learningpathsv1.DisconnectTodoistRequest,
) (*learningpathsv1.DisconnectTodoistResponse, error) {
	user := getUser(ctx)
	if user == nil {
		return nil, connect.Errorf(
			connect.CodeUnauthenticated, "user not authenticated",
		)
	}

	if err := h.app.services.Todoist.Disconnect(ctx, user.ID); err != nil {
		return nil, connecttools.MapError(err)
	}
	return &learningpathsv1.DisconnectTodoistResponse{}, nil
}

func (h *todoistConnectHandler) GetTodoistConnectionStatus(
	ctx context.Context,
	_ *learningpathsv1.GetTodoistConnectionStatusRequest,
) (*learningpathsv1.GetTodoistConnectionStatusResponse, error) {
	user := getUser(ctx)
	if user == nil {
		return nil, connect.Errorf(
			connect.CodeUnauthenticated, "user not authenticated",
		)
	}

	st, err := h.app.services.Todoist.Status(ctx, user.ID)
	if err != nil {
		return nil, connecttools.MapError(err)
	}

	resp := &learningpathsv1.GetTodoistConnectionStatusResponse{
		Connected:      st.Connected,
		NeedsReconnect: st.NeedsReconnect,
	}
	if st.Connected {
		resp.ConnectedAt = st.ConnectedAt.Format(time.RFC3339)
	}
	return resp, nil
}

func (h *todoistConnectHandler) GetTodoistSyncState(
	ctx context.Context,
	req *learningpathsv1.GetTodoistSyncStateRequest,
) (*learningpathsv1.GetTodoistSyncStateResponse, error) {
	user := getUser(ctx)
	if user == nil {
		return nil, connect.Errorf(
			connect.CodeUnauthenticated, "user not authenticated",
		)
	}

	pathID, err := uuid.Parse(req.LearningPathId)
	if err != nil {
		return nil, connect.Errorf(
			connect.CodeInvalidArgument, "invalid learning path ID",
		)
	}

	state, err := h.app.services.Todoist.SyncState(ctx, user.ID, pathID)
	if err != nil {
		return nil, connecttools.MapError(err)
	}

	items := make([]*learningpathsv1.TodoistItemState, len(state.Items))
	for i, it := range state.Items {
		items[i] = &learningpathsv1.TodoistItemState{
			ItemId:        it.ID.String(),
			Description:   it.Description,
			Due:           it.Due,
			Completed:     it.Completed,
			TodoistTaskId: derefString(it.TodoistTaskID),
		}
	}
	return &learningpathsv1.GetTodoistSyncStateResponse{
		Connected:        state.Status.Connected,
		NeedsReconnect:   state.Status.NeedsReconnect,
		RequestedScope:   state.Status.RequestedScope,
		TodoistProjectId: state.ProjectID,
		ActiveModule:     state.ActiveModule,
		Items:            items,
		Paused:           state.Paused,
	}, nil
}

func derefString(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}
