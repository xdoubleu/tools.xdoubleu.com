package jobs

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"

	"tools.xdoubleu.com/internal/anthropicadmin"
)

// Claude Code usage gauges, registered on client_golang's default registry
// which cmd/api's /metrics handler already serves (mirrors the gauges in
// issue_signal_collector.go). CollectClaudeCodeUsageJob refreshes them on a
// timer — never at scrape time — since the Anthropic Admin API is a
// third-party dependency this process shouldn't block a scrape on.
//
//nolint:gochecknoglobals //Prometheus collectors are process-wide by design
var (
	claudeCodeTokens = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Name: "claude_code_tokens",
		Help: "Daily Claude Code token usage by model and token type " +
			"(input, output, cache_creation, cache_read), summed across every " +
			"actor in the organization.",
	}, []string{"model", "token_type"})
	claudeCodeEstimatedCostUSD = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Name: "claude_code_estimated_cost_usd",
		Help: "Daily Claude Code estimated cost by model, in USD, summed " +
			"across every actor in the organization.",
	}, []string{"model"})
)

// centsPerDollar converts EstimatedCost.Amount (minor currency units, e.g.
// cents for USD) to the dollar unit claudeCodeEstimatedCostUSD reports in.
const centsPerDollar = 100

// claudeCodeUsageRunEvery matches the Claude Code Analytics API's own
// documented data freshness (up to a 1-hour delay) — polling much faster
// would just re-read the same still-stale numbers.
const claudeCodeUsageRunEvery = 30 * time.Minute

// claudeCodeUsageClient is the subset of anthropicadmin.Client
// CollectClaudeCodeUsageJob reads.
type claudeCodeUsageClient interface {
	GetClaudeCodeUsage(
		ctx context.Context,
		date string,
	) ([]anthropicadmin.UsageRecord, error)
}

// nowFn returns the current time; a var so tests can pin "today".
//
//nolint:gochecknoglobals //test seam, see comment above
var nowFn = time.Now

// CollectClaudeCodeUsageJob refreshes the Claude Code token-usage and
// estimated-cost Prometheus gauges from the Anthropic Admin API's Claude
// Code Analytics endpoint, for the current UTC day. A missing Admin API key
// leaves the gauges untouched rather than resetting them to zero; other
// errors are logged and skipped. Run always returns nil.
type CollectClaudeCodeUsageJob struct {
	client claudeCodeUsageClient
}

func NewCollectClaudeCodeUsageJob(
	client claudeCodeUsageClient,
) *CollectClaudeCodeUsageJob {
	return &CollectClaudeCodeUsageJob{client: client}
}

func (j *CollectClaudeCodeUsageJob) ID() string {
	return "collect-claude-code-usage"
}

func (j *CollectClaudeCodeUsageJob) RunEvery() time.Duration {
	return claudeCodeUsageRunEvery
}

func (j *CollectClaudeCodeUsageJob) Run(
	ctx context.Context,
	logger *slog.Logger,
) error {
	today := nowFn().UTC().Format(time.DateOnly)

	records, err := j.client.GetClaudeCodeUsage(ctx, today)
	if errors.Is(err, anthropicadmin.ErrNotConfigured) {
		return nil
	}
	if err != nil {
		logAPIErr(ctx, logger,
			"claude-code-usage-collector: failed to fetch usage report",
			err, anthropicadmin.IsTransientAPIError(err))
		return nil
	}

	tokensByModel := make(map[string]map[string]float64)
	costByModel := make(map[string]float64)
	for _, record := range records {
		for _, mb := range record.ModelBreakdown {
			byType, ok := tokensByModel[mb.Model]
			if !ok {
				byType = make(map[string]float64)
				tokensByModel[mb.Model] = byType
			}
			byType["input"] += mb.Tokens.Input
			byType["output"] += mb.Tokens.Output
			byType["cache_creation"] += mb.Tokens.CacheCreation
			byType["cache_read"] += mb.Tokens.CacheRead

			// Assumes USD throughout — EstimatedCost.Currency is not checked;
			// the Admin API has no documented non-USD org, and adding
			// multi-currency label cardinality here would be speculative.
			costByModel[mb.Model] += mb.EstimatedCost.Amount / centsPerDollar
		}
	}

	claudeCodeTokens.Reset()
	for model, byType := range tokensByModel {
		for tokenType, total := range byType {
			claudeCodeTokens.WithLabelValues(model, tokenType).Set(total)
		}
	}

	claudeCodeEstimatedCostUSD.Reset()
	for model, total := range costByModel {
		claudeCodeEstimatedCostUSD.WithLabelValues(model).Set(total)
	}

	return nil
}
