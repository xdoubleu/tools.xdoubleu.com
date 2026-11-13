package services

import (
	"context"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"

	"tools.xdoubleu.com/apps/factory/internal/models"
)

const (
	// signalWindow is how far back finished jobs count towards the gauges.
	signalWindow = 24 * time.Hour
	// undeployedAfter is how long a merged task may take to go live.
	undeployedAfter = time.Hour
	// unmergeableAfter is how long an armed pull request may be unable to merge.
	unmergeableAfter = time.Hour
)

//nolint:gochecknoglobals //Prometheus collectors are process-wide by design
var (
	factoryRedPRs = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "factory_pull_requests_red",
		Help: "Open pull requests linking a task whose CI is failing.",
	})
	factoryUnmergeable = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "factory_pull_requests_unmergeable",
		Help: "Armed pull requests that conflict with or are behind main and have " +
			"been for over an hour.",
	})
	factoryUndeployed = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "factory_merged_tasks_undeployed",
		Help: "Tasks merged over an hour ago whose merge commit is not live.",
	})
	factoryJobsNoPR = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "factory_jobs_finished_without_pr",
		Help: "Jobs that succeeded in the last day without opening a pull request.",
	})
	factoryJobsUnverified = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "factory_jobs_unverified_outcome",
		Help: "Jobs finished in the last day whose outcome says not verified " +
			"or could not run.",
	})
)

// CollectSignals refreshes the failure gauges: red pull requests, merged but
// undeployed tasks, jobs that ended without a pull request, and outcomes that
// admit the work was unchecked. Red-for-30-minutes is the alert rule's `for`.
func (s *JobService) CollectSignals(ctx context.Context) error {
	now := s.now()
	counts, err := s.jobs.FailureCounts(ctx, now.Add(-signalWindow))
	if err != nil {
		return err
	}
	factoryRedPRs.Set(float64(counts.RedPullRequests))
	factoryJobsNoPR.Set(float64(counts.JobsWithoutPR))
	factoryJobsUnverified.Set(float64(counts.UnverifiedOutcome))

	stuck, err := s.linked.UnmergeableCount(ctx, now.Add(-unmergeableAfter))
	if err != nil {
		return err
	}
	factoryUnmergeable.Set(float64(stuck))

	tasks, err := s.tasks.MergedBetween(
		ctx, now.Add(-deployWindow), now.Add(-undeployedAfter))
	if err != nil {
		return err
	}
	s.FillDeploy(ctx, tasks)
	pending := 0
	for i := range tasks {
		if tasks[i].Deploy == models.DeployPending {
			pending++
		}
	}
	factoryUndeployed.Set(float64(pending))
	return nil
}
