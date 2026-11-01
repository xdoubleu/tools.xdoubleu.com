package main

import (
	"context"
	"testing"
	"time"

	"connectrpc.com/connect/v2"
	"connectrpc.com/connect/v2/connecthttp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	factoryv1 "tools.xdoubleu.com/gen/factory/v1"
	"tools.xdoubleu.com/gen/factory/v1/factoryv1connect"
	"tools.xdoubleu.com/internal/models"
)

func runnerClients(t *testing.T) (
	factoryv1connect.RunnerAdminServiceClient,
	factoryv1connect.RunnerServiceClient,
) {
	t.Helper()
	ts := connectServer(t)
	transport := connecthttp.NewTransport(
		ts.Client(), ts.URL, connecthttp.WithReadMaxBytes(0),
	)
	return factoryv1connect.NewRunnerAdminServiceClient(connect.NewClient(transport)),
		factoryv1connect.NewRunnerServiceClient(connect.NewClient(transport))
}

func resetRunners(t *testing.T) {
	t.Helper()
	_, err := testApp.db.Exec(context.Background(), "DELETE FROM factory.runners")
	require.NoError(t, err)
}

// TestFactoryRunner_PairsThroughTheRealStack uses a real TOTP-verified admin
// session for the step-up and the real router, so a recovery code is what
// authorises the pairing.
func TestFactoryRunner_PairsThroughTheRealStack(t *testing.T) {
	resetRunners(t)
	t.Cleanup(func() { resetRunners(t) })
	admin := newAgentAdmin(t, models.RoleAdmin)
	adminClient, runnerClient := runnerClients(t)

	_, err := adminClient.CreateRunner(admin.ctx(),
		&factoryv1.CreateRunnerRequest{Name: "Home Mac", TotpCode: "000000"})
	assert.Equal(t, connect.CodePermissionDenied, connect.CodeOf(err), "wrong code")

	created, err := adminClient.CreateRunner(admin.ctx(),
		&factoryv1.CreateRunnerRequest{Name: "Home Mac", TotpCode: admin.nextCode()})
	require.NoError(t, err)

	// The runner has no session: it pairs with the code alone.
	paired, err := runnerClient.Pair(context.Background(), &factoryv1.PairRequest{
		Code: created.PairingCode, Version: "0.1.0",
	})
	require.NoError(t, err)

	// A heartbeat is held for 20 seconds; the runner hanging up early still
	// counts as a sign of life.
	ctx, info := connect.NewClientContext(context.Background())
	info.RequestHeader().Set("Authorization", "Bearer "+paired.DeviceToken)
	ctx, cancel := context.WithTimeout(ctx, 500*time.Millisecond)
	defer cancel()
	_, _ = runnerClient.Heartbeat(ctx, &factoryv1.HeartbeatRequest{Version: "0.1.0"})

	listed, err := adminClient.ListRunners(admin.ctx(), &factoryv1.ListRunnersRequest{})
	require.NoError(t, err)
	require.Len(t, listed.Runners, 1)
	assert.True(t, listed.Runners[0].Online)
	assert.Equal(t, "0.1.0", listed.Runners[0].Version)

	_, err = adminClient.RevokeRunner(admin.ctx(),
		&factoryv1.RevokeRunnerRequest{Id: created.Runner.Id})
	require.NoError(t, err)
	listed, err = adminClient.ListRunners(admin.ctx(), &factoryv1.ListRunnersRequest{})
	require.NoError(t, err)
	assert.False(t, listed.Runners[0].Online)
}
