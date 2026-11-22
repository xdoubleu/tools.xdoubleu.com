package connect

import (
	"context"
	"strings"

	factoryv1 "tools.xdoubleu.com/gen/factory/v1"
	"tools.xdoubleu.com/internal/platform/factory/internal/models"
	"tools.xdoubleu.com/internal/platform/factory/internal/services"
)

// ProtoRoutine is a routine with its newest job.
func ProtoRoutine(r *models.Routine) *factoryv1.Routine {
	out := &factoryv1.Routine{
		Name:            r.Name,
		RepoId:          r.RepoID.String(),
		Enabled:         r.Enabled,
		IntervalMinutes: int32(r.IntervalMinutes), //nolint:gosec // capped to a month
		LastJob:         nil,
	}
	if r.LastJob != nil {
		out.LastJob = ProtoJob(r.LastJob)
	}
	return out
}

// maxRoutineOutcome caps a job's outcome as the board shows it.
const maxRoutineOutcome = 2000

func truncateOutcome(s string) string {
	if len(s) <= maxRoutineOutcome {
		return s
	}
	return strings.ToValidUTF8(s[:maxRoutineOutcome], "") + "…"
}

func (h *taskConnectHandler) ListRoutines(
	ctx context.Context,
	_ *factoryv1.ListRoutinesRequest,
) (*factoryv1.ListRoutinesResponse, error) {
	if _, err := actorFrom(ctx); err != nil {
		return nil, err
	}
	list, err := h.app.Services.Board.ListRoutines(ctx)
	if err != nil {
		return nil, MapError(err)
	}
	out := make([]*factoryv1.Routine, len(list))
	for i := range list {
		out[i] = ProtoRoutine(&list[i])
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
	repoID, err := ParseID(req.GetRepoId())
	if err != nil {
		return nil, err
	}
	routine, err := h.app.Services.Board.SaveRoutine(ctx, actor, services.RoutineInput{
		Name: req.GetName(), RepoID: repoID, Enabled: req.GetEnabled(),
		IntervalMinutes: int(req.GetIntervalMinutes()),
	})
	if err != nil {
		return nil, MapError(err)
	}
	return &factoryv1.SaveRoutineResponse{Routine: ProtoRoutine(routine)}, nil
}

func (h *taskConnectHandler) RunRoutine(
	ctx context.Context,
	req *factoryv1.RunRoutineRequest,
) (*factoryv1.RunRoutineResponse, error) {
	actor, err := actorFrom(ctx)
	if err != nil {
		return nil, err
	}
	job, err := h.app.Services.Board.RunRoutine(ctx, actor, req.GetName())
	if err != nil {
		return nil, MapError(err)
	}
	return &factoryv1.RunRoutineResponse{Job: ProtoJob(job)}, nil
}

func (h *taskConnectHandler) DeleteRoutine(
	ctx context.Context,
	req *factoryv1.DeleteRoutineRequest,
) (*factoryv1.DeleteRoutineResponse, error) {
	actor, err := actorFrom(ctx)
	if err != nil {
		return nil, err
	}
	if err = h.app.Services.Board.DeleteRoutine(ctx, actor, req.GetName()); err != nil {
		return nil, MapError(err)
	}
	return &factoryv1.DeleteRoutineResponse{}, nil
}

func (h *taskConnectHandler) StopRoutine(
	ctx context.Context,
	req *factoryv1.StopRoutineRequest,
) (*factoryv1.StopRoutineResponse, error) {
	actor, err := actorFrom(ctx)
	if err != nil {
		return nil, err
	}
	job, err := h.app.Services.Jobs.StopRoutine(ctx, actor, req.GetName())
	if err != nil {
		return nil, MapError(err)
	}
	return &factoryv1.StopRoutineResponse{Job: ProtoJob(job)}, nil
}
