package obs

import (
	"context"
	"errors"
	"slices"
	"time"

	"connectrpc.com/connect/v2"
	"google.golang.org/protobuf/proto"

	observabilityv1 "tools.xdoubleu.com/gen/observability/v1"
	"tools.xdoubleu.com/internal/models")

// validTriggerSources/validOutcomes mirror global.automated_actions' CHECK
// constraints, so bad values return CodeInvalidArgument.
//
//nolint:gochecknoglobals // fixed lookup tables, not mutated
var validTriggerSources = []string{"schedule", "api", "manual", "ci"}

//nolint:gochecknoglobals // fixed lookup table, not mutated
var validOutcomes = []string{"succeeded", "failed", "no_action_needed"}

var (
	errInvalidTriggerSource = errors.New(
		"trigger_source must be one of schedule, api, manual, ci")
	errEmptyRoutineName = errors.New("routine_name is required")
	errInvalidOutcome   = errors.New(
		"outcome must be one of succeeded, failed, no_action_needed")
	errInvalidMode     = errors.New(`mode must be "open" or "close"`)
	errNegativeMetrics = errors.New("metrics must not be negative")
)

func (h *obsConnectHandler) OpenAutomatedAction(
	ctx context.Context,
	req *observabilityv1.OpenAutomatedActionRequest,
) (*observabilityv1.OpenAutomatedActionResponse, error) {
	if err := RequireAdmin(ctx); err != nil {
		return nil, err
	}
	resp, err := h.openAutomatedAction(
		ctx, req.GetTriggerSource(), req.GetRoutineName(),
	)
	if err != nil {
		return nil, err
	}
	return resp, nil
}

// openAutomatedAction records that a routine started. Routines run outside
// this process, so this is the only record they ran.
func (h *obsConnectHandler) openAutomatedAction(
	ctx context.Context,
	triggerSource, routineName string,
) (*observabilityv1.OpenAutomatedActionResponse, error) {
	if !slices.Contains(validTriggerSources, triggerSource) {
		return nil, connect.NewError(connect.CodeInvalidArgument,
			errInvalidTriggerSource.Error()).WithCause(errInvalidTriggerSource)
	}
	if routineName == "" {
		return nil, connect.NewError(
			connect.CodeInvalidArgument,
			errEmptyRoutineName.Error(),
		).WithCause(errEmptyRoutineName)
	}

	id, err := h.deps.AutomatedActionsRepo.Open(ctx, triggerSource, routineName)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err.Error()).WithCause(err)
	}

	return &observabilityv1.OpenAutomatedActionResponse{Id: id}, nil
}

func (h *obsConnectHandler) CloseAutomatedAction(
	ctx context.Context,
	req *observabilityv1.CloseAutomatedActionRequest,
) (*observabilityv1.CloseAutomatedActionResponse, error) {
	if err := RequireAdmin(ctx); err != nil {
		return nil, err
	}
	resp, err := h.closeAutomatedAction(
		ctx,
		req.GetId(),
		req.GetOutcome(),
		req.GetPrUrl(),
		req.GetError(),
		req.GetMetrics(),
	)
	if err != nil {
		return nil, err
	}
	return resp, nil
}

// closeAutomatedAction closes the row with the routine's outcome and, when
// its workflow measured them, the run's metrics.
func (h *obsConnectHandler) closeAutomatedAction(
	ctx context.Context,
	id int64,
	outcome, prURL, errorText string,
	metrics *observabilityv1.RunMetrics,
) (*observabilityv1.CloseAutomatedActionResponse, error) {
	if !slices.Contains(validOutcomes, outcome) {
		return nil, connect.NewError(connect.CodeInvalidArgument, errInvalidOutcome.Error()).
			WithCause(errInvalidOutcome)
	}
	m, err := runMetricsModel(metrics)
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err.Error()).WithCause(err)
	}

	if err = h.deps.AutomatedActionsRepo.Close(
		ctx, id, outcome, prURL, errorText, m,
	); err != nil {
		return nil, connect.NewError(connect.CodeInternal, err.Error()).WithCause(err)
	}

	return &observabilityv1.CloseAutomatedActionResponse{}, nil
}

func (h *obsConnectHandler) GetAutomatedActions(
	ctx context.Context,
	req *observabilityv1.GetAutomatedActionsRequest,
) (*observabilityv1.GetAutomatedActionsResponse, error) {
	if err := RequireAdmin(ctx); err != nil {
		return nil, err
	}
	resp, err := h.automatedActions(ctx, req.GetWindowDays())
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err.Error()).WithCause(err)
	}
	return resp, nil
}

// automatedActions builds the run-history response for Connect and MCP.
func (h *obsConnectHandler) automatedActions(
	ctx context.Context,
	windowDays int32,
) (*observabilityv1.GetAutomatedActionsResponse, error) {
	runs, err := h.deps.AutomatedActionsRepo.ListRecent(
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

// recordAction backs the record_action MCP tool, dispatching to open or close
// by a.Mode.
func (h *obsConnectHandler) recordAction(
	ctx context.Context,
	a RecordActionArgs,
) (proto.Message, error) {
	switch a.Mode {
	case "open":
		return h.openAutomatedAction(ctx, a.TriggerSource, a.RoutineName)
	case "close":
		return h.closeAutomatedAction(
			ctx, a.ID, a.Outcome, a.PRURL, a.Error, a.Metrics.proto(),
		)
	default:
		return nil, connect.NewError(connect.CodeInvalidArgument, errInvalidMode.Error()).
			WithCause(errInvalidMode)
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
		Metrics:       protoRunMetrics(a.Metrics),
	}
}

// runMetricsModel converts the request's metrics; nil stays nil.
func runMetricsModel(m *observabilityv1.RunMetrics) (*models.RunMetrics, error) {
	if m == nil {
		return nil, nil //nolint:nilnil // no metrics is valid
	}
	if min(
		int64(m.GetRequests()), m.GetInputTokens(), m.GetOutputTokens(),
		m.GetReasoningTokens(), m.GetCacheReadTokens(),
		int64(m.GetToolCalls()), int64(m.GetToolErrors()),
		int64(m.GetRepeatedToolCalls()),
	) < 0 || m.GetCostUsd() < 0 || m.GetDurationSeconds() < 0 {
		return nil, errNegativeMetrics
	}
	return &models.RunMetrics{
		Requests:          m.GetRequests(),
		InputTokens:       m.GetInputTokens(),
		OutputTokens:      m.GetOutputTokens(),
		ReasoningTokens:   m.GetReasoningTokens(),
		CacheReadTokens:   m.GetCacheReadTokens(),
		CostUSD:           m.GetCostUsd(),
		DurationSeconds:   m.GetDurationSeconds(),
		ToolCalls:         m.GetToolCalls(),
		ToolErrors:        m.GetToolErrors(),
		RepeatedToolCalls: m.GetRepeatedToolCalls(),
	}, nil
}

func protoRunMetrics(m *models.RunMetrics) *observabilityv1.RunMetrics {
	if m == nil {
		return nil
	}
	return &observabilityv1.RunMetrics{
		Requests:          m.Requests,
		InputTokens:       m.InputTokens,
		OutputTokens:      m.OutputTokens,
		ReasoningTokens:   m.ReasoningTokens,
		CacheReadTokens:   m.CacheReadTokens,
		CostUsd:           m.CostUSD,
		DurationSeconds:   m.DurationSeconds,
		ToolCalls:         m.ToolCalls,
		ToolErrors:        m.ToolErrors,
		RepeatedToolCalls: m.RepeatedToolCalls,
	}
}
