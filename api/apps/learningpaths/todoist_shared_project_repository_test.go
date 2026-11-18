package learningpaths_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"tools.xdoubleu.com/apps/learningpaths/internal/repositories"
	"tools.xdoubleu.com/internal/kit/crypto"
)

func TestTodoistSharedProjectAndSections_RoundTrip(t *testing.T) {
	testSealer, err := crypto.New(testCfg.EncryptionKey)
	require.NoError(t, err)
	repo := repositories.New(testDB, testSealer).LearningPaths
	ctx := t.Context()
	owner := "todoist-shared-user"

	projectID, err := repo.GetUserTodoistProjectID(ctx, owner)
	require.NoError(t, err)
	assert.Empty(t, projectID)

	require.NoError(t, repo.SetUserTodoistProjectID(ctx, owner, "proj-1"))
	require.NoError(t, repo.SetUserTodoistProjectID(ctx, owner, "proj-2"))
	projectID, err = repo.GetUserTodoistProjectID(ctx, owner)
	require.NoError(t, err)
	assert.Equal(t, "proj-2", projectID)

	first, _ := seedPath(t, owner)
	second, _ := seedPath(t, owner)
	require.NoError(t, repo.SetTodoistSectionID(ctx, second, "sec-2"))

	paths, err := repo.ListForSync(ctx, owner)
	require.NoError(t, err)
	require.Len(t, paths, 2)
	assert.Equal(t, first, paths[0].ID)
	assert.Nil(t, paths[0].TodoistSectionID)
	assert.Equal(t, second, paths[1].ID)
	require.NotNil(t, paths[1].TodoistSectionID)
	assert.Equal(t, "sec-2", *paths[1].TodoistSectionID)

	require.NoError(t, repo.SetTodoistSectionID(ctx, second, ""))
	lp, err := repo.GetByID(ctx, second)
	require.NoError(t, err)
	assert.Nil(t, lp.TodoistSectionID)
}

func TestTodoistUserIDs_ListsConnectedUsers(t *testing.T) {
	seedTodoistConnection(t, "data:read_write,data:delete,project:delete")
	testSealer, err := crypto.New(testCfg.EncryptionKey)
	require.NoError(t, err)
	repo := repositories.New(testDB, testSealer).OAuthConnections

	userIDs, err := repo.TodoistUserIDs(t.Context())
	require.NoError(t, err)
	assert.Contains(t, userIDs, userID)
}
