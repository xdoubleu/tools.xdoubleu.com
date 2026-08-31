package jobs_test

import (
	"context"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"tools.xdoubleu.com/internal/mailer"
	"tools.xdoubleu.com/internal/models"
	"tools.xdoubleu.com/internal/observability/jobs"
	"tools.xdoubleu.com/internal/repositories"
	"tools.xdoubleu.com/internal/sentryapi"
)

// r2UsageAboveThreshold/ciDurationAboveThresholdMs mirror the unexported
// defaults in threshold_alert.go (50 GB / 15 minutes) — chosen well past
// them so a future default tweak within the same order of magnitude doesn't
// flip these tests.
const (
	r2UsageAboveThreshold    = 100 * 1024 * 1024 * 1024 // 100 GB
	ciDurationAboveThreshold = 25 * 60 * 1000           // 25 minutes
)

// mutableHostMetricsRepo is a *repositories.HostMetricsRepository stand-in
// whose samples can change between two job.Run calls in the same test, for
// the re-arm scenario.
type mutableHostMetricsRepo struct {
	mu      sync.Mutex
	samples []models.HostMetricSample
	err     error
}

func (f *mutableHostMetricsRepo) setSamples(samples []models.HostMetricSample) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.samples = samples
}

func (f *mutableHostMetricsRepo) Since(
	_ context.Context, _ time.Time,
) ([]models.HostMetricSample, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.samples, f.err
}

func cpuSamples(values ...float64) []models.HostMetricSample {
	samples := make([]models.HostMetricSample, len(values))
	for i, v := range values {
		samples[i] = models.HostMetricSample{
			SampledAt: time.Now(), CPUPercent: v, MemoryPercent: 0, DiskPercent: 0,
		}
	}
	return samples
}

func diskSamples(values ...float64) []models.HostMetricSample {
	samples := make([]models.HostMetricSample, len(values))
	for i, v := range values {
		samples[i] = models.HostMetricSample{
			SampledAt: time.Now(), CPUPercent: 0, MemoryPercent: 0, DiskPercent: v,
		}
	}
	return samples
}

type fakeWorkflowDurationStatsRepo struct {
	stats []models.WorkflowDurationStat
	err   error
}

func (f fakeWorkflowDurationStatsRepo) WorkflowDurationStats(
	_ context.Context, _ time.Time,
) ([]models.WorkflowDurationStat, error) {
	return f.stats, f.err
}

// fakeAlertStateRepo mirrors global.alert_states as an in-memory map, so
// state written by one job.Run call is read back by the next — same
// contract *repositories.AlertStatesRepository provides. mu guards it since
// notifications.Service delivers on a background worker.
type fakeAlertStateRepo struct {
	mu        sync.Mutex
	states    map[string]models.AlertState
	getErr    error
	upsertErr error
}

func newFakeAlertStateRepo() *fakeAlertStateRepo {
	return &fakeAlertStateRepo{ //nolint:exhaustruct // zero-value errs
		states: map[string]models.AlertState{},
	}
}

func (f *fakeAlertStateRepo) Get(
	_ context.Context, ruleKey string,
) (*models.AlertState, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.getErr != nil {
		return nil, f.getErr
	}
	s, ok := f.states[ruleKey]
	if !ok {
		return nil, nil //nolint:nilnil // "never evaluated" is a valid absence
	}
	return &s, nil
}

func (f *fakeAlertStateRepo) Upsert(_ context.Context, s models.AlertState) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.upsertErr != nil {
		return f.upsertErr
	}
	f.states[s.RuleKey] = s
	return nil
}

func (f *fakeAlertStateRepo) get(ruleKey string) (models.AlertState, bool) {
	f.mu.Lock()
	defer f.mu.Unlock()
	s, ok := f.states[ruleKey]
	return s, ok
}

func (f *fakeAlertStateRepo) len() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.states)
}

// onlyEnabledSettings enables exactly one source, so a test can exercise
// one rule without needing working fakes for every other rule's
// dependencies — a disabled rule's evaluate function is never called.
type onlyEnabledSettings struct {
	allowed repositories.NotificationSource
}

