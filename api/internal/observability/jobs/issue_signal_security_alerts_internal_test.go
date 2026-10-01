package jobs

import (
	"errors"
	"testing"

	"github.com/prometheus/client_golang/prometheus/testutil"
	"github.com/stretchr/testify/assert"

	"tools.xdoubleu.com/internal/github"
)

func securityAlertsJob(
	alerts []github.SecurityAlert, err error,
) *IssueSignalCollectorJob {
	//nolint:exhaustruct //only the security-alert lister is exercised
	return &IssueSignalCollectorJob{
		gh: stubGithubClient{
			prs: nil, prsErr: nil, runs: nil, runsErr: nil,
			alerts: alerts, alertsErr: err,
			vars: nil, varsErr: nil,
		},
	}
}

func TestCollectSecurityAlertsNoneSeedsZeroSeries(t *testing.T) {
	resetGauges()
	logger, buf := loggerWithBuf()

	securityAlertsJob(nil, nil).collectSecurityAlerts(t.Context(), logger)

	assert.Equal(t, len(seededSeverities),
		testutil.CollectAndCount(githubOpenSecurityAlerts))
	for _, severity := range seededSeverities {
		assert.InDelta(t, 0.0, testutil.ToFloat64(
			githubOpenSecurityAlerts.WithLabelValues(severity)), 0)
	}
	assert.InDelta(t, 1.0, testutil.ToFloat64(githubSecurityAlertsFetchOK), 0)
	assert.Empty(t, buf.String())
}

func TestCollectSecurityAlertsCountsUnseededSeverity(t *testing.T) {
	resetGauges()
	logger, _ := loggerWithBuf()

	securityAlertsJob([]github.SecurityAlert{
		alertWithSeverity("critical"),
		alertWithSeverity(""),
	}, nil).collectSecurityAlerts(t.Context(), logger)

	assert.InDelta(t, 1.0, testutil.ToFloat64(
		githubOpenSecurityAlerts.WithLabelValues("critical")), 0)
	assert.InDelta(t, 1.0, testutil.ToFloat64(
		githubOpenSecurityAlerts.WithLabelValues("")), 0)
	assert.Equal(t, len(seededSeverities)+1,
		testutil.CollectAndCount(githubOpenSecurityAlerts))
}

func TestCollectSecurityAlertsFailureClearsFetchOK(t *testing.T) {
	for name, err := range map[string]error{
		"forbidden":      errors.New("github API returned 403: forbidden"),
		"not configured": github.ErrNotConfigured,
	} {
		t.Run(name, func(t *testing.T) {
			resetGauges()
			githubSecurityAlertsFetchOK.Set(1)
			githubOpenSecurityAlerts.WithLabelValues("high").Set(3)
			logger, _ := loggerWithBuf()

			securityAlertsJob(nil, err).collectSecurityAlerts(t.Context(), logger)

			assert.InDelta(t, 0.0,
				testutil.ToFloat64(githubSecurityAlertsFetchOK), 0)
			assert.InDelta(t, 3.0, testutil.ToFloat64(
				githubOpenSecurityAlerts.WithLabelValues("high")), 0)
		})
	}
}

func TestCollectSecurityAlertsForbiddenIsLogged(t *testing.T) {
	resetGauges()
	logger, buf := loggerWithBuf()

	securityAlertsJob(nil, errors.New("github API returned 403: forbidden")).
		collectSecurityAlerts(t.Context(), logger)

	assert.Contains(t, buf.String(), "failed to list security alerts")
}
