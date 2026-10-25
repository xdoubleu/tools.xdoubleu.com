package main

import (
	"context"
	"testing"
	"time"

	"connectrpc.com/connect/v2"
	"connectrpc.com/connect/v2/connecthttp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	observabilityv1 "tools.xdoubleu.com/gen/observability/v1"
	"tools.xdoubleu.com/gen/observability/v1/observabilityv1connect"
	"tools.xdoubleu.com/internal/models"
)

// stubStorageScanRunner records scan starts without R2.
type stubStorageScanRunner struct {
	started int
}

func (s *stubStorageScanRunner) StartStorageScan() {
	s.started++
}

// withStorageScanRunner stubs testApp's books app for the test.
func withStorageScanRunner(t *testing.T, runner storageScanRunner) {
	t.Helper()
	orig := testApp.booksApp
	testApp.booksApp = runner
	t.Cleanup(func() { testApp.booksApp = orig })
}

func observabilityClient(
	t *testing.T,
) observabilityv1connect.ObservabilityServiceClient {
	t.Helper()
	ts := connectServer(t)
	return observabilityv1connect.NewObservabilityServiceClient(
		connect.NewClient(
			connecthttp.NewTransport(ts.Client(), ts.URL, connecthttp.WithReadMaxBytes(0)),
		),
	)
}

func TestObservabilityGetJobStats_AsAdmin(t *testing.T) {
	ctx := context.Background()
	promoteToAdmin(t)
	t.Cleanup(func() { demoteToUser(t) })

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
	req := &observabilityv1.GetJobStatsRequest{WindowDays: 7}
	resp, err := client.GetJobStats(withCookies(context.Background(), accessToken), req)
	require.NoError(t, err)

	var steam *observabilityv1.JobStat
	for _, s := range resp.Stats {
		if s.JobId == "steam" {
			steam = s
		}
	}
	require.NotNil(t, steam)
	assert.GreaterOrEqual(t, steam.TotalRuns, int64(2))
	assert.GreaterOrEqual(t, steam.FailedRuns, int64(1))
	assert.NotEmpty(t, resp.RecentRuns)
}

func TestObservabilityGetJobStats_NonAdmin(t *testing.T) {
	demoteToUser(t)
	client := observabilityClient(t)
	req := &observabilityv1.GetJobStatsRequest{WindowDays: 7}
	_, err := client.GetJobStats(withCookies(context.Background(), accessToken), req)
	requirePermissionDenied(t, err)
}

func TestObservabilityGetUsageStats_AsAdmin(t *testing.T) {
	ctx := context.Background()
	promoteToAdmin(t)
	t.Cleanup(func() { demoteToUser(t) })

	require.NoError(t, testApp.usageRepo.Flush(ctx, []models.UsageEntry{
		{Day: time.Now(), App: "books", Endpoint: "root", Count: 3, Bytes: 300},
	}))

	client := observabilityClient(t)
	req := &observabilityv1.GetUsageStatsRequest{WindowDays: 7}
	resp, err := client.GetUsageStats(withCookies(context.Background(), accessToken), req)
	require.NoError(t, err)
	assert.NotEmpty(t, resp.Entries)
	assert.NotContains(t, resp.UnusedApps, "books")
	assert.Contains(t, resp.UnusedApps, "watchparty")
}

func TestObservabilityGetUsageStats_NonAdmin(t *testing.T) {
	demoteToUser(t)
	client := observabilityClient(t)
	req := &observabilityv1.GetUsageStatsRequest{WindowDays: 7}
	_, err := client.GetUsageStats(withCookies(context.Background(), accessToken), req)
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
		OrphanKeys:             []string{"books/b1/orphan.epub"},
		DeletedOrphanSizeBytes: 50,
		DeletedOrphanCount:     1,
	}))

	client := observabilityClient(t)
	req := &observabilityv1.GetStorageStatsRequest{}
	resp, err := client.GetStorageStats(
		withCookies(context.Background(), accessToken),
		req,
	)
	require.NoError(t, err)
	require.NotNil(t, resp.Latest)
	assert.Equal(t, int64(1234), resp.Latest.TotalSizeBytes)
	assert.NotEmpty(t, resp.Latest.PrefixBreakdown)
	assert.Equal(t, []string{"books/b1/orphan.epub"}, resp.Latest.OrphanKeys)
	assert.Equal(t, int64(1), resp.Latest.DeletedOrphanCount)
	assert.Equal(t, int64(50), resp.Latest.DeletedOrphanSizeBytes)
}

func TestObservabilityTriggerStorageScan_NonAdmin(t *testing.T) {
	demoteToUser(t)
	client := observabilityClient(t)
	req := &observabilityv1.TriggerStorageScanRequest{}
	_, err := client.TriggerStorageScan(
		withCookies(context.Background(), accessToken),
		req,
	)
	requirePermissionDenied(t, err)
}

// The scan walks the whole bucket, so the RPC only starts it.
func TestObservabilityTriggerStorageScan_AsAdmin(t *testing.T) {
	promoteToAdmin(t)
	t.Cleanup(func() { demoteToUser(t) })
	runner := &stubStorageScanRunner{started: 0}
	withStorageScanRunner(t, runner)

	client := observabilityClient(t)
	req := &observabilityv1.TriggerStorageScanRequest{}
	_, err := client.TriggerStorageScan(
		withCookies(context.Background(), accessToken),
		req,
	)
	require.NoError(t, err)
	assert.Equal(t, 1, runner.started)
}

func TestObservabilityGetDatabaseStats_AsAdmin(t *testing.T) {
	promoteToAdmin(t)
	t.Cleanup(func() { demoteToUser(t) })

	client := observabilityClient(t)
	req := &observabilityv1.GetDatabaseStatsRequest{}
	resp, err := client.GetDatabaseStats(
		withCookies(context.Background(), accessToken),
		req,
	)
	require.NoError(t, err)
	assert.Positive(t, resp.TotalSizeBytes)

	var hasGlobal bool
	for _, s := range resp.Schemas {
		if s.Name == "global" {
			hasGlobal = true
		}
	}
	assert.True(t, hasGlobal)
}

func TestObservabilityGetDatabaseStats_NonAdmin(t *testing.T) {
	demoteToUser(t)
	client := observabilityClient(t)
	req := &observabilityv1.GetDatabaseStatsRequest{}
	_, err := client.GetDatabaseStats(withCookies(context.Background(), accessToken), req)
	requirePermissionDenied(t, err)
}

func requirePermissionDenied(t *testing.T, err error) {
	t.Helper()
	require.Error(t, err)
	var connectErr *connect.Error
	require.ErrorAs(t, err, &connectErr)
	assert.Equal(t, connect.CodePermissionDenied, connectErr.Code())
}
