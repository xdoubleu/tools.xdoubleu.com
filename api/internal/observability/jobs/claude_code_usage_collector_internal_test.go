package jobs

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"tools.xdoubleu.com/internal/anthropicadmin"
)

type stubClaudeCodeUsageClient struct {
	records []anthropicadmin.UsageRecord
	err     error
}

func (s stubClaudeCodeUsageClient) GetClaudeCodeUsage(
	_ context.Context, _ string,
) ([]anthropicadmin.UsageRecord, error) {
	return s.records, s.err
}

func pinNow(t *testing.T, when time.Time) {
	t.Helper()
	original := nowFn
	nowFn = func() time.Time { return when }
	t.Cleanup(func() { nowFn = original })
}

func TestCollectClaudeCodeUsageJob_ID(t *testing.T) {
	job := NewCollectClaudeCodeUsageJob(
		stubClaudeCodeUsageClient{records: nil, err: nil},
	)
	assert.Equal(t, "collect-claude-code-usage", job.ID())
}

func TestCollectClaudeCodeUsageJob_RunEvery(t *testing.T) {
	job := NewCollectClaudeCodeUsageJob(
		stubClaudeCodeUsageClient{records: nil, err: nil},
	)
	assert.Equal(t, claudeCodeUsageRunEvery, job.RunEvery())
}

func TestCollectClaudeCodeUsageJob_Run_NotConfigured(t *testing.T) {
	pinNow(t, time.Date(2026, 9, 14, 12, 0, 0, 0, time.UTC))
	logger, _ := loggerWithBuf()

	job := NewCollectClaudeCodeUsageJob(
		stubClaudeCodeUsageClient{records: nil, err: anthropicadmin.ErrNotConfigured},
	)
	err := job.Run(t.Context(), logger)
	require.NoError(t, err)
}

func TestCollectClaudeCodeUsageJob_Run_APIError(t *testing.T) {
	pinNow(t, time.Date(2026, 9, 14, 12, 0, 0, 0, time.UTC))
	logger, _ := loggerWithBuf()

	job := NewCollectClaudeCodeUsageJob(
		stubClaudeCodeUsageClient{records: nil, err: errors.New("boom")},
	)
	err := job.Run(t.Context(), logger)
	require.NoError(t, err)
}

func TestCollectClaudeCodeUsageJob_Run_SetsGauges(t *testing.T) {
	pinNow(t, time.Date(2026, 9, 14, 12, 0, 0, 0, time.UTC))
	logger, _ := loggerWithBuf()

	records := []anthropicadmin.UsageRecord{
		{
			CustomerType: "subscription",
			Date:         "2026-09-14T00:00:00Z",
			ModelBreakdown: []anthropicadmin.ModelBreakdown{
				{
					Model: "claude-sonnet-5",
					Tokens: anthropicadmin.Tokens{
						Input: 100, Output: 50, CacheCreation: 10, CacheRead: 5,
					},
					EstimatedCost: anthropicadmin.EstimatedCost{
						Amount: 200, Currency: "USD",
					},
				},
			},
		},
		{
			CustomerType: "api",
			Date:         "2026-09-14T00:00:00Z",
			ModelBreakdown: []anthropicadmin.ModelBreakdown{
				{
					Model: "claude-sonnet-5",
					Tokens: anthropicadmin.Tokens{
						Input: 25, Output: 10, CacheCreation: 0, CacheRead: 0,
					},
					EstimatedCost: anthropicadmin.EstimatedCost{
						Amount: 50, Currency: "USD",
					},
				},
			},
		},
	}

	job := NewCollectClaudeCodeUsageJob(
		stubClaudeCodeUsageClient{records: records, err: nil},
	)
	err := job.Run(t.Context(), logger)
	require.NoError(t, err)

	assert.InEpsilon(
		t,
		float64(125),
		testutil.ToFloat64(
			claudeCodeTokens.WithLabelValues("claude-sonnet-5", "input"),
		),
		0,
	)
	assert.InEpsilon(
		t,
		float64(60),
		testutil.ToFloat64(
			claudeCodeTokens.WithLabelValues("claude-sonnet-5", "output"),
		),
		0,
	)
	assert.InEpsilon(
		t,
		float64(10),
		testutil.ToFloat64(
			claudeCodeTokens.WithLabelValues("claude-sonnet-5", "cache_creation"),
		),
		0,
	)
	assert.InEpsilon(
		t,
		float64(5),
		testutil.ToFloat64(
			claudeCodeTokens.WithLabelValues("claude-sonnet-5", "cache_read"),
		),
		0,
	)
	assert.InEpsilon(
		t,
		float64(2.5),
		testutil.ToFloat64(
			claudeCodeEstimatedCostUSD.WithLabelValues("claude-sonnet-5"),
		),
		0,
	)
}