func (s onlyEnabledSettings) IsEnabled(
	_ context.Context, source repositories.NotificationSource,
) (bool, error) {
	return source == s.allowed, nil
}

type disabledSettings struct{}

func (disabledSettings) IsEnabled(
	_ context.Context, _ repositories.NotificationSource,
) (bool, error) {
	return false, nil
}

type erroringSettings struct{ err error }

func (s erroringSettings) IsEnabled(
	_ context.Context, _ repositories.NotificationSource,
) (bool, error) {
	return false, s.err
}

func subjectsContaining(sent []string, substr string) int {
	count := 0
	for _, s := range sent {
		if strings.Contains(s, substr) {
			count++
		}
	}
	return count
}

func TestThresholdAlertJob_IDAndSchedule(t *testing.T) {
	job := jobs.NewThresholdAlertJob(
		&mutableHostMetricsRepo{}, //nolint:exhaustruct // fixture
		noSnapshotGetter,
		fakeWorkflowDurationStatsRepo{}, //nolint:exhaustruct // fixture
		stubStatsLister{},               //nolint:exhaustruct // fixture
		disabledSettings{},
		newFakeAlertStateRepo(),
		testNotifications(t, &fakeMailer{}), //nolint:exhaustruct // fixture
	)
	assert.Equal(t, "threshold-alert", job.ID())
	assert.Equal(t, 5*time.Minute, job.RunEvery())
}

func TestThresholdAlertJob_HostCPUSustainedBreach_SendsBreachEmail(t *testing.T) {
	hostMetrics := &mutableHostMetricsRepo{ //nolint:exhaustruct // fixture
		samples: cpuSamples(90, 92, 95),
	}
	states := newFakeAlertStateRepo()
	mail := &fakeMailer{} //nolint:exhaustruct // fixture
	notifSvc := testNotifications(t, mail)

	job := jobs.NewThresholdAlertJob(
		hostMetrics, noSnapshotGetter,
		fakeWorkflowDurationStatsRepo{}, //nolint:exhaustruct // fixture
		stubStatsLister{},               //nolint:exhaustruct // fixture
		onlyEnabledSettings{allowed: repositories.NotificationSourceHostCPUHigh},
		states, notifSvc,
	)

	require.NoError(t, job.Run(t.Context(), testLogger()))
	notifSvc.WaitUntilDone()

	require.Len(t, mail.sent, 1)
	assert.Contains(t, mail.sent[0], "Host CPU usage above threshold")

	state, ok := states.get("host_cpu_high")
	require.True(t, ok)
	assert.True(t, state.Breaching)
	assert.NotNil(t, state.Since)
	assert.NotNil(t, state.LastNotifiedAt)
	assert.InDelta(t, 92.333, state.CurrentValue, 0.01)
}

func TestThresholdAlertJob_HostCPUNotSustained_NoBreach(t *testing.T) {
	// One sample at/under threshold breaks the "every sample" requirement.
	hostMetrics := &mutableHostMetricsRepo{ //nolint:exhaustruct // fixture
		samples: cpuSamples(90, 90, 70),
	}
	states := newFakeAlertStateRepo()
	mail := &fakeMailer{} //nolint:exhaustruct // fixture
	notifSvc := testNotifications(t, mail)

	job := jobs.NewThresholdAlertJob(
		hostMetrics, noSnapshotGetter,
		fakeWorkflowDurationStatsRepo{}, //nolint:exhaustruct // fixture
		stubStatsLister{},               //nolint:exhaustruct // fixture
		onlyEnabledSettings{allowed: repositories.NotificationSourceHostCPUHigh},
		states, notifSvc,
	)

	require.NoError(t, job.Run(t.Context(), testLogger()))
	notifSvc.WaitUntilDone()

	assert.Empty(t, mail.sent)
	state, ok := states.get("host_cpu_high")
	require.True(t, ok)
	assert.False(t, state.Breaching)
	assert.Nil(t, state.Since)
}

