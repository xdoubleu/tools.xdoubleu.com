package learningpaths

import (
	"context"

	"connectrpc.com/connect/v2"
	"github.com/google/uuid"

	"tools.xdoubleu.com/apps/learningpaths/internal/models"
	learningpathsv1 "tools.xdoubleu.com/gen/learningpaths/v1"
)

func (h *learningPathsConnectHandler) ListLearningPaths(
	ctx context.Context,
	req *learningpathsv1.ListLearningPathsRequest,
) (*learningpathsv1.ListLearningPathsResponse, error) {
	user := getUser(ctx)
	if user == nil {
		return nil, connect.Errorf(
			connect.CodeUnauthenticated,
			"user not authenticated",
		)
	}

	list, hasMore, err := h.app.services.LearningPaths.List(
		ctx, user.ID, req.Limit, req.Offset,
	)
	if err != nil {
		return nil, mapError(err)
	}

	return &learningpathsv1.ListLearningPathsResponse{
		LearningPaths: protoLearningPaths(list),
		HasMore:       hasMore,
	}, nil
}

func (h *learningPathsConnectHandler) GetLearningPath(
	ctx context.Context,
	req *learningpathsv1.GetLearningPathRequest,
) (*learningpathsv1.GetLearningPathResponse, error) {
	user := getUser(ctx)
	if user == nil {
		return nil, connect.Errorf(
			connect.CodeUnauthenticated,
			"user not authenticated",
		)
	}

	id, err := uuid.Parse(req.Id)
	if err != nil {
		return nil, connect.Errorf(
			connect.CodeInvalidArgument,
			"invalid learning path ID",
		)
	}

	lp, err := h.app.services.LearningPaths.Get(ctx, id, user.ID)
	if err != nil {
		return nil, mapError(err)
	}

	return &learningpathsv1.GetLearningPathResponse{
		LearningPath: protoLearningPath(lp),
	}, nil
}

func (h *learningPathsConnectHandler) CreateLearningPath(
	ctx context.Context,
	req *learningpathsv1.CreateLearningPathRequest,
) (*learningpathsv1.CreateLearningPathResponse, error) {
	user := getUser(ctx)
	if user == nil {
		return nil, connect.Errorf(
			connect.CodeUnauthenticated,
			"user not authenticated",
		)
	}

	//nolint:exhaustruct //ID/UserID/timestamps assigned by the service/repository
	lp := models.LearningPath{
		Title:             req.Title,
		Goal:              req.Goal,
		Routine:           req.Routine,
		Modules:           dtoToModules(req.Modules),
		Resources:         dtoToResources(req.Resources),
		ReminderSchedules: req.ReminderSchedules,
	}

	created, err := h.app.services.LearningPaths.Create(ctx, user.ID, lp)
	if err != nil {
		return nil, mapError(err)
	}

	return &learningpathsv1.CreateLearningPathResponse{
		LearningPath: protoLearningPath(created),
	}, nil
}

func (h *learningPathsConnectHandler) UpdateLearningPath(
	ctx context.Context,
	req *learningpathsv1.UpdateLearningPathRequest,
) (*learningpathsv1.UpdateLearningPathResponse, error) {
	user := getUser(ctx)
	if user == nil {
		return nil, connect.Errorf(
			connect.CodeUnauthenticated,
			"user not authenticated",
		)
	}

	id, err := uuid.Parse(req.Id)
	if err != nil {
		return nil, connect.Errorf(
			connect.CodeInvalidArgument,
			"invalid learning path ID",
		)
	}

	//nolint:exhaustruct //UserID/timestamps assigned by the service/repository
	lp := models.LearningPath{
		ID:                id,
		Title:             req.Title,
		Goal:              req.Goal,
		Routine:           req.Routine,
		Modules:           dtoToModules(req.Modules),
		Resources:         dtoToResources(req.Resources),
		ReminderSchedules: req.ReminderSchedules,
	}

	if err = h.app.services.LearningPaths.Update(ctx, user.ID, lp); err != nil {
		return nil, mapError(err)
	}

	updated, err := h.app.services.LearningPaths.Get(ctx, id, user.ID)
	if err != nil {
		return nil, mapError(err)
	}

	return &learningpathsv1.UpdateLearningPathResponse{
		LearningPath: protoLearningPath(updated),
	}, nil
}

