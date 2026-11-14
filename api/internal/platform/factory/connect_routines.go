package factory

import (
	"context"

	factoryv1 "tools.xdoubleu.com/gen/factory/v1"
	"tools.xdoubleu.com/internal/platform/factory/internal/models"
	"tools.xdoubleu.com/internal/platform/factory/internal/services"
)

func protoRoutine(r *models.Routine) *factoryv1.Routine {
	out := &factoryv1.Routine{
		Name:            r.Name,
		RepoId:          r.RepoID.String(),
		Enabled:         r.Enabled,
		IntervalMinutes: int32(r.IntervalMinutes), //nolint:gosec // capped to a month
		LastJob:         nil,
	}
	if r.LastJob != nil {
		out.LastJob = protoJob(r.LastJob)
	}
	return out
}

func (h *taskConnectHandler) ListRoutines(
	ctx context.Context,
	_ *factoryv1.ListRoutinesRequest,
) (*factoryv1.ListRoutinesResponse, error) {
	if _, err := actorFrom(ctx); err != nil {
		return nil, err
	}
	list, err := h.app.services.Jobs.ListRoutines(ctx)
	if err != nil {
		return nil, mapError(err)
	}
	out := make([]*factoryv1.Routine, len(list))
	for i := range list {
		out[i] = protoRoutine(&list[i])
	}
	return &factoryv1.ListRoutinesResponse{Routines: out}, nil
}

func (h *taskConnectHandler) SaveRoutine(
	ctx context.Context,
	req *factoryv1.SaveRoutineRequest,
) (*factoryv1.SaveRoutineResponse, error) {
	actor, err := actorFrom(ctx)
	if err != nil {
		return nil, err
	}
	repoID, err := parseID(req.GetRepoId())
	if err != nil {
		return nil, err
	}
	routine, err := h.app.services.Jobs.SaveRoutine(ctx, actor, services.RoutineInput{
		Name: req.GetName(), RepoID: repoID, Enabled: req.GetEnabled(),
		IntervalMinutes: int(req.GetIntervalMinutes()),
	})
	if err != nil {
		return nil, mapError(err)
	}
	return &factoryv1.SaveRoutineResponse{Routine: protoRoutine(routine)}, nil
}

func (h *taskConnectHandler) RunRoutine(
	ctx context.Context,
	req *factoryv1.RunRoutineRequest,
) (*factoryv1.RunRoutineResponse, error) {
	actor, err := actorFrom(ctx)
	if err != nil {
		return nil, err
	}
	job, err := h.app.services.Jobs.RunRoutine(ctx, actor, req.GetName())
	if err != nil {
		return nil, mapError(err)
	}
	return &factoryv1.RunRoutineResponse{Job: protoJob(job)}, nil
}

func (h *taskConnectHandler) StopRoutine(
	ctx context.Context,
	req *factoryv1.StopRoutineRequest,
) (*factoryv1.StopRoutineResponse, error) {
	actor, err := actorFrom(ctx)
	if err != nil {
		return nil, err
	}
	job, err := h.app.services.Jobs.StopRoutine(ctx, actor, req.GetName())
	if err != nil {
		return nil, mapError(err)
	}
	return &factoryv1.StopRoutineResponse{Job: protoJob(job)}, nil
}