func TestThresholdAlertJob_HostCPUNoSamples_NoBreachNoError(t *testing.T) {
	hostMetrics := &mutableHostMetricsRepo{} //nolint:exhaustruct // fixture, no samples
	states := newFakeAlertStateRepo()
	mail := &fakeMailer{} //nolint:exhaustruct // fixture
	notifSvc := testNotifications(t, mail)

	job := jobs.NewThresholdAlertJob(
		hostMetrics, noSnapshotGetter,
		fakeWorkflowDurationStatsRepo{}, //nolint:exhaustruct // fixture
		stubStatsLister{},               //nolint:exhaustruct // fixture
		onlyEnabledSettings{allowed: repositories.NotificationSourceHostCPUHigh},
		states, notifSvc,
	)

	require.NoError(t, job.Run(t.Context(), testLogger()))
	notifSvc.WaitUntilDone()

	assert.Empty(t, mail.sent)
	state, ok := states.get("host_cpu_high")
	require.True(t, ok)
	assert.False(t, state.Breaching)
	assert.Zero(t, state.CurrentValue)
}

func TestThresholdAlertJob_HostDiskInstant_BreachesOnLatestSampleOnly(t *testing.T) {
	// Latest sample above threshold breaches even though an earlier one in
	// the same window wasn't — host_disk_high is instant, not sustained.
	hostMetrics := &mutableHostMetricsRepo{ //nolint:exhaustruct // fixture
		samples: diskSamples(10, 95),
	}
	states := newFakeAlertStateRepo()
	mail := &fakeMailer{} //nolint:exhaustruct // fixture
	notifSvc := testNotifications(t, mail)

	job := jobs.NewThresholdAlertJob(
		hostMetrics, noSnapshotGetter,
		fakeWorkflowDurationStatsRepo{}, //nolint:exhaustruct // fixture
		stubStatsLister{},               //nolint:exhaustruct // fixture
		onlyEnabledSettings{allowed: repositories.NotificationSourceHostDiskHigh},
		states, notifSvc,
	)

	require.NoError(t, job.Run(t.Context(), testLogger()))
	notifSvc.WaitUntilDone()

	require.Len(t, mail.sent, 1)
	state, ok := states.get("host_disk_high")
	require.True(t, ok)
	assert.True(t, state.Breaching)
	assert.InDelta(t, 95, state.CurrentValue, 0.01)
}

func TestThresholdAlertJob_R2UsageHigh_Breach(t *testing.T) {
	storage := fakeStorageSnapshotGetter{ //nolint:exhaustruct // fixture
		snap: &models.StorageSnapshot{ //nolint:exhaustruct // fixture
			TotalSizeBytes: r2UsageAboveThreshold,
		},
	}
	states := newFakeAlertStateRepo()
	mail := &fakeMailer{} //nolint:exhaustruct // fixture
	notifSvc := testNotifications(t, mail)

	job := jobs.NewThresholdAlertJob(
		&mutableHostMetricsRepo{}, //nolint:exhaustruct // fixture
		storage,
		fakeWorkflowDurationStatsRepo{}, //nolint:exhaustruct // fixture
		stubStatsLister{},               //nolint:exhaustruct // fixture
		onlyEnabledSettings{allowed: repositories.NotificationSourceR2UsageHigh},
		states, notifSvc,
	)

	require.NoError(t, job.Run(t.Context(), testLogger()))
	notifSvc.WaitUntilDone()

	require.Len(t, mail.sent, 1)
	assert.Contains(t, mail.sent[0], "R2 storage usage above threshold")
	state, ok := states.get("r2_usage_high")
	require.True(t, ok)
	assert.True(t, state.Breaching)
}

