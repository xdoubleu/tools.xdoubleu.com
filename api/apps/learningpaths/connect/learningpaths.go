package connect

import (
	"context"

	"connectrpc.com/connect/v2"
	"github.com/google/uuid"

	"tools.xdoubleu.com/apps/learningpaths/internal/models"
	"tools.xdoubleu.com/apps/learningpaths/internal/protomap"
	learningpathsv1 "tools.xdoubleu.com/gen/learningpaths/v1"
	"tools.xdoubleu.com/internal/connecttools"
)

func (h *Handler) ListLearningPaths(
	ctx context.Context,
	req *learningpathsv1.ListLearningPathsRequest,
) (*learningpathsv1.ListLearningPathsResponse, error) {
	uid, err := userID(ctx)
	if err != nil {
		return nil, err
	}

	list, hasMore, err := h.paths.List(
		ctx, uid, req.Limit, req.Offset,
	)
	if err != nil {
		return nil, connecttools.MapError(err)
	}

	return &learningpathsv1.ListLearningPathsResponse{
		LearningPaths: protomap.LearningPaths(list),
		HasMore:       hasMore,
	}, nil
}

func (h *Handler) GetLearningPath(
	ctx context.Context,
	req *learningpathsv1.GetLearningPathRequest,
) (*learningpathsv1.GetLearningPathResponse, error) {
	uid, err := userID(ctx)
	if err != nil {
		return nil, err
	}

	id, err := parsePathID(req.Id)
	if err != nil {
		return nil, err
	}

	lp, err := h.paths.Get(ctx, id, uid)
	if err != nil {
		return nil, connecttools.MapError(err)
	}

	return &learningpathsv1.GetLearningPathResponse{
		LearningPath: protomap.LearningPath(lp),
	}, nil
}

func (h *Handler) CreateLearningPath(
	ctx context.Context,
	req *learningpathsv1.CreateLearningPathRequest,
) (*learningpathsv1.CreateLearningPathResponse, error) {
	uid, err := userID(ctx)
	if err != nil {
		return nil, err
	}

	//nolint:exhaustruct //ID/UserID/timestamps assigned by the service/repository
	lp := models.LearningPath{
		Title:             req.Title,
		Goal:              req.Goal,
		Routine:           req.Routine,
		Modules:           protomap.ModulesFromProto(req.Modules),
		Resources:         protomap.ResourcesFromProto(req.Resources),
		ReminderSchedules: req.ReminderSchedules,
	}

	created, err := h.paths.Create(ctx, uid, lp)
	if err != nil {
		return nil, connecttools.MapError(err)
	}

	return &learningpathsv1.CreateLearningPathResponse{
		LearningPath: protomap.LearningPath(created),
	}, nil
}

func (h *Handler) UpdateLearningPath(
	ctx context.Context,
	req *learningpathsv1.UpdateLearningPathRequest,
) (*learningpathsv1.UpdateLearningPathResponse, error) {
	uid, err := userID(ctx)
	if err != nil {
		return nil, err
	}

	id, err := parsePathID(req.Id)
	if err != nil {
		return nil, err
	}

	//nolint:exhaustruct //UserID/timestamps assigned by the service/repository
	lp := models.LearningPath{
		ID:                id,
		Title:             req.Title,
		Goal:              req.Goal,
		Routine:           req.Routine,
		Modules:           protomap.ModulesFromProto(req.Modules),
		Resources:         protomap.ResourcesFromProto(req.Resources),
		ReminderSchedules: req.ReminderSchedules,
	}

	if err = h.paths.Update(ctx, uid, lp); err != nil {
		return nil, connecttools.MapError(err)
	}

	updated, err := h.paths.Get(ctx, id, uid)
	if err != nil {
		return nil, connecttools.MapError(err)
	}

	return &learningpathsv1.UpdateLearningPathResponse{
		LearningPath: protomap.LearningPath(updated),
	}, nil
}

func (h *Handler) DeleteLearningPath(
	ctx context.Context,
	req *learningpathsv1.DeleteLearningPathRequest,
) (*learningpathsv1.DeleteLearningPathResponse, error) {
	uid, err := userID(ctx)
	if err != nil {
		return nil, err
	}

	id, err := parsePathID(req.Id)
	if err != nil {
		return nil, err
	}

	if err = h.paths.Delete(ctx, id, uid); err != nil {
		return nil, connecttools.MapError(err)
	}

	return &learningpathsv1.DeleteLearningPathResponse{}, nil
}

func (h *Handler) RecordItemProgress(
	ctx context.Context,
	req *learningpathsv1.RecordItemProgressRequest,
) (*learningpathsv1.RecordItemProgressResponse, error) {
	uid, err := userID(ctx)
	if err != nil {
		return nil, err
	}

	itemID, err := uuid.Parse(req.ItemId)
	if err != nil {
		return nil, connect.Errorf(
			connect.CodeInvalidArgument,
			"invalid item ID",
		)
	}

	if err = h.paths.RecordItemProgress(
		ctx, uid, itemID, req.Completed,
	); err != nil {
		return nil, connecttools.MapError(err)
	}

	return &learningpathsv1.RecordItemProgressResponse{}, nil
}

func (h *Handler) SetLearningPathPaused(
	ctx context.Context,
	req *learningpathsv1.SetLearningPathPausedRequest,
) (*learningpathsv1.SetLearningPathPausedResponse, error) {
	uid, err := userID(ctx)
	if err != nil {
		return nil, err
	}

	id, err := parsePathID(req.Id)
	if err != nil {
		return nil, err
	}

	if err = h.paths.SetPaused(
		ctx, id, uid, req.Paused,
	); err != nil {
		return nil, connecttools.MapError(err)
	}

	lp, err := h.paths.Get(ctx, id, uid)
	if err != nil {
		return nil, connecttools.MapError(err)
	}

	return &learningpathsv1.SetLearningPathPausedResponse{
		LearningPath: protomap.LearningPath(lp),
	}, nil
}

func (h *Handler) GetLearningPathProgress(
	ctx context.Context,
	req *learningpathsv1.GetLearningPathProgressRequest,
) (*learningpathsv1.GetLearningPathProgressResponse, error) {
	uid, err := userID(ctx)
	if err != nil {
		return nil, err
	}

	id, err := parsePathID(req.Id)
	if err != nil {
		return nil, err
	}

	progress, err := h.paths.Progress(ctx, id, uid)
	if err != nil {
		return nil, connecttools.MapError(err)
	}

	return protomap.Progress(progress), nil
}
