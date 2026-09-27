package learningpaths_test

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"tools.xdoubleu.com/apps/learningpaths/internal/repositories"
	"tools.xdoubleu.com/internal/crypto"
	"tools.xdoubleu.com/internal/database"
)

// seedPath adds a path with one module and one item for owner, returning the
// path and item ids.
func seedPath(t *testing.T, owner string) (uuid.UUID, uuid.UUID) {
	t.Helper()
	ctx := context.Background()

	var pathID uuid.UUID
	require.NoError(t, testDB.QueryRow(ctx, `
		INSERT INTO learningpaths.learning_paths (user_id, title)
		VALUES ($1, $2) RETURNING id`,
		owner, "Learn Go",
	).Scan(&pathID))

	var moduleID uuid.UUID
	require.NoError(t, testDB.QueryRow(ctx, `
		INSERT INTO learningpaths.modules (learning_path_id, title, sort_order)
		VALUES ($1, 'M1', 0) RETURNING id`,
		pathID,
	).Scan(&moduleID))

	var itemID uuid.UUID
	require.NoError(t, testDB.QueryRow(ctx, `
		INSERT INTO learningpaths.items (module_id, type, description, sort_order)
		VALUES ($1, 'do', 'practice', 0) RETURNING id`,
		moduleID,
	).Scan(&itemID))
	return pathID, itemID
}

// TestTodoistTaskID_RoundTrip covers SetItemTodoistTaskID, the task id flowing
// back through GetModules, and GetPathIDForItem ownership.
func TestTodoistTaskID_RoundTrip(t *testing.T) {
	testSealer, err := crypto.New(testCfg.EncryptionKey)
	require.NoError(t, err)
	repo := repositories.New(testDB, testSealer).LearningPaths

	pathID, itemID := seedPath(t, "todoist-roundtrip-user")
	ctx := t.Context()

	require.NoError(t, repo.SetItemTodoistTaskID(ctx, itemID, "task-1"))

	modules, err := repo.GetModules(ctx, pathID)
	require.NoError(t, err)
	assert.Equal(t, 1, len(modules))
	assert.Equal(t, "task-1", *modules[0].Items[0].TodoistTaskID)

	gotPathID, err := repo.GetPathIDForItem(ctx, itemID, "todoist-roundtrip-user")
	require.NoError(t, err)
	assert.Equal(t, pathID, gotPathID)

	// Clearing the id rounds the pipeline back to "no task expected".
	require.NoError(t, repo.SetItemTodoistTaskID(ctx, itemID, ""))
	modules, err = repo.GetModules(ctx, pathID)
	require.NoError(t, err)
	assert.Empty(t, *modules[0].Items[0].TodoistTaskID)
}

func TestGetPathIDForItem_ForeignUserNotFound(t *testing.T) {
	testSealer, err := crypto.New(testCfg.EncryptionKey)
	require.NoError(t, err)
	repo := repositories.New(testDB, testSealer).LearningPaths

	_, itemID := seedPath(t, "todoist-owner")
	ctx := t.Context()

	_, err = repo.GetPathIDForItem(ctx, itemID, "someone-else")
	assert.ErrorIs(t, err, database.ErrResourceNotFound)
}

// TestGetModules_UnsetTaskIDIsNil confirms a freshly-inserted item (no task
// id) reports nil rather than "", and completion round-trips through the
// extended item scan.
func TestGetModules_UnsetTaskIDIsNil(t *testing.T) {
	testSealer, err := crypto.New(testCfg.EncryptionKey)
	require.NoError(t, err)
	repo := repositories.New(testDB, testSealer).LearningPaths

	pathID, itemID := seedPath(t, "todoist-unset-user")
	ctx := t.Context()

	require.NoError(t, repo.RecordItemProgress(ctx, itemID, "todoist-unset-user", true))

	modules, err := repo.GetModules(ctx, pathID)
	require.NoError(t, err)
	assert.True(t, modules[0].Items[0].Completed)
	assert.Nil(t, modules[0].Items[0].TodoistTaskID)
}
