package main

import (
	"context"
	"errors"
	"testing"
	"time"

	"connectrpc.com/connect"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	observabilityv1 "tools.xdoubleu.com/gen/observability/v1"
	"tools.xdoubleu.com/gen/observability/v1/observabilityv1connect"
	"tools.xdoubleu.com/internal/models"
)

// stubStorageScanRunner lets TriggerStorageScan tests control the scan
// outcome without depending on a real R2 bucket being reachable.
type stubStorageScanRunner struct {
	err error
}

func (s *stubStorageScanRunner) RunStorageScanNow(_ context.Context) error {
	return s.err
}

// withStorageScanRunner swaps testApp's reading app for a stub for the
// duration of the test.
func withStorageScanRunner(t *testing.T, runner storageScanRunner) {
	t.Helper()
	orig := testApp.readingApp
	testApp.readingApp = runner
	t.Cleanup(func() { testApp.readingApp = orig })
}

func observabilityClient(
	t *testing.T,
) observabilityv1connect.ObservabilityServiceClient {
	t.Helper()
	ts := connectServer(t)
	return observabilityv1connect.NewObservabilityServiceClient(ts.Client(), ts.URL)
}

func TestObservabilityGetJobStats_AsAdmin(t *testing.T) {
	ctx := context.Background()
	promoteToAdmin(t)
	t.Cleanup(func() { demoteToUser(t) })

	// Seed a couple of job runs.
	require.NoError(t, testApp.jobRunsRepo.Insert(ctx, models.JobRun{
		JobID:      "steam",
		StartedAt:  time.Now(),
		DurationMs: 500,
		Success:    true,
		Error:      "",
	}))
	require.NoError(t, testApp.jobRunsRepo.Insert(ctx, models.JobRun{
		JobID:      "steam",
		StartedAt:  time.Now(),
		DurationMs: 700,
		Success:    false,
		Error:      "boom",
	}))

	client := observabilityClient(t)
	req := connect.NewRequest(&observabilityv1.GetJobStatsRequest{WindowDays: 7})
	setCookieOnRequest(req, accessToken)
	resp, err := client.GetJobStats(context.Background(), req)
	require.NoError(t, err)

	var steam *observabilityv1.JobStat
	for _, s := range resp.Msg.Stats {
		if s.JobId == "steam" {
			steam = s
		}
	}
	require.NotNil(t, steam)
	assert.GreaterOrEqual(t, steam.TotalRuns, int64(2))
	assert.GreaterOrEqual(t, steam.FailedRuns, int64(1))
	assert.NotEmpty(t, resp.Msg.RecentRuns)
}

func TestObservabilityGetJobStats_NonAdmin(t *testing.T) {
	demoteToUser(t)
	client := observabilityClient(t)
	req := connect.NewRequest(&observabilityv1.GetJobStatsRequest{WindowDays: 7})
	setCookieOnRequest(req, accessToken)
	_, err := client.GetJobStats(context.Background(), req)
	requirePermissionDenied(t, err)
}

func TestObservabilityGetUsageStats_AsAdmin(t *testing.T) {
	ctx := context.Background()
	promoteToAdmin(t)
	t.Cleanup(func() { demoteToUser(t) })

	require.NoError(t, testApp.usageRepo.Flush(ctx, []models.UsageEntry{
		{Day: time.Now(), App: "reading", Endpoint: "root", Count: 3},
	}))

	client := observabilityClient(t)
	req := connect.NewRequest(&observabilityv1.GetUsageStatsRequest{WindowDays: 7})
	setCookieOnRequest(req, accessToken)
	resp, err := client.GetUsageStats(context.Background(), req)
	require.NoError(t, err)
	assert.NotEmpty(t, resp.Msg.Entries)
}

func TestObservabilityGetUsageStats_NonAdmin(t *testing.T) {
	demoteToUser(t)
	client := observabilityClient(t)
	req := connect.NewRequest(&observabilityv1.GetUsageStatsRequest{WindowDays: 7})
	setCookieOnRequest(req, accessToken)
	_, err := client.GetUsageStats(context.Background(), req)
	requirePermissionDenied(t, err)
}