func TestThresholdAlertJob_CIDurationHigh_Breach(t *testing.T) {
	workflowRuns := fakeWorkflowDurationStatsRepo{
		stats: []models.WorkflowDurationStat{
			{
				WorkflowName: "ci", AvgDurationMs: ciDurationAboveThreshold,
				P95DurationMs: ciDurationAboveThreshold, RunCount: 5,
			},
		},
		err: nil,
	}
	states := newFakeAlertStateRepo()
	mail := &fakeMailer{} //nolint:exhaustruct // fixture
	notifSvc := testNotifications(t, mail)

	job := jobs.NewThresholdAlertJob(
		&mutableHostMetricsRepo{}, //nolint:exhaustruct // fixture
		noSnapshotGetter,
		workflowRuns,
		stubStatsLister{}, //nolint:exhaustruct // fixture
		onlyEnabledSettings{allowed: repositories.NotificationSourceCIDurationHigh},
		states, notifSvc,
	)

	require.NoError(t, job.Run(t.Context(), testLogger()))
	notifSvc.WaitUntilDone()

	require.Len(t, mail.sent, 1)
	state, ok := states.get("ci_duration_high")
	require.True(t, ok)
	assert.True(t, state.Breaching)
	assert.InDelta(t, ciDurationAboveThreshold, state.CurrentValue, 0.01)
}

// TestThresholdAlertJob_ReArmEmailsTwice is the core scenario issue #1283
// exists for: global.notified_issues' append-only dedup can only notify
// once ever, but a breach → recover → breach cycle must email on both
// breaches, since the second incident is a genuinely new event.
func TestThresholdAlertJob_ReArmEmailsTwice(t *testing.T) {
	hostMetrics := &mutableHostMetricsRepo{ //nolint:exhaustruct // fixture
		samples: cpuSamples(90, 92, 95),
	}
	states := newFakeAlertStateRepo()
	mail := &fakeMailer{} //nolint:exhaustruct // fixture
	notifSvc := testNotifications(t, mail)

	job := jobs.NewThresholdAlertJob(
		hostMetrics, noSnapshotGetter,
		fakeWorkflowDurationStatsRepo{}, //nolint:exhaustruct // fixture
		stubStatsLister{},               //nolint:exhaustruct // fixture
		onlyEnabledSettings{allowed: repositories.NotificationSourceHostCPUHigh},
		states, notifSvc,
	)

	// Breach.
	require.NoError(t, job.Run(t.Context(), testLogger()))
	notifSvc.WaitUntilDone()

	// Recover.
	hostMetrics.setSamples(cpuSamples(10, 12, 15))
	require.NoError(t, job.Run(t.Context(), testLogger()))
	notifSvc.WaitUntilDone()

	// Breach again -- the re-arm.
	hostMetrics.setSamples(cpuSamples(91, 93, 96))
	require.NoError(t, job.Run(t.Context(), testLogger()))
	notifSvc.WaitUntilDone()

	assert.Equal(t, 2, subjectsContaining(mail.sent, "above threshold"))
	assert.Equal(t, 1, subjectsContaining(mail.sent, "recovered"))

	state, ok := states.get("host_cpu_high")
	require.True(t, ok)
	assert.True(t, state.Breaching)
}

func TestThresholdAlertJob_DisabledSource_SkipsWithoutWritingState(t *testing.T) {
	hostMetrics := &mutableHostMetricsRepo{ //nolint:exhaustruct // fixture
		samples: cpuSamples(90, 92, 95),
	}
	states := newFakeAlertStateRepo()
	mail := &fakeMailer{} //nolint:exhaustruct // fixture
	notifSvc := testNotifications(t, mail)

	job := jobs.NewThresholdAlertJob(
		hostMetrics, noSnapshotGetter,
		fakeWorkflowDurationStatsRepo{}, //nolint:exhaustruct // fixture
		stubStatsLister{},               //nolint:exhaustruct // fixture
		disabledSettings{},
		states, notifSvc,
	)

	require.NoError(t, job.Run(t.Context(), testLogger()))
	notifSvc.WaitUntilDone()

	assert.Empty(t, mail.sent)
	assert.Zero(t, states.len())
}

