package main

import (
	"context"
	"errors"
	"slices"
	"time"

	"connectrpc.com/connect"
	"google.golang.org/protobuf/proto"

	observabilityv1 "tools.xdoubleu.com/gen/observability/v1"
	"tools.xdoubleu.com/internal/models"
)

// validTriggerSources/validOutcomes mirror global.automated_actions' CHECK
// constraints (cmd/api/migrations/00050_automated_actions.sql) — validated
// here too so a bad value surfaces as CodeInvalidArgument instead of a raw
// Postgres constraint-violation error.
//
//nolint:gochecknoglobals // fixed lookup tables, not mutated
var validTriggerSources = []string{"schedule", "api", "manual"}

//nolint:gochecknoglobals // fixed lookup table, not mutated
var validOutcomes = []string{"succeeded", "failed", "no_action_needed"}

var (
	errInvalidTriggerSource = errors.New(
		"trigger_source must be one of schedule, api, manual")
	errEmptyRoutineName = errors.New("routine_name is required")
	errInvalidOutcome   = errors.New(
		"outcome must be one of succeeded, failed, no_action_needed")
	errInvalidMode = errors.New(`mode must be "open" or "close"`)
)

func (h *obsConnectHandler) OpenAutomatedAction(
	ctx context.Context,
	req *connect.Request[observabilityv1.OpenAutomatedActionRequest],
) (*connect.Response[observabilityv1.OpenAutomatedActionResponse], error) {
	if err := requireAdmin(ctx); err != nil {
		return nil, err
	}
	resp, err := h.openAutomatedAction(
		ctx, req.Msg.GetTriggerSource(), req.Msg.GetRoutineName(),
	)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(resp), nil
}

// openAutomatedAction records that routineName has started, fired by
// triggerSource — the first step a self-healing routine takes, since it
// runs outside api's own process and nothing else would ever learn it ran
// (issue #1441).
func (h *obsConnectHandler) openAutomatedAction(
	ctx context.Context,
	triggerSource, routineName string,
) (*observabilityv1.OpenAutomatedActionResponse, error) {
	if !slices.Contains(validTriggerSources, triggerSource) {
		return nil, connect.NewError(connect.CodeInvalidArgument,
			errInvalidTriggerSource)
	}
	if routineName == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errEmptyRoutineName)
	}

	id, err := h.app.automatedActionsRepo.Open(ctx, triggerSource, routineName)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	return &observabilityv1.OpenAutomatedActionResponse{Id: id}, nil
}

func (h *obsConnectHandler) CloseAutomatedAction(
	ctx context.Context,
	req *connect.Request[observabilityv1.CloseAutomatedActionRequest],
) (*connect.Response[observabilityv1.CloseAutomatedActionResponse], error) {
	if err := requireAdmin(ctx); err != nil {
		return nil, err
	}
	resp, err := h.closeAutomatedAction(
		ctx,
		req.Msg.GetId(),
		req.Msg.GetOutcome(),
		req.Msg.GetPrUrl(),
		req.Msg.GetError(),
	)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(resp), nil
}

// closeAutomatedAction closes out the row id refers to — the last step a
// self-healing routine takes, whether it succeeded, failed, or found
// nothing to do.
func (h *obsConnectHandler) closeAutomatedAction(
	ctx context.Context,
	id int64,
	outcome, prURL, errorText string,
) (*observabilityv1.CloseAutomatedActionResponse, error) {
	if !slices.Contains(validOutcomes, outcome) {
		return nil, connect.NewError(connect.CodeInvalidArgument, errInvalidOutcome)
	}

	if err := h.app.automatedActionsRepo.Close(
		ctx, id, outcome, prURL, errorText,
	); err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	return &observabilityv1.CloseAutomatedActionResponse{}, nil
}

func (h *obsConnectHandler) GetAutomatedActions(
	ctx context.Context,
	req *connect.Request[observabilityv1.GetAutomatedActionsRequest],
) (*connect.Response[observabilityv1.GetAutomatedActionsResponse], error) {
	if err := requireAdmin(ctx); err != nil {
		return nil, err
	}
	resp, err := h.automatedActions(ctx, req.Msg.GetWindowDays())
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	return connect.NewResponse(resp), nil
}

// automatedActions runs the run-history query and builds the response. It
// is shared by the Connect handler above and the get_automated_actions MCP
// tool.
func (h *obsConnectHandler) automatedActions(
	ctx context.Context,
	windowDays int32,
) (*observabilityv1.GetAutomatedActionsResponse, error) {
	runs, err := h.app.automatedActionsRepo.ListRecent(
		ctx, windowSince(windowDays), recentRunsLimit,
	)
	if err != nil {
		return nil, err
	}

	protoRuns := make([]*observabilityv1.AutomatedAction, len(runs))
	for i, r := range runs {
		protoRuns[i] = protoAutomatedAction(&r)
	}

	return &observabilityv1.GetAutomatedActionsResponse{Actions: protoRuns}, nil
}

// recordAction backs the record_action MCP tool, dispatching to
// openAutomatedAction or closeAutomatedAction by a.Mode — the MCP surface
// exposes one tool for both operations (per issue #1441) even though the
// Connect service, like every other mutating pair here, keeps them as two
// separate RPCs.
func (h *obsConnectHandler) recordAction(
	ctx context.Context,
	a recordActionArgs,
) (proto.Message, error) {
	switch a.Mode {
	case "open":
		return h.openAutomatedAction(ctx, a.TriggerSource, a.RoutineName)
	case "close":
		return h.closeAutomatedAction(ctx, a.ID, a.Outcome, a.PRURL, a.Error)
	default:
		return nil, connect.NewError(connect.CodeInvalidArgument, errInvalidMode)
	}
}

func protoAutomatedAction(a *models.AutomatedAction) *observabilityv1.AutomatedAction {
	var finishedAt string
	if a.FinishedAt != nil {
		finishedAt = a.FinishedAt.Format(time.RFC3339)
	}

	return &observabilityv1.AutomatedAction{
		Id:            a.ID,
		FiredAt:       a.FiredAt.Format(time.RFC3339),
		TriggerSource: a.TriggerSource,
		RoutineName:   a.RoutineName,
		FinishedAt:    finishedAt,
		Outcome:       a.Outcome,
		PrUrl:         a.PRURL,
		Error:         a.Error,
	}
}
