package todos_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/xdoubleu/essentia/v4/pkg/logging"
	"tools.xdoubleu.com/apps/todos/internal/jobs"
	"tools.xdoubleu.com/apps/todos/internal/repositories"
)

func TestArchiveJob_NothingToDo(t *testing.T) {
	repos := repositories.New(testDB)
	job := jobs.NewArchiveJob(repos.Tasks)
	assert.Equal(t, "todos-archive", job.ID())
	assert.NotZero(t, job.RunEvery())

	err := job.Run(context.Background(), logging.NewNopLogger())
	assert.NoError(t, err)
}

func TestArchiveJob_ArchivesDoneTasks(t *testing.T) {
	// Insert a done task whose completed_at exceeds the archive threshold.
	_, err := testDB.Exec(t.Context(), `
		INSERT INTO todos.archive_settings (user_id, archive_after_hours)
		VALUES ($1, 1) ON CONFLICT (user_id) DO UPDATE SET archive_after_hours = 1`,
		userID,
	)
	require.NoError(t, err)

	_, err = testDB.Exec(t.Context(), `
		INSERT INTO todos.tasks (owner_user_id, title, status, completed_at)
		VALUES ($1, 'Old done task', 'done', now() - interval '2 hours')`,
		userID,
	)
	require.NoError(t, err)

	repos := repositories.New(testDB)
	job := jobs.NewArchiveJob(repos.Tasks)
	err = job.Run(context.Background(), logging.NewNopLogger())
	require.NoError(t, err)

	var count int
	err = testDB.QueryRow(t.Context(), `
		SELECT COUNT(*) FROM todos.tasks
		WHERE owner_user_id = $1 AND status = 'archived'`, userID,
	).Scan(&count)
	require.NoError(t, err)
	assert.Positive(t, count)
}