func TestThresholdAlertJob_SettingsErrorPropagates(t *testing.T) {
	job := jobs.NewThresholdAlertJob(
		&mutableHostMetricsRepo{}, //nolint:exhaustruct // fixture
		noSnapshotGetter,
		fakeWorkflowDurationStatsRepo{}, //nolint:exhaustruct // fixture
		stubStatsLister{},               //nolint:exhaustruct // fixture
		erroringSettings{err: assert.AnError},
		newFakeAlertStateRepo(),
		testNotifications(t, &fakeMailer{}), //nolint:exhaustruct // fixture
	)

	err := job.Run(t.Context(), testLogger())
	require.ErrorIs(t, err, assert.AnError)
}

func TestThresholdAlertJob_EvaluateErrorLogsAndContinues(t *testing.T) {
	hostMetrics := &mutableHostMetricsRepo{ //nolint:exhaustruct // fixture
		err: assert.AnError,
	}
	states := newFakeAlertStateRepo()
	logger, buf := testLoggerWithBuf()

	job := jobs.NewThresholdAlertJob(
		hostMetrics, noSnapshotGetter,
		fakeWorkflowDurationStatsRepo{}, //nolint:exhaustruct // fixture
		stubStatsLister{},               //nolint:exhaustruct // fixture
		onlyEnabledSettings{allowed: repositories.NotificationSourceHostCPUHigh},
		states,
		testNotifications(t, &fakeMailer{}), //nolint:exhaustruct // fixture
	)

	require.NoError(t, job.Run(t.Context(), logger))
	assert.Contains(t, buf.String(), "evaluate failed")
	assert.Zero(t, states.len())
}

func TestThresholdAlertJob_StatesGetErrorPropagates(t *testing.T) {
	hostMetrics := &mutableHostMetricsRepo{ //nolint:exhaustruct // fixture
		samples: cpuSamples(90, 92, 95),
	}
	states := newFakeAlertStateRepo()
	states.getErr = assert.AnError

	job := jobs.NewThresholdAlertJob(
		hostMetrics, noSnapshotGetter,
		fakeWorkflowDurationStatsRepo{}, //nolint:exhaustruct // fixture
		stubStatsLister{},               //nolint:exhaustruct // fixture
		onlyEnabledSettings{allowed: repositories.NotificationSourceHostCPUHigh},
		states,
		testNotifications(t, &fakeMailer{}), //nolint:exhaustruct // fixture
	)

	err := job.Run(t.Context(), testLogger())
	require.ErrorIs(t, err, assert.AnError)
}

func TestThresholdAlertJob_MailerNotConfigured_RetriesNextRun(t *testing.T) {
	hostMetrics := &mutableHostMetricsRepo{ //nolint:exhaustruct // fixture
		samples: cpuSamples(90, 92, 95),
	}
	states := newFakeAlertStateRepo()
	mail := &fakeMailer{err: mailer.ErrNotConfigured} //nolint:exhaustruct // fixture
	notifSvc := testNotifications(t, mail)

	job := jobs.NewThresholdAlertJob(
		hostMetrics, noSnapshotGetter,
		fakeWorkflowDurationStatsRepo{}, //nolint:exhaustruct // fixture
		stubStatsLister{},               //nolint:exhaustruct // fixture
		onlyEnabledSettings{allowed: repositories.NotificationSourceHostCPUHigh},
		states, notifSvc,
	)

	require.NoError(t, job.Run(t.Context(), testLogger()))
	notifSvc.WaitUntilDone()

	// The mailer being unconfigured means no state row was written yet --
	// the transition into breach is retried on the next run instead of
	// silently marked handled.
	_, ok := states.get("host_cpu_high")
	assert.False(t, ok)
}

