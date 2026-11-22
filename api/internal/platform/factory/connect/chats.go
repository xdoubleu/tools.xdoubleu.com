package connect

import (
	"context"
	"time"

	"github.com/google/uuid"

	factoryv1 "tools.xdoubleu.com/gen/factory/v1"
	"tools.xdoubleu.com/gen/factory/v1/factoryv1connect"
	"tools.xdoubleu.com/internal/platform/factory/internal/models"
	"tools.xdoubleu.com/internal/platform/factory/internal/services"
	"tools.xdoubleu.com/internal/platform/factory/present"
)

// ChatPublisher pushes a chat's new events to the sockets watching it.
type ChatPublisher interface {
	Publish(id uuid.UUID)
}

type chatConnectHandler struct {
	app *Deps
}

var _ factoryv1connect.ChatServiceHandler = (*chatConnectHandler)(nil)

func protoChat(c *models.ChatSession) *factoryv1.Chat {
	out := &factoryv1.Chat{
		Id:        c.ID.String(),
		RepoId:    c.RepoID.String(),
		TaskId:    "",
		Harness:   present.HarnessToProto(c.Harness),
		RunnerId:  c.RunnerID.String(),
		State:     string(c.State),
		Branch:    c.Branch,
		Report:    c.Report,
		LastSeq:   int32(c.LastSeq), //nolint:gosec // bounded by the event cap
		CreatedBy: c.CreatedBy,
		CreatedAt: c.CreatedAt.Format(time.RFC3339),
		UpdatedAt: c.UpdatedAt.Format(time.RFC3339),
		StoppedAt: formatTime(c.StoppedAt),
	}
	if c.TaskID != uuid.Nil {
		out.TaskId = c.TaskID.String()
	}
	return out
}

func (h *chatConnectHandler) publish(id uuid.UUID) {
	if h.app.Chats != nil {
		h.app.Chats.Publish(id)
	}
}

func (h *chatConnectHandler) StartChat(
	ctx context.Context,
	req *factoryv1.StartChatRequest,
) (*factoryv1.StartChatResponse, error) {
	actor, err := actorFrom(ctx)
	if err != nil {
		return nil, err
	}
	repoID, err := present.ParseID(req.GetRepoId())
	if err != nil {
		return nil, err
	}
	runnerID, err := present.ParseID(req.GetRunnerId())
	if err != nil {
		return nil, err
	}
	taskID := uuid.Nil
	if req.GetTaskId() != "" {
		if taskID, err = present.ParseID(req.GetTaskId()); err != nil {
			return nil, err
		}
	}
	session, err := h.app.Services.Chats.Start(ctx, actor, services.StartChat{
		RepoID: repoID, TaskID: taskID, RunnerID: runnerID,
		Harness: harnessFromProto(req.GetHarness()),
	})
	if err != nil {
		return nil, present.MapError(err)
	}
	return &factoryv1.StartChatResponse{Chat: protoChat(session)}, nil
}

func (h *chatConnectHandler) SendChatMessage(
	ctx context.Context,
	req *factoryv1.SendChatMessageRequest,
) (*factoryv1.SendChatMessageResponse, error) {
	id, err := h.reply(ctx, req.GetId(), req.GetText(), h.app.Services.Chats.SendMessage)
	if err != nil {
		return nil, err
	}
	h.publish(id)
	return &factoryv1.SendChatMessageResponse{}, nil
}

func (h *chatConnectHandler) AnswerChat(
	ctx context.Context,
	req *factoryv1.AnswerChatRequest,
) (*factoryv1.AnswerChatResponse, error) {
	id, err := h.reply(ctx, req.GetId(), req.GetText(), h.app.Services.Chats.Answer)
	if err != nil {
		return nil, err
	}
	h.publish(id)
	return &factoryv1.AnswerChatResponse{}, nil
}

type replyFunc func(
	ctx context.Context, actor models.Actor, id uuid.UUID, text string,
) error

func (h *chatConnectHandler) reply(
	ctx context.Context,
	rawID, text string,
	do replyFunc,
) (uuid.UUID, error) {
	actor, err := actorFrom(ctx)
	if err != nil {
		return uuid.Nil, err
	}
	id, err := present.ParseID(rawID)
	if err != nil {
		return uuid.Nil, err
	}
	if err = do(ctx, actor, id, text); err != nil {
		return uuid.Nil, present.MapError(err)
	}
	return id, nil
}

func (h *chatConnectHandler) StopChat(
	ctx context.Context,
	req *factoryv1.StopChatRequest,
) (*factoryv1.StopChatResponse, error) {
	actor, err := actorFrom(ctx)
	if err != nil {
		return nil, err
	}
	id, err := present.ParseID(req.GetId())
	if err != nil {
		return nil, err
	}
	session, err := h.app.Services.Chats.Stop(ctx, actor, id, req.GetReport())
	if err != nil {
		return nil, present.MapError(err)
	}
	h.publish(id)
	return &factoryv1.StopChatResponse{Chat: protoChat(session)}, nil
}

func (h *chatConnectHandler) ListChats(
	ctx context.Context,
	req *factoryv1.ListChatsRequest,
) (*factoryv1.ListChatsResponse, error) {
	actor, err := actorFrom(ctx)
	if err != nil {
		return nil, err
	}
	sessions, err := h.app.Services.Chats.List(ctx, actor, req.GetActiveOnly())
	if err != nil {
		return nil, present.MapError(err)
	}
	out := make([]*factoryv1.Chat, len(sessions))
	for i := range sessions {
		out[i] = protoChat(&sessions[i])
	}
	return &factoryv1.ListChatsResponse{Chats: out}, nil
}

// ActorFrom is the person behind a request to a cookie-gated route.
func ActorFrom(ctx context.Context) (models.Actor, error) {
	return actorFrom(ctx)
}
