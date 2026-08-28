package repositories_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"tools.xdoubleu.com/internal/repositories"
)

func TestDBStatsTotalSize(t *testing.T) {
	repo := repositories.NewDBStatsRepository(testDB)

	size, err := repo.TotalSize(t.Context())
	require.NoError(t, err)
	assert.Positive(t, size)
}

func TestDBStatsSchemaSizes(t *testing.T) {
	repo := repositories.NewDBStatsRepository(testDB)

	schemas, err := repo.SchemaSizes(t.Context())
	require.NoError(t, err)

	var hasGlobal bool
	for _, s := range schemas {
		if s.Name == "global" {
			hasGlobal = true
		}
	}
	assert.True(t, hasGlobal)
}

func TestDBStatsTableSizes(t *testing.T) {
	repo := repositories.NewDBStatsRepository(testDB)

	tables, err := repo.TableSizes(t.Context())
	require.NoError(t, err)

	var hasJobRuns bool
	for _, tbl := range tables {
		if tbl.SchemaName == "global" && tbl.TableName == "job_runs" {
			hasJobRuns = true
		}
	}
	assert.True(t, hasJobRuns)
}
