package main

import (
	"context"
	"encoding/json"
	"strconv"
	"testing"
	"time"

	"connectrpc.com/connect"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"

	observabilityv1 "tools.xdoubleu.com/gen/observability/v1"
	"tools.xdoubleu.com/internal/models"
)

func TestObservabilityOpenAutomatedAction_AsAdmin(t *testing.T) {
	promoteToAdmin(t)
	t.Cleanup(func() { demoteToUser(t) })

	client := observabilityClient(t)
	req := connect.NewRequest(&observabilityv1.OpenAutomatedActionRequest{
		TriggerSource: "schedule",
		RoutineName:   "dependabot-triage",
	})
	setCookieOnRequest(req, accessToken)
	resp, err := client.OpenAutomatedAction(context.Background(), req)
	require.NoError(t, err)
	assert.Positive(t, resp.Msg.Id)
}

func TestObservabilityOpenAutomatedAction_InvalidTriggerSource(t *testing.T) {
	promoteToAdmin(t)
	t.Cleanup(func() { demoteToUser(t) })

	client := observabilityClient(t)
	req := connect.NewRequest(&observabilityv1.OpenAutomatedActionRequest{
		TriggerSource: "cron",
		RoutineName:   "dependabot-triage",
	})
	setCookieOnRequest(req, accessToken)
	_, err := client.OpenAutomatedAction(context.Background(), req)
	require.Error(t, err)
	assert.Equal(t, connect.CodeInvalidArgument, connect.CodeOf(err))
}

func TestObservabilityOpenAutomatedAction_EmptyRoutineName(t *testing.T) {
	promoteToAdmin(t)
	t.Cleanup(func() { demoteToUser(t) })

	client := observabilityClient(t)
	req := connect.NewRequest(&observabilityv1.OpenAutomatedActionRequest{
		TriggerSource: "manual",
		RoutineName:   "",
	})
	setCookieOnRequest(req, accessToken)
	_, err := client.OpenAutomatedAction(context.Background(), req)
	require.Error(t, err)
	assert.Equal(t, connect.CodeInvalidArgument, connect.CodeOf(err))
}

func TestObservabilityOpenAutomatedAction_NonAdmin(t *testing.T) {
	demoteToUser(t)
	client := observabilityClient(t)
	req := connect.NewRequest(&observabilityv1.OpenAutomatedActionRequest{
		TriggerSource: "manual",
		RoutineName:   "x",
	})
	setCookieOnRequest(req, accessToken)
	_, err := client.OpenAutomatedAction(context.Background(), req)
	requirePermissionDenied(t, err)
}

func TestObservabilityCloseAutomatedAction_AsAdmin(t *testing.T) {
	promoteToAdmin(t)
	t.Cleanup(func() { demoteToUser(t) })

	client := observabilityClient(t)
	openReq := connect.NewRequest(&observabilityv1.OpenAutomatedActionRequest{
		TriggerSource: "api",
		RoutineName:   "storage-cleanup",
	})
	setCookieOnRequest(openReq, accessToken)
	openResp, err := client.OpenAutomatedAction(context.Background(), openReq)
	require.NoError(t, err)

	closeReq := connect.NewRequest(&observabilityv1.CloseAutomatedActionRequest{
		Id:      openResp.Msg.Id,
		Outcome: "succeeded",
		PrUrl:   "https://github.com/o/r/pull/9",
	})
	setCookieOnRequest(closeReq, accessToken)
	_, err = client.CloseAutomatedAction(context.Background(), closeReq)
	require.NoError(t, err)

	listReq := connect.NewRequest(&observabilityv1.GetAutomatedActionsRequest{
		WindowDays: 1,
	})
	setCookieOnRequest(listReq, accessToken)
	listResp, err := client.GetAutomatedActions(context.Background(), listReq)
	require.NoError(t, err)

	var found *observabilityv1.AutomatedAction
	for _, a := range listResp.Msg.Actions {
		if a.Id == openResp.Msg.Id {
			found = a
		}
	}
	require.NotNil(t, found)
	assert.Equal(t, "succeeded", found.Outcome)
	assert.Equal(t, "https://github.com/o/r/pull/9", found.PrUrl)
	assert.NotEmpty(t, found.FinishedAt)
}

func TestObservabilityCloseAutomatedAction_WithMetrics(t *testing.T) {
	promoteToAdmin(t)
	t.Cleanup(func() { demoteToUser(t) })

	client := observabilityClient(t)
	openReq := connect.NewRequest(&observabilityv1.OpenAutomatedActionRequest{
		TriggerSource: "ci",
		RoutineName:   "red-pr-repair",
	})
	setCookieOnRequest(openReq, accessToken)
	openResp, err := client.OpenAutomatedAction(context.Background(), openReq)
	require.NoError(t, err)

	metrics := &observabilityv1.RunMetrics{
		Requests:          14,
		InputTokens:       52000,
		OutputTokens:      3100,
		ReasoningTokens:   900,
		CacheReadTokens:   40000,
		CostUsd:           0.021,
		DurationSeconds:   95,
		ToolCalls:         22,
		ToolErrors:        2,
		RepeatedToolCalls: 1,
	}
	closeReq := connect.NewRequest(&observabilityv1.CloseAutomatedActionRequest{
		Id:      openResp.Msg.Id,
		Outcome: "no_action_needed",
		Metrics: metrics,
	})
	setCookieOnRequest(closeReq, accessToken)
	_, err = client.CloseAutomatedAction(context.Background(), closeReq)
	require.NoError(t, err)

	listReq := connect.NewRequest(&observabilityv1.GetAutomatedActionsRequest{
		WindowDays: 1,
	})
	setCookieOnRequest(listReq, accessToken)
	listResp, err := client.GetAutomatedActions(context.Background(), listReq)
	require.NoError(t, err)

	var found *observabilityv1.AutomatedAction
	for _, a := range listResp.Msg.Actions {
		if a.Id == openResp.Msg.Id {
			found = a
		}
	}
	require.NotNil(t, found)
	assert.Equal(t, "ci", found.TriggerSource)
	assert.True(t, proto.Equal(metrics, found.Metrics))
}

