package factory_test

import (
	"testing"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	factoryv1 "tools.xdoubleu.com/gen/factory/v1"
)

// sampleCount is the observations of a histogram, or the total of a counter,
// summed over its label sets; 0 when nothing was recorded.
func sampleCount(t *testing.T, name string) float64 {
	t.Helper()
	families, err := prometheus.DefaultGatherer.Gather()
	require.NoError(t, err)
	var total float64
	for _, f := range families {
		if f.GetName() != name {
			continue
		}
		for _, m := range f.GetMetric() {
			switch {
			case m.GetHistogram() != nil:
				total += float64(m.GetHistogram().GetSampleCount())
			case m.GetCounter() != nil:
				total += m.GetCounter().GetValue()
			}
		}
	}
	return total
}

func TestAFinishedJobsTimingsAreExported(t *testing.T) {
	env := browserJobEnv(t)
	job := env.runningJob(t)
	phases := sampleCount(t, "factory_job_phase_seconds")
	attempts := sampleCount(t, "factory_job_attempts_total")
	merges := sampleCount(t, "factory_job_time_to_merge_seconds")

	_, err := env.runner.CompleteJob(bearer(env.token), &factoryv1.CompleteJobRequest{
		JobId: job.Id, Succeeded: true,
		Timings: &factoryv1.JobTimings{
			AgentSeconds: 120, PublishSeconds: 5, CiSeconds: 300,
			MergeSeconds: 60, TotalSeconds: 430, GateRetries: 1, CiRetries: 2, PushRetries: 1000,
		},
	})
	require.NoError(t, err)

	assert.InDelta(t, 4, sampleCount(t, "factory_job_phase_seconds")-phases, 0)
	assert.InDelta(t, 103, sampleCount(t, "factory_job_attempts_total")-attempts, 0,
		"a runner's retry count is capped at 100")
	assert.InDelta(t, 1, sampleCount(t, "factory_job_time_to_merge_seconds")-merges, 0)
}

func TestAJobFinishedWithoutTimingsExportsNone(t *testing.T) {
	env := browserJobEnv(t)
	job := env.runningJob(t)
	phases := sampleCount(t, "factory_job_phase_seconds")

	_, err := env.runner.CompleteJob(bearer(env.token),
		&factoryv1.CompleteJobRequest{JobId: job.Id, Succeeded: true})
	require.NoError(t, err)
	assert.InDelta(t, phases, sampleCount(t, "factory_job_phase_seconds"), 0)
}

func TestAFailedJobsTimingsSkipTimeToMerge(t *testing.T) {
	env := browserJobEnv(t)
	job := env.runningJob(t)
	merges := sampleCount(t, "factory_job_time_to_merge_seconds")

	_, err := env.runner.CompleteJob(bearer(env.token), &factoryv1.CompleteJobRequest{
		JobId: job.Id, Succeeded: false, Timings: &factoryv1.JobTimings{TotalSeconds: 9},
	})
	require.NoError(t, err)
	assert.InDelta(t, merges, sampleCount(t, "factory_job_time_to_merge_seconds"), 0)
}
