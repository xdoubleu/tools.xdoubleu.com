package repositories_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"tools.xdoubleu.com/internal/models"
	"tools.xdoubleu.com/internal/repositories"
)

func clearHostMetrics(t *testing.T) {
	t.Helper()
	_, err := testDB.Exec(t.Context(), "DELETE FROM global.host_metric_samples")
	require.NoError(t, err)
}

func TestHostMetricsInsertAndSince(t *testing.T) {
	clearHostMetrics(t)
	repo := repositories.NewHostMetricsRepository(testDB)
	now := time.Now()

	require.NoError(t, repo.Insert(t.Context(), models.HostMetricSample{
		SampledAt: now, CPUPercent: 10, MemoryPercent: 20, DiskPercent: 30,
	}))
	require.NoError(t, repo.Insert(t.Context(), models.HostMetricSample{
		SampledAt: now.Add(time.Minute), CPUPercent: 15, MemoryPercent: 25,
		DiskPercent: 35,
	}))

	samples, err := repo.Since(t.Context(), now.Add(-time.Hour))
	require.NoError(t, err)
	require.Len(t, samples, 2)
	assert.InDelta(t, 10, samples[0].CPUPercent, 0.001)
	assert.InDelta(t, 15, samples[1].CPUPercent, 0.001)
}

func TestHostMetricsPruneOlderThan(t *testing.T) {
	clearHostMetrics(t)
	repo := repositories.NewHostMetricsRepository(testDB)

	old := time.Now().AddDate(0, 0, -60)
	require.NoError(t, repo.Insert(t.Context(), models.HostMetricSample{
		SampledAt: old, CPUPercent: 1, MemoryPercent: 1, DiskPercent: 1,
	}))
	require.NoError(t, repo.Insert(t.Context(), models.HostMetricSample{
		SampledAt: time.Now(), CPUPercent: 2, MemoryPercent: 2, DiskPercent: 2,
	}))

	require.NoError(
		t, repo.PruneOlderThan(t.Context(), time.Now().AddDate(0, 0, -30)),
	)

	samples, err := repo.Since(t.Context(), time.Now().AddDate(0, 0, -90))
	require.NoError(t, err)
	require.Len(t, samples, 1)
	assert.InDelta(t, 2, samples[0].CPUPercent, 0.001)
}
