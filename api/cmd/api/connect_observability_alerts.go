package main

import (
	"context"
	"time"

	"connectrpc.com/connect"

	observabilityv1 "tools.xdoubleu.com/gen/observability/v1"
)

// GetAlertStates surfaces jobs.ThresholdAlertJob's per-rule breach/recovery
// state (issue #1283) — an alert whose state can't be inspected produces
// exactly the "why didn't I get emailed" question issue #1214's
// get_notification_settings existed to fix.

func (h *obsConnectHandler) GetAlertStates(
	ctx context.Context,
	_ *connect.Request[observabilityv1.GetAlertStatesRequest],
) (*connect.Response[observabilityv1.GetAlertStatesResponse], error) {
	if err := requireAdmin(ctx); err != nil {
		return nil, err
	}

	resp, err := h.alertStates(ctx)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	return connect.NewResponse(resp), nil
}

// alertStates is shared by the Connect handler above and the
// get_alert_states MCP tool.
func (h *obsConnectHandler) alertStates(
	ctx context.Context,
) (*observabilityv1.GetAlertStatesResponse, error) {
	states, err := h.app.alertStatesRepo.List(ctx)
	if err != nil {
		return nil, err
	}

	protoStates := make([]*observabilityv1.AlertState, len(states))
	for i, s := range states {
		protoStates[i] = &observabilityv1.AlertState{
			RuleKey:        s.RuleKey,
			Breaching:      s.Breaching,
			Since:          formatAlertTime(s.Since),
			LastNotifiedAt: formatAlertTime(s.LastNotifiedAt),
			CurrentValue:   s.CurrentValue,
			Threshold:      s.Threshold,
		}
	}

	return &observabilityv1.GetAlertStatesResponse{States: protoStates}, nil
}

func formatAlertTime(t *time.Time) string {
	if t == nil {
		return ""
	}
	return t.Format(time.RFC3339)
}