func (h *learningPathsConnectHandler) DeleteLearningPath(
	ctx context.Context,
	req *learningpathsv1.DeleteLearningPathRequest,
) (*learningpathsv1.DeleteLearningPathResponse, error) {
	user := getUser(ctx)
	if user == nil {
		return nil, connect.Errorf(
			connect.CodeUnauthenticated,
			"user not authenticated",
		)
	}

	id, err := uuid.Parse(req.Id)
	if err != nil {
		return nil, connect.Errorf(
			connect.CodeInvalidArgument,
			"invalid learning path ID",
		)
	}

	if err = h.app.services.LearningPaths.Delete(ctx, id, user.ID); err != nil {
		return nil, mapError(err)
	}

	return &learningpathsv1.DeleteLearningPathResponse{}, nil
}

func (h *learningPathsConnectHandler) RecordItemProgress(
	ctx context.Context,
	req *learningpathsv1.RecordItemProgressRequest,
) (*learningpathsv1.RecordItemProgressResponse, error) {
	user := getUser(ctx)
	if user == nil {
		return nil, connect.Errorf(
			connect.CodeUnauthenticated,
			"user not authenticated",
		)
	}

	itemID, err := uuid.Parse(req.ItemId)
	if err != nil {
		return nil, connect.Errorf(
			connect.CodeInvalidArgument,
			"invalid item ID",
		)
	}

	if err = h.app.services.LearningPaths.RecordItemProgress(
		ctx, user.ID, itemID, req.Completed,
	); err != nil {
		return nil, mapError(err)
	}

	return &learningpathsv1.RecordItemProgressResponse{}, nil
}

func (h *learningPathsConnectHandler) SetLearningPathPaused(
	ctx context.Context,
	req *learningpathsv1.SetLearningPathPausedRequest,
) (*learningpathsv1.SetLearningPathPausedResponse, error) {
	user := getUser(ctx)
	if user == nil {
		return nil, connect.Errorf(
			connect.CodeUnauthenticated,
			"user not authenticated",
		)
	}

	id, err := uuid.Parse(req.Id)
	if err != nil {
		return nil, connect.Errorf(
			connect.CodeInvalidArgument,
			"invalid learning path ID",
		)
	}

	if err = h.app.services.LearningPaths.SetPaused(
		ctx, id, user.ID, req.Paused,
	); err != nil {
		return nil, mapError(err)
	}

	lp, err := h.app.services.LearningPaths.Get(ctx, id, user.ID)
	if err != nil {
		return nil, mapError(err)
	}

	return &learningpathsv1.SetLearningPathPausedResponse{
		LearningPath: protoLearningPath(lp),
	}, nil
}

func (h *learningPathsConnectHandler) GetLearningPathProgress(
	ctx context.Context,
	req *learningpathsv1.GetLearningPathProgressRequest,
) (*learningpathsv1.GetLearningPathProgressResponse, error) {
	user := getUser(ctx)
	if user == nil {
		return nil, connect.Errorf(
			connect.CodeUnauthenticated,
			"user not authenticated",
		)
	}

	id, err := uuid.Parse(req.Id)
	if err != nil {
		return nil, connect.Errorf(
			connect.CodeInvalidArgument,
			"invalid learning path ID",
		)
	}

	lp, err := h.app.services.LearningPaths.GetProgress(ctx, id, user.ID)
	if err != nil {
		return nil, mapError(err)
	}

	return progressFromLearningPath(lp), nil
}
