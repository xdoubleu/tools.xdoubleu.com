package main

import (
	"context"
	"testing"
	"time"

	"connectrpc.com/connect"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	observabilityv1 "tools.xdoubleu.com/gen/observability/v1"
	"tools.xdoubleu.com/internal/models"
)

func TestObservabilityGetAlertStates_AsAdmin(t *testing.T) {
	promoteToAdmin(t)
	t.Cleanup(func() { demoteToUser(t) })

	since := time.Now().Add(-time.Hour)
	notifiedAt := time.Now()
	require.NoError(
		t,
		testApp.alertStatesRepo.Upsert(context.Background(), models.AlertState{
			RuleKey: "host_cpu_high", Breaching: true, Since: &since,
			LastNotifiedAt: &notifiedAt, CurrentValue: 92.5, Threshold: 80,
		}),
	)
	t.Cleanup(func() {
		_, _ = testApp.db.Exec(
			context.Background(),
			"DELETE FROM global.alert_states WHERE rule_key = 'host_cpu_high'",
		)
	})

	client := observabilityClient(t)
	req := connect.NewRequest(&observabilityv1.GetAlertStatesRequest{})
	setCookieOnRequest(req, accessToken)
	resp, err := client.GetAlertStates(context.Background(), req)
	require.NoError(t, err)

	var found *observabilityv1.AlertState
	for _, s := range resp.Msg.States {
		if s.RuleKey == "host_cpu_high" {
			found = s
		}
	}
	require.NotNil(t, found)
	assert.True(t, found.Breaching)
	assert.NotEmpty(t, found.Since)
	assert.NotEmpty(t, found.LastNotifiedAt)
	assert.InDelta(t, 92.5, found.CurrentValue, 0.001)
	assert.InDelta(t, 80, found.Threshold, 0.001)
}

func TestObservabilityGetAlertStates_NonAdmin(t *testing.T) {
	demoteToUser(t)
	client := observabilityClient(t)
	req := connect.NewRequest(&observabilityv1.GetAlertStatesRequest{})
	setCookieOnRequest(req, accessToken)
	_, err := client.GetAlertStates(context.Background(), req)
	requirePermissionDenied(t, err)
}