func TestThresholdAlertJob_SlowTransactionHTTPHigh_Breach(t *testing.T) {
	stats := stubStatsLister{ //nolint:exhaustruct // fixture
		stats: []sentryapi.TransactionStat{
			{
				Transaction:   "POST /games.v1.GamesService/RefreshSteamGame",
				Project:       "tools-api",
				P95DurationMs: 144000,
				RequestCount:  35,
			},
			{
				Transaction:   "steam",
				Project:       "tools-api",
				P95DurationMs: 24000,
				RequestCount:  2,
			},
			{
				Transaction:   "/dashboard/games",
				Project:       "tools-web",
				P95DurationMs: 4000,
				RequestCount:  8,
			},
		},
	}
	states := newFakeAlertStateRepo()
	mail := &fakeMailer{} //nolint:exhaustruct // fixture
	notifSvc := testNotifications(t, mail)

	job := jobs.NewThresholdAlertJob(
		&mutableHostMetricsRepo{}, //nolint:exhaustruct // fixture
		noSnapshotGetter,
		fakeWorkflowDurationStatsRepo{}, //nolint:exhaustruct // fixture
		stats,
		onlyEnabledSettings{
			allowed: repositories.NotificationSourceSlowHTTPHigh,
		},
		states,
		notifSvc,
	)

	require.NoError(t, job.Run(t.Context(), testLogger()))
	notifSvc.WaitUntilDone()

	require.Len(t, mail.sent, 1)
	state, ok := states.get("slow_transaction_http_high")
	require.True(t, ok)
	assert.True(t, state.Breaching)
	assert.InDelta(t, 144000, state.CurrentValue, 0.01)
}

func TestThresholdAlertJob_SlowTransactionHTTPHigh_ProgressWebSocketBreaches(
	t *testing.T,
) {
	// GET /games/api/progress and GET /books/api/progress are WebSocket
	// upgrades (internal/progressws), not bounded request/response
	// handlers -- their Sentry transaction spans the whole connection
	// lifetime, so they permanently breach the HTTP handler rule the
	// moment any client session outlives the 5s threshold (issue #1320).
	// That's accepted as an inherent characteristic of a long-lived
	// WebSocket route rather than excluded: acceptWithHandshakeSpan
	// (internal/communication/wstools/websocket.go) tracks the actually-
	// bounded part of the route (the handshake) separately instead. The
	// sibling ".../refresh" route is a normal bounded handler.
	stats := stubStatsLister{ //nolint:exhaustruct // fixture
		stats: []sentryapi.TransactionStat{
			{
				Transaction:   "GET /games/api/progress",
				Project:       "tools-api",
				P95DurationMs: 125123,
				RequestCount:  180,
			},
			{
				Transaction:   "GET /books/api/progress",
				Project:       "tools-api",
				P95DurationMs: 300000,
				RequestCount:  90,
			},
			{
				Transaction:   "GET /games/api/progress/{id}/refresh",
				Project:       "tools-api",
				P95DurationMs: 2000,
				RequestCount:  10,
			},
		},
	}
	states := newFakeAlertStateRepo()
	mail := &fakeMailer{} //nolint:exhaustruct // fixture
	notifSvc := testNotifications(t, mail)

	job := jobs.NewThresholdAlertJob(
		&mutableHostMetricsRepo{}, //nolint:exhaustruct // fixture
		noSnapshotGetter,
		fakeWorkflowDurationStatsRepo{}, //nolint:exhaustruct // fixture
		stats,
		onlyEnabledSettings{
			allowed: repositories.NotificationSourceSlowHTTPHigh,
		},
		states,
		notifSvc,
	)

	require.NoError(t, job.Run(t.Context(), testLogger()))
	notifSvc.WaitUntilDone()

	require.Len(t, mail.sent, 1)
	state, ok := states.get("slow_transaction_http_high")
	require.True(t, ok)
	assert.True(t, state.Breaching)
	assert.InDelta(t, 300000, state.CurrentValue, 0.01)
}

