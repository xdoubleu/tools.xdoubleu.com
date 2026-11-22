package connect

import (
	"context"
	"time"

	"tools.xdoubleu.com/apps/learningpaths/internal/protomap"
	"tools.xdoubleu.com/apps/learningpaths/internal/services"
	learningpathsv1 "tools.xdoubleu.com/gen/learningpaths/v1"
	"tools.xdoubleu.com/gen/learningpaths/v1/learningpathsv1connect"
	"tools.xdoubleu.com/internal/connecttools"
)

// TodoistHandler serves the TodoistService RPCs.
type TodoistHandler struct {
	todoist *services.TodoistService
}

// NewTodoistHandler returns a TodoistHandler backed by svc.
func NewTodoistHandler(svc *services.TodoistService) *TodoistHandler {
	return &TodoistHandler{todoist: svc}
}

var _ learningpathsv1connect.TodoistServiceHandler = (*TodoistHandler)(nil)

func (h *TodoistHandler) ConnectTodoist(
	ctx context.Context,
	_ *learningpathsv1.ConnectTodoistRequest,
) (*learningpathsv1.ConnectTodoistResponse, error) {
	uid, err := userID(ctx)
	if err != nil {
		return nil, err
	}

	url := h.todoist.AuthorizeURL(uid)
	return &learningpathsv1.ConnectTodoistResponse{
		AuthorizeUrl: url,
	}, nil
}

func (h *TodoistHandler) DisconnectTodoist(
	ctx context.Context,
	_ *learningpathsv1.DisconnectTodoistRequest,
) (*learningpathsv1.DisconnectTodoistResponse, error) {
	uid, err := userID(ctx)
	if err != nil {
		return nil, err
	}

	if err = h.todoist.Disconnect(ctx, uid); err != nil {
		return nil, connecttools.MapError(err)
	}
	return &learningpathsv1.DisconnectTodoistResponse{}, nil
}

func (h *TodoistHandler) GetTodoistConnectionStatus(
	ctx context.Context,
	_ *learningpathsv1.GetTodoistConnectionStatusRequest,
) (*learningpathsv1.GetTodoistConnectionStatusResponse, error) {
	uid, err := userID(ctx)
	if err != nil {
		return nil, err
	}

	st, err := h.todoist.Status(ctx, uid)
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

func (h *TodoistHandler) GetTodoistSyncState(
	ctx context.Context,
	req *learningpathsv1.GetTodoistSyncStateRequest,
) (*learningpathsv1.GetTodoistSyncStateResponse, error) {
	uid, err := userID(ctx)
	if err != nil {
		return nil, err
	}

	pathID, err := parsePathID(req.LearningPathId)
	if err != nil {
		return nil, err
	}

	state, err := h.todoist.SyncState(ctx, uid, pathID)
	if err != nil {
		return nil, connecttools.MapError(err)
	}

	return protomap.TodoistSyncState(state), nil
}