func TestObservabilityCloseAutomatedAction_NegativeMetrics(t *testing.T) {
	promoteToAdmin(t)
	t.Cleanup(func() { demoteToUser(t) })

	client := observabilityClient(t)
	req := connect.NewRequest(&observabilityv1.CloseAutomatedActionRequest{
		Id:      1,
		Outcome: "succeeded",
		Metrics: &observabilityv1.RunMetrics{Requests: 3, ToolErrors: -1},
	})
	setCookieOnRequest(req, accessToken)
	_, err := client.CloseAutomatedAction(context.Background(), req)
	require.Error(t, err)
	assert.Equal(t, connect.CodeInvalidArgument, connect.CodeOf(err))
}

func TestObservabilityCloseAutomatedAction_InvalidOutcome(t *testing.T) {
	promoteToAdmin(t)
	t.Cleanup(func() { demoteToUser(t) })

	client := observabilityClient(t)
	req := connect.NewRequest(&observabilityv1.CloseAutomatedActionRequest{
		Id:      1,
		Outcome: "done",
	})
	setCookieOnRequest(req, accessToken)
	_, err := client.CloseAutomatedAction(context.Background(), req)
	require.Error(t, err)
	assert.Equal(t, connect.CodeInvalidArgument, connect.CodeOf(err))
}

func TestObservabilityCloseAutomatedAction_NonAdmin(t *testing.T) {
	demoteToUser(t)
	client := observabilityClient(t)
	req := connect.NewRequest(&observabilityv1.CloseAutomatedActionRequest{
		Id:      1,
		Outcome: "succeeded",
	})
	setCookieOnRequest(req, accessToken)
	_, err := client.CloseAutomatedAction(context.Background(), req)
	requirePermissionDenied(t, err)
}

func TestObservabilityGetAutomatedActions_NonAdmin(t *testing.T) {
	demoteToUser(t)
	client := observabilityClient(t)
	req := connect.NewRequest(&observabilityv1.GetAutomatedActionsRequest{
		WindowDays: 7,
	})
	setCookieOnRequest(req, accessToken)
	_, err := client.GetAutomatedActions(context.Background(), req)
	requirePermissionDenied(t, err)
}

// TestAppsMCPRecordActionOpenAndClose: open then close by the returned id.
func TestAppsMCPRecordActionOpenAndClose(t *testing.T) {
	promoteToAdmin(t)
	t.Cleanup(func() { demoteToUser(t) })

	session := appsMCPSession(t, accessToken.Value)

	//nolint:exhaustruct // name + arguments are all a tool call needs
	openRes, err := session.CallTool(context.Background(), &mcp.CallToolParams{
		Name: "record_action",
		Arguments: map[string]any{
			"mode":           "open",
			"trigger_source": "schedule",
			"routine_name":   "mcp-test-routine",
		},
	})
	require.NoError(t, err)
	require.False(t, openRes.IsError, toolMessage(openRes, err))
	require.Len(t, openRes.Content, 1)
	openText, ok := openRes.Content[0].(*mcp.TextContent)
	require.True(t, ok)
	assert.Contains(t, openText.Text, `"id"`)

	var opened struct {
		ID string `json:"id"`
	}
	require.NoError(t, json.Unmarshal([]byte(openText.Text), &opened))
	require.NotEmpty(t, opened.ID)
	openedID, err := strconv.ParseInt(opened.ID, 10, 64)
	require.NoError(t, err)

	//nolint:exhaustruct // name + arguments are all a tool call needs
	closeRes, err := session.CallTool(context.Background(), &mcp.CallToolParams{
		Name: "record_action",
		Arguments: map[string]any{
			"mode":    "close",
			"id":      openedID,
			"outcome": "no_action_needed",
			"metrics": map[string]any{
				"requests":            5,
				"input_tokens":        1200,
				"output_tokens":       80,
				"reasoning_tokens":    0,
				"cache_read_tokens":   0,
				"cost_usd":            0.001,
				"duration_seconds":    12.5,
				"tool_calls":          3,
				"tool_errors":         0,
				"repeated_tool_calls": 0,
			},
		},
	})
	require.NoError(t, err)
	assert.False(t, closeRes.IsError, toolMessage(closeRes, err))

	runs, err := testApp.automatedActionsRepo.ListRecent(
		context.Background(), time.Now().Add(-time.Hour), 50,
	)
	require.NoError(t, err)
	var closed *models.AutomatedAction
	for i := range runs {
		if runs[i].ID == openedID {
			closed = &runs[i]
		}
	}
	require.NotNil(t, closed)
	require.NotNil(t, closed.Metrics)
	assert.Equal(t, int32(5), closed.Metrics.Requests)
	assert.InDelta(t, 12.5, closed.Metrics.DurationSeconds, 0.001)
}

func TestAppsMCPRecordActionInvalidMode(t *testing.T) {
	promoteToAdmin(t)
	t.Cleanup(func() { demoteToUser(t) })

	session := appsMCPSession(t, accessToken.Value)
	//nolint:exhaustruct // name + arguments are all a tool call needs
	res, err := session.CallTool(context.Background(), &mcp.CallToolParams{
		Name: "record_action",
		Arguments: map[string]any{
			"mode": "delete",
		},
	})
	assert.Contains(t, toolMessage(res, err), "mode")
}