func TestThresholdAlertJob_SlowTransactionJobHigh_NotBreachingBelowThreshold(
	t *testing.T,
) {
	// steam at 24s is a legitimate long-running background job, well under
	// the 60s job threshold -- it must not breach on its own.
	stats := stubStatsLister{ //nolint:exhaustruct // fixture
		stats: []sentryapi.TransactionStat{
			{
				Transaction:   "steam",
				Project:       "tools-api",
				P95DurationMs: 24000,
				RequestCount:  2,
			},
			{
				Transaction:   "GET /games/api/progress",
				Project:       "tools-api",
				P95DurationMs: 144000,
				RequestCount:  35,
			},
		},
	}
	states := newFakeAlertStateRepo()
	mail := &fakeMailer{} //nolint:exhaustruct // fixture
	notifSvc := testNotifications(t, mail)

	job := jobs.NewThresholdAlertJob(
		&mutableHostMetricsRepo{}, //nolint:exhaustruct // fixture
		noSnapshotGetter,
		fakeWorkflowDurationStatsRepo{}, //nolint:exhaustruct // fixture
		stats,
		onlyEnabledSettings{
			allowed: repositories.NotificationSourceSlowJobHigh,
		},
		states,
		notifSvc,
	)

	require.NoError(t, job.Run(t.Context(), testLogger()))
	notifSvc.WaitUntilDone()

	assert.Empty(t, mail.sent)
	state, ok := states.get("slow_transaction_job_high")
	require.True(t, ok)
	assert.False(t, state.Breaching)
	assert.InDelta(t, 24000, state.CurrentValue, 0.01)
}

func TestThresholdAlertJob_SlowTransactionFrontendHigh_ExcludedTransactionIgnored(
	t *testing.T,
) {
	// NextNodeServer.clientComponentLoading is a known Next.js-internal
	// transaction, not a real page load -- it must never breach the
	// frontend rule even at 61s.
	stats := stubStatsLister{ //nolint:exhaustruct // fixture
		stats: []sentryapi.TransactionStat{
			{
				Transaction:   "NextNodeServer.clientComponentLoading",
				Project:       "tools-web",
				P95DurationMs: 61000,
				RequestCount:  548,
			},
			{
				Transaction:   "/dashboard/games",
				Project:       "tools-web",
				P95DurationMs: 2000,
				RequestCount:  8,
			},
		},
	}
	states := newFakeAlertStateRepo()
	mail := &fakeMailer{} //nolint:exhaustruct // fixture
	notifSvc := testNotifications(t, mail)

	job := jobs.NewThresholdAlertJob(
		&mutableHostMetricsRepo{}, //nolint:exhaustruct // fixture
		noSnapshotGetter,
		fakeWorkflowDurationStatsRepo{}, //nolint:exhaustruct // fixture
		stats,
		onlyEnabledSettings{
			allowed: repositories.NotificationSourceSlowFEHigh,
		},
		states,
		notifSvc,
	)

	require.NoError(t, job.Run(t.Context(), testLogger()))
	notifSvc.WaitUntilDone()

	assert.Empty(t, mail.sent)
	state, ok := states.get("slow_transaction_frontend_high")
	require.True(t, ok)
	assert.False(t, state.Breaching)
	assert.InDelta(t, 2000, state.CurrentValue, 0.01)
}

func TestThresholdAlertJob_SlowTransactionHigh_NotConfigured_NoBreachNoError(
	t *testing.T,
) {
	stats := stubStatsLister{ //nolint:exhaustruct // fixture
		err: sentryapi.ErrNotConfigured,
	}
	states := newFakeAlertStateRepo()
	mail := &fakeMailer{} //nolint:exhaustruct // fixture
	notifSvc := testNotifications(t, mail)

	job := jobs.NewThresholdAlertJob(
		&mutableHostMetricsRepo{}, //nolint:exhaustruct // fixture
		noSnapshotGetter,
		fakeWorkflowDurationStatsRepo{}, //nolint:exhaustruct // fixture
		stats,
		onlyEnabledSettings{
			allowed: repositories.NotificationSourceSlowHTTPHigh,
		},
		states,
		notifSvc,
	)

	require.NoError(t, job.Run(t.Context(), testLogger()))
	notifSvc.WaitUntilDone()

	assert.Empty(t, mail.sent)
	state, ok := states.get("slow_transaction_http_high")
	require.True(t, ok)
	assert.False(t, state.Breaching)
	assert.Zero(t, state.CurrentValue)
}
