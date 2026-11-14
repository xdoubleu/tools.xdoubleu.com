package main

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"tools.xdoubleu.com/factory-runner/internal/runner"
)

func TestConfigMaxJobsIsSavedAndBounded(t *testing.T) {
	e := newEnv(t)
	assert.Equal(t, runner.DefaultMaxJobs, e.app.maxJobs())

	require.Equal(t, 0, e.do("config", "max-jobs", "3"))
	assert.Equal(t, 3, e.app.maxJobs())
	require.NoError(t, e.app.setMaxJobsTo(4))
	assert.Equal(t, 4, e.app.maxJobs())
	require.Error(t, e.app.setMaxJobsTo(0))

	assert.Equal(t, 1, e.do("config", "max-jobs", "many"))
	assert.Equal(t, 1, e.do("config", "max-jobs", "99"))
	e.out.Reset()
	require.Equal(t, 0, e.do("config", "show"))
	assert.Contains(t, e.out.String(), "max-jobs: 4")
}

func TestConfigOnLimitDefaultsToWaitAndIsSaved(t *testing.T) {
	e := newEnv(t)
	e.out.Reset()
	require.Equal(t, 0, e.do("config", "show"))
	assert.Contains(t, e.out.String(), "on-limit: wait")

	require.Equal(t, 0, e.do("config", "on-limit", "fallback"))
	assert.Equal(t, 1, e.do("config", "on-limit", "never"))
	e.out.Reset()
	require.Equal(t, 0, e.do("config", "show"))
	assert.Contains(t, e.out.String(), "on-limit: fallback")
}
