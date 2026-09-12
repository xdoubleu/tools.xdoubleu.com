package learningpaths

import (
	"context"
	"fmt"

	"connectrpc.com/connect"
	"github.com/google/uuid"

	"tools.xdoubleu.com/apps/learningpaths/internal/models"
	learningpathsv1 "tools.xdoubleu.com/gen/learningpaths/v1"
)

func (h *learningPathsConnectHandler) ListLearningPaths(
	ctx context.Context,
	req *connect.Request[learningpathsv1.ListLearningPathsRequest],
) (*connect.Response[learningpathsv1.ListLearningPathsResponse], error) {
	user := getUser(ctx)
	if user == nil {
		return nil, connect.NewError(
			connect.CodeUnauthenticated,
			fmt.Errorf("user not authenticated"),
		)
	}

	list, hasMore, err := h.app.services.LearningPaths.List(
		ctx, user.ID, req.Msg.Limit, req.Msg.Offset,
	)
	if err != nil {
		return nil, mapError(err)
	}

	return connect.NewResponse(&learningpathsv1.ListLearningPathsResponse{
		LearningPaths: protoLearningPaths(list),
		HasMore:       hasMore,
	}), nil
}

func (h *learningPathsConnectHandler) GetLearningPath(
	ctx context.Context,
	req *connect.Request[learningpathsv1.GetLearningPathRequest],
) (*connect.Response[learningpathsv1.GetLearningPathResponse], error) {
	user := getUser(ctx)
	if user == nil {
		return nil, connect.NewError(
			connect.CodeUnauthenticated,
			fmt.Errorf("user not authenticated"),
		)
	}

	id, err := uuid.Parse(req.Msg.Id)
	if err != nil {
		return nil, connect.NewError(
			connect.CodeInvalidArgument,
			fmt.Errorf("invalid learning path ID"),
		)
	}

	lp, err := h.app.services.LearningPaths.Get(ctx, id, user.ID)
	if err != nil {
		return nil, mapError(err)
	}

	return connect.NewResponse(&learningpathsv1.GetLearningPathResponse{
		LearningPath: protoLearningPath(lp),
	}), nil
}

func (h *learningPathsConnectHandler) CreateLearningPath(
	ctx context.Context,
	req *connect.Request[learningpathsv1.CreateLearningPathRequest],
) (*connect.Response[learningpathsv1.CreateLearningPathResponse], error) {
	user := getUser(ctx)
	if user == nil {
		return nil, connect.NewError(
			connect.CodeUnauthenticated,
			fmt.Errorf("user not authenticated"),
		)
	}

	//nolint:exhaustruct //ID/UserID/timestamps assigned by the service/repository
	lp := models.LearningPath{
		Title:     req.Msg.Title,
		Goal:      req.Msg.Goal,
		Routine:   req.Msg.Routine,
		Modules:   dtoToModules(req.Msg.Modules),
		Resources: dtoToResources(req.Msg.Resources),
	}

	created, err := h.app.services.LearningPaths.Create(ctx, user.ID, lp)
	if err != nil {
		return nil, mapError(err)
	}

	return connect.NewResponse(&learningpathsv1.CreateLearningPathResponse{
		LearningPath: protoLearningPath(created),
	}), nil
}

func (h *learningPathsConnectHandler) UpdateLearningPath(
	ctx context.Context,
	req *connect.Request[learningpathsv1.UpdateLearningPathRequest],
) (*connect.Response[learningpathsv1.UpdateLearningPathResponse], error) {
	user := getUser(ctx)
	if user == nil {
		return nil, connect.NewError(
			connect.CodeUnauthenticated,
			fmt.Errorf("user not authenticated"),
		)
	}

	id, err := uuid.Parse(req.Msg.Id)
	if err != nil {
		return nil, connect.NewError(
			connect.CodeInvalidArgument,
			fmt.Errorf("invalid learning path ID"),
		)
	}

	//nolint:exhaustruct //UserID/timestamps assigned by the service/repository
	lp := models.LearningPath{
		ID:        id,
		Title:     req.Msg.Title,
		Goal:      req.Msg.Goal,
		Routine:   req.Msg.Routine,
		Modules:   dtoToModules(req.Msg.Modules),
		Resources: dtoToResources(req.Msg.Resources),
	}

	if err = h.app.services.LearningPaths.Update(ctx, user.ID, lp); err != nil {
		return nil, mapError(err)
	}

	updated, err := h.app.services.LearningPaths.Get(ctx, id, user.ID)
	if err != nil {
		return nil, mapError(err)
	}

	return connect.NewResponse(&learningpathsv1.UpdateLearningPathResponse{
		LearningPath: protoLearningPath(updated),
	}), nil
}

func (h *learningPathsConnectHandler) DeleteLearningPath(
	ctx context.Context,
	req *connect.Request[learningpathsv1.DeleteLearningPathRequest],
) (*connect.Response[learningpathsv1.DeleteLearningPathResponse], error) {
	user := getUser(ctx)
	if user == nil {
		return nil, connect.NewError(
			connect.CodeUnauthenticated,
			fmt.Errorf("user not authenticated"),
		)
	}

	id, err := uuid.Parse(req.Msg.Id)
	if err != nil {
		return nil, connect.NewError(
			connect.CodeInvalidArgument,
			fmt.Errorf("invalid learning path ID"),
		)
	}

	if err = h.app.services.LearningPaths.Delete(ctx, id, user.ID); err != nil {
		return nil, mapError(err)
	}

	return connect.NewResponse(&learningpathsv1.DeleteLearningPathResponse{}), nil
}

func (h *learningPathsConnectHandler) RecordItemProgress(
	ctx context.Context,
	req *connect.Request[learningpathsv1.RecordItemProgressRequest],
) (*connect.Response[learningpathsv1.RecordItemProgressResponse], error) {
	user := getUser(ctx)
	if user == nil {
		return nil, connect.NewError(
			connect.CodeUnauthenticated,
			fmt.Errorf("user not authenticated"),
		)
	}

	itemID, err := uuid.Parse(req.Msg.ItemId)
	if err != nil {
		return nil, connect.NewError(
			connect.CodeInvalidArgument,
			fmt.Errorf("invalid item ID"),
		)
	}

	if err = h.app.services.LearningPaths.RecordItemProgress(
		ctx, user.ID, itemID, req.Msg.Completed,
	); err != nil {
		return nil, mapError(err)
	}

	return connect.NewResponse(&learningpathsv1.RecordItemProgressResponse{}), nil
}
