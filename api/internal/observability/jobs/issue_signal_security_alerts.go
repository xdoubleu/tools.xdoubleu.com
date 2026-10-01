package jobs

import (
	"context"
	"errors"
	"log/slog"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"

	"tools.xdoubleu.com/internal/github"
)

//nolint:gochecknoglobals //Prometheus collectors are process-wide by design
var (
	githubOpenSecurityAlerts = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Name: "github_open_security_alerts",
		Help: "Open Dependabot, code-scanning and secret-scanning alerts, by severity.",
	}, []string{"severity"})
	githubSecurityAlertsFetchOK = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "github_security_alerts_fetch_ok",
		Help: "1 when the last security-alert fetch succeeded, 0 when GitHub " +
			"isn't connected or the fetch failed (e.g. a 403 for a missing " +
			"scope or permission), so github_open_security_alerts is stale.",
	})
)

// seededSeverities always get a series, so zero open alerts reads as 0
// rather than an absent gauge that looks like a dead collector.
//
//nolint:gochecknoglobals //small fixed list, read-only
var seededSeverities = []string{"critical", "high", "medium", "low"}

// collectSecurityAlerts leaves github_open_security_alerts untouched on
// failure and records it in github_security_alerts_fetch_ok instead.
func (j *IssueSignalCollectorJob) collectSecurityAlerts(
	ctx context.Context,
	logger *slog.Logger,
) {
	alerts, err := j.gh.ListSecurityAlerts(ctx)
	if errors.Is(err, github.ErrNotConfigured) {
		githubSecurityAlertsFetchOK.Set(0)
		return
	}
	if err != nil {
		githubSecurityAlertsFetchOK.Set(0)
		logAPIErr(ctx, logger,
			"issue-signal-collector: failed to list security alerts",
			err, github.IsTransientAPIError(err))
		return
	}

	bySeverity := make(map[string]int, len(seededSeverities))
	for _, severity := range seededSeverities {
		bySeverity[severity] = 0
	}
	for _, alert := range alerts {
		bySeverity[alert.Severity]++
	}
	githubOpenSecurityAlerts.Reset()
	for severity, count := range bySeverity {
		githubOpenSecurityAlerts.WithLabelValues(severity).Set(float64(count))
	}
	githubSecurityAlertsFetchOK.Set(1)
}
