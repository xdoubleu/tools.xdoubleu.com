package factory_test

import (
	"context"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"tools.xdoubleu.com/internal/kit/database"
	"tools.xdoubleu.com/internal/platform/factory/internal/models"
	"tools.xdoubleu.com/internal/platform/factory/internal/repositories"
)

func startedChat(t *testing.T) (*repositories.ChatsRepository, *models.ChatSession) {
	t.Helper()
	ctx := context.Background()
	browserJobEnv(t)
	var runnerID uuid.UUID
	require.NoError(t, testDB.QueryRow(ctx,
		`SELECT id FROM factory.runners WHERE name = 'Home Mac'`).Scan(&runnerID))
	repoID, err := uuid.Parse(seededRepoID(t))
	require.NoError(t, err)
	var in models.ChatSession
	in.RepoID, in.RunnerID, in.Harness = repoID, runnerID, models.HarnessOpenCode
	in.Branch = "agent/chat-life0001"
	repo := repositories.New(testDB).Chats
	chat, err := repo.Start(ctx, in, "human:u")
	require.NoError(t, err)
	return repo, chat
}

func jobStatus(t *testing.T, id uuid.UUID) string {
	t.Helper()
	var status string
	require.NoError(t, testDB.QueryRow(context.Background(),
		`SELECT status FROM factory.jobs WHERE id = $1`, id).Scan(&status))
	return status
}

func TestAChatPausesFreesItsJobAndReopensOnANewOne(t *testing.T) {
	ctx := context.Background()
	repo, chat := startedChat(t)

	paused, err := repo.Pause(ctx, chat.ID)
	require.NoError(t, err)
	assert.Equal(t, models.ChatPaused, paused.State)
	assert.Equal(t, "succeeded", jobStatus(t, chat.JobID))
	_, err = repo.Pause(ctx, chat.ID)
	require.ErrorIs(t, err, database.ErrResourceConflict, "only an idle chat pauses")

	reopened, err := repo.Reopen(ctx, chat.ID, "human:u")
	require.NoError(t, err)
	assert.Equal(t, models.ChatWorking, reopened.State)
	assert.NotEqual(t, chat.JobID, reopened.JobID)
	assert.Equal(t, "running", jobStatus(t, reopened.JobID))
	_, err = repo.Reopen(ctx, chat.ID, "human:u")
	require.ErrorIs(t, err, database.ErrResourceConflict, "only a paused chat reopens")
}

func TestReopenRefusesARunnerAtItsJobCap(t *testing.T) {
	ctx := context.Background()
	repo, chat := startedChat(t)
	_, err := repo.Pause(ctx, chat.ID)
	require.NoError(t, err)
	_, err = testDB.Exec(ctx,
		`UPDATE factory.runners SET max_jobs = 0 WHERE id = $1`, chat.RunnerID)
	require.NoError(t, err)

	_, err = repo.Reopen(ctx, chat.ID, "human:u")

	require.ErrorIs(t, err, database.ErrResourceConflict)
}

func TestTheListKeepsTheLastPersonMessageNewestFirst(t *testing.T) {
	ctx := context.Background()
	repo, chat := startedChat(t)
	long := strings.Repeat("é", 300)
	require.NoError(t, repo.Append(ctx, chat.ID, []models.ChatEventInput{
		{Kind: models.ChatEventMessage, Payload: "first"},
		{Kind: "update", Payload: "{}"},
		{Kind: models.ChatEventAnswer, Payload: long},
	}))

	list, err := repo.List(ctx, false, 10)

	require.NoError(t, err)
	require.NotEmpty(t, list)
	assert.Equal(t, chat.ID, list[0].ID)
	assert.Equal(t, strings.Repeat("é", 200), list[0].LastMessage)
}

func TestPurgeClosedDeletesOldEventsAndKeepsTheSummary(t *testing.T) {
	ctx := context.Background()
	repo, chat := startedChat(t)
	require.NoError(t, repo.Append(ctx, chat.ID, []models.ChatEventInput{
		{Kind: models.ChatEventMessage, Payload: "hello"},
	}))
	_, err := repo.Stop(ctx, chat.ID, "left 2 files", models.JobSucceeded)
	require.NoError(t, err)

	cutoff := chat.CreatedAt.AddDate(0, 0, -30)
	purged, err := repo.PurgeClosed(ctx, cutoff)
	require.NoError(t, err)
	assert.Zero(t, purged, "closed after the cutoff")

	_, err = testDB.Exec(ctx, `UPDATE factory.chat_sessions
		SET stopped_at = now() - interval '31 days' WHERE id = $1`, chat.ID)
	require.NoError(t, err)
	purged, err = repo.PurgeClosed(ctx, cutoff)
	require.NoError(t, err)
	assert.Equal(t, 1, purged)

	events, err := repo.After(ctx, chat.ID, 0, 10)
	require.NoError(t, err)
	assert.Empty(t, events)
	got, err := repo.Get(ctx, chat.ID)
	require.NoError(t, err)
	assert.Equal(t, "left 2 files", got.Report)
	assert.Equal(t, "hello", got.LastMessage)
	purged, err = repo.PurgeClosed(ctx, cutoff)
	require.NoError(t, err)
	assert.Zero(t, purged, "purged once")
}
