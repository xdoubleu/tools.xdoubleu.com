package learningpaths_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"tools.xdoubleu.com/apps/learningpaths/internal/models"
	"tools.xdoubleu.com/apps/learningpaths/internal/repositories"
	"tools.xdoubleu.com/internal/crypto"
)

func TestTodoistProjectAndDue_RoundTrip(t *testing.T) {
	testSealer, err := crypto.New(testCfg.EncryptionKey)
	require.NoError(t, err)
	repo := repositories.New(testDB, testSealer).LearningPaths
	ctx := t.Context()

	pathID, _ := seedPath(t, "todoist-project-user")

	lp, err := repo.GetByID(ctx, pathID)
	require.NoError(t, err)
	assert.Nil(t, lp.TodoistProjectID)

	//nolint:exhaustruct //fixture sets only the fields under test
	modules := []models.Module{{
		Title: "M1",
		Items: []models.Item{{Description: "read", Due: "every day at 20:00"}},
	}}
	require.NoError(t, repo.ReplaceModules(ctx, pathID, modules))
	require.NoError(t, repo.SetTodoistProjectID(ctx, pathID, "proj-1"))

	lp, err = repo.GetByID(ctx, pathID)
	require.NoError(t, err)
	require.NotNil(t, lp.TodoistProjectID)
	assert.Equal(t, "proj-1", *lp.TodoistProjectID)

	got, err := repo.GetModules(ctx, pathID)
	require.NoError(t, err)
	assert.Equal(t, "every day at 20:00", got[0].Items[0].Due)

	require.NoError(t, repo.SetTodoistProjectID(ctx, pathID, ""))
	lp, err = repo.GetByID(ctx, pathID)
	require.NoError(t, err)
	assert.Nil(t, lp.TodoistProjectID)
}