func TestObservabilityGetStorageStats_AsAdmin(t *testing.T) {
	ctx := context.Background()
	promoteToAdmin(t)
	t.Cleanup(func() { demoteToUser(t) })

	require.NoError(t, testApp.storageRepo.Insert(ctx, models.StorageSnapshot{
		ScannedAt:            time.Now(),
		TotalSizeBytes:       1234,
		ObjectCount:          5,
		OrphanSizeBytes:      100,
		OrphanCount:          1,
		StaleUploadSizeBytes: 0,
		StaleUploadCount:     0,
		PrefixBreakdown: []models.PrefixStat{
			{Prefix: "books", SizeBytes: 1234, Count: 5},
		},
	}))

	client := observabilityClient(t)
	req := connect.NewRequest(&observabilityv1.GetStorageStatsRequest{})
	setCookieOnRequest(req, accessToken)
	resp, err := client.GetStorageStats(context.Background(), req)
	require.NoError(t, err)
	require.NotNil(t, resp.Msg.Latest)
	assert.Equal(t, int64(1234), resp.Msg.Latest.TotalSizeBytes)
	assert.NotEmpty(t, resp.Msg.Latest.PrefixBreakdown)
}

func TestObservabilityTriggerStorageScan_NonAdmin(t *testing.T) {
	demoteToUser(t)
	client := observabilityClient(t)
	req := connect.NewRequest(&observabilityv1.TriggerStorageScanRequest{})
	setCookieOnRequest(req, accessToken)
	_, err := client.TriggerStorageScan(context.Background(), req)
	requirePermissionDenied(t, err)
}

func TestObservabilityTriggerStorageScan_AsAdmin(t *testing.T) {
	ctx := context.Background()
	promoteToAdmin(t)
	t.Cleanup(func() { demoteToUser(t) })
	withStorageScanRunner(t, &stubStorageScanRunner{err: nil})

	require.NoError(t, testApp.storageRepo.Insert(ctx, models.StorageSnapshot{
		ScannedAt:            time.Now(),
		TotalSizeBytes:       5678,
		ObjectCount:          9,
		OrphanSizeBytes:      0,
		OrphanCount:          0,
		StaleUploadSizeBytes: 0,
		StaleUploadCount:     0,
		PrefixBreakdown: []models.PrefixStat{
			{Prefix: "books", SizeBytes: 5678, Count: 9},
		},
	}))

	client := observabilityClient(t)
	req := connect.NewRequest(&observabilityv1.TriggerStorageScanRequest{})
	setCookieOnRequest(req, accessToken)
	resp, err := client.TriggerStorageScan(context.Background(), req)
	require.NoError(t, err)
	require.NotNil(t, resp.Msg.Latest)
	assert.Equal(t, int64(5678), resp.Msg.Latest.TotalSizeBytes)
	assert.NotEmpty(t, resp.Msg.Latest.PrefixBreakdown)
}

// TestObservabilityTriggerStorageScan_AsAdmin_ScanFails covers error
// propagation: when the underlying scan fails, the RPC must surface that as
// CodeInternal rather than silently returning a stale/empty snapshot.
func TestObservabilityTriggerStorageScan_AsAdmin_ScanFails(t *testing.T) {
	promoteToAdmin(t)
	t.Cleanup(func() { demoteToUser(t) })
	withStorageScanRunner(t, &stubStorageScanRunner{err: errors.New("boom")})

	client := observabilityClient(t)
	req := connect.NewRequest(&observabilityv1.TriggerStorageScanRequest{})
	setCookieOnRequest(req, accessToken)
	_, err := client.TriggerStorageScan(context.Background(), req)
	require.Error(t, err)
	var connectErr *connect.Error
	require.ErrorAs(t, err, &connectErr)
	assert.Equal(t, connect.CodeInternal, connectErr.Code())
}

func TestObservabilityGetDatabaseStats_AsAdmin(t *testing.T) {
	promoteToAdmin(t)
	t.Cleanup(func() { demoteToUser(t) })

	client := observabilityClient(t)
	req := connect.NewRequest(&observabilityv1.GetDatabaseStatsRequest{})
	setCookieOnRequest(req, accessToken)
	resp, err := client.GetDatabaseStats(context.Background(), req)
	require.NoError(t, err)
	assert.Positive(t, resp.Msg.TotalSizeBytes)

	// The global schema always exists in the test DB.
	var hasGlobal bool
	for _, s := range resp.Msg.Schemas {
		if s.Name == "global" {
			hasGlobal = true
		}
	}
	assert.True(t, hasGlobal)
}

func TestObservabilityGetDatabaseStats_NonAdmin(t *testing.T) {
	demoteToUser(t)
	client := observabilityClient(t)
	req := connect.NewRequest(&observabilityv1.GetDatabaseStatsRequest{})
	setCookieOnRequest(req, accessToken)
	_, err := client.GetDatabaseStats(context.Background(), req)
	requirePermissionDenied(t, err)
}

func requirePermissionDenied(t *testing.T, err error) {
	t.Helper()
	require.Error(t, err)
	var connectErr *connect.Error
	require.ErrorAs(t, err, &connectErr)
	assert.Equal(t, connect.CodePermissionDenied, connectErr.Code())
}
