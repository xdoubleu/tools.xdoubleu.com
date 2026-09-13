package learningpaths_test

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"golang.org/x/oauth2"

	"tools.xdoubleu.com/apps/learningpaths/internal/models"
	"tools.xdoubleu.com/apps/learningpaths/internal/repositories"
	"tools.xdoubleu.com/internal/crypto"
	sharedmodels "tools.xdoubleu.com/internal/models"
)

// TestLearningPathsRepository_ErrorPropagation exercises the
// postgres.PgxErrorToHTTPError error branch of every repository method —
// hard to reach with a valid query, so this uses an already-canceled
// context to force each query to fail fast.
func TestLearningPathsRepository_ErrorPropagation(t *testing.T) {
	testSealer, err := crypto.New(testCfg.EncryptionKey)
	assert.NoError(t, err)
	repos := repositories.New(testDB, testSealer)
	repo := repos.LearningPaths

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, _, err = repo.ListForUser(ctx, userID, 10, 0)
	assert.Error(t, err)

	_, err = repo.GetByID(ctx, uuid.New())
	assert.Error(t, err)

	//nolint:exhaustruct //other fields optional
	_, err = repo.Create(ctx, models.LearningPath{UserID: userID, Title: "x"})
	assert.Error(t, err)

	//nolint:exhaustruct //other fields optional
	err = repo.Update(ctx, models.LearningPath{ID: uuid.New(), UserID: userID})
	assert.Error(t, err)

	err = repo.Delete(ctx, uuid.New(), userID)
	assert.Error(t, err)

	//nolint:exhaustruct //other fields optional
	err = repo.ReplaceModules(ctx, uuid.New(), []models.Module{{Title: "M1"}})
	assert.Error(t, err)

	_, err = repo.GetModules(ctx, uuid.New())
	assert.Error(t, err)

	//nolint:exhaustruct //other fields optional
	err = repo.ReplaceResources(ctx, uuid.New(), []models.Resource{{Text: "x"}})
	assert.Error(t, err)

	_, err = repo.GetResources(ctx, uuid.New())
	assert.Error(t, err)

	err = repo.RecordItemProgress(ctx, uuid.New(), userID, true)
	assert.Error(t, err)

	_, err = repo.GetItemForUser(ctx, uuid.New(), userID)
	assert.Error(t, err)
}

// TestOAuthConnectionsRepository_ErrorPropagation exercises the
// postgres.PgxErrorToHTTPError error branch of the per-user Todoist OAuth
// connections repository (issue #1475), the same way as above.
func TestOAuthConnectionsRepository_ErrorPropagation(t *testing.T) {
	testSealer, err := crypto.New(testCfg.EncryptionKey)
	assert.NoError(t, err)
	repo := repositories.New(testDB, testSealer).OAuthConnections

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, _, err = repo.Get(ctx, userID, sharedmodels.OAuthProviderTodoist)
	assert.Error(t, err)

	//nolint:exhaustruct //other fields unused by this test
	err = repo.Upsert(
		ctx, userID, sharedmodels.OAuthProviderTodoist, &oauth2.Token{AccessToken: "x"},
	)
	assert.Error(t, err)

	//nolint:exhaustruct //other fields unused by this test
	err = repo.UpdateToken(
		ctx, userID, sharedmodels.OAuthProviderTodoist, &oauth2.Token{AccessToken: "x"},
	)
	assert.Error(t, err)

	err = repo.Delete(ctx, userID, sharedmodels.OAuthProviderTodoist)
	assert.Error(t, err)

	_, err = repo.GetStatus(ctx, userID, sharedmodels.OAuthProviderTodoist)
	assert.Error(t, err)
}
