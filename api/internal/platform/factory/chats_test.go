package factory_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"tools.xdoubleu.com/internal/platform/factory/internal/models"
	"tools.xdoubleu.com/internal/platform/factory/internal/repositories"
	"tools.xdoubleu.com/internal/platform/factory/internal/services"
)

type chatEnv struct {
	svc      *services.Services
	repoID   uuid.UUID
	runnerID uuid.UUID
	admin    models.Actor
}

// newChatEnv is one online runner with room for maxJobs jobs, and one repo.
func newChatEnv(t *testing.T, maxJobs int) chatEnv {
	t.Helper()
	ctx := context.Background()
	cleanRunners(t)
	for _, table := range []string{"chat_sessions", "jobs"} {
		_, err := testDB.Exec(ctx, "DELETE FROM factory."+table)
		require.NoError(t, err)
	}
	var e chatEnv
	require.NoError(t, testDB.QueryRow(ctx, `
		INSERT INTO factory.repos (owner, name, default_branch)
		VALUES ('acme', 'chat-' || gen_random_uuid(), 'main') RETURNING id`).Scan(&e.repoID))
	require.NoError(t, testDB.QueryRow(ctx, `
		INSERT INTO factory.runners (
			name, created_by, token_hash, paired_at, last_seen, max_jobs
		)
		VALUES ('Mac', 'human:x', 'hash', now(), now(), $1) RETURNING id`, maxJobs,
	).Scan(&e.runnerID))

	r := repositories.New(testDB)
	e.svc = services.New(services.Stores{
		Tasks: r.Tasks, Repos: r.Repos, Runners: r.Runners, Jobs: r.Jobs,
		Events: r.Events, Settings: r.Settings, Linked: r.Linked,
		Routines: r.Routines, Chats: r.Chats,
	}, nil, nil, nil, "", "")
	e.admin.Kind, e.admin.ID, e.admin.Browser, e.admin.Admin =
		models.ActorHuman, "u", true, true
	return e
}

func (e chatEnv) start(t *testing.T) (*models.ChatSession, error) {
	t.Helper()
	var in services.StartChat
	in.RepoID, in.RunnerID, in.Harness = e.repoID, e.runnerID, models.HarnessClaudeCode
	return e.svc.Chats.Start(context.Background(), e.admin, in)
}

func (e chatEnv) jobStatus(t *testing.T, id uuid.UUID) (string, string) {
	t.Helper()
	var kind, status string
	require.NoError(t, testDB.QueryRow(context.Background(),
		`SELECT kind, status FROM factory.jobs WHERE id = $1`, id).Scan(&kind, &status))
	return kind, status
}

func TestChatStartCountsTowardTheRunnersJobs(t *testing.T) {
	e := newChatEnv(t, 1)

	s, err := e.start(t)
	require.NoError(t, err)

	kind, status := e.jobStatus(t, s.JobID)
	assert.Equal(t, "chat", kind)
	assert.Equal(t, "running", status)
	_, err = e.start(t)
	assert.ErrorIs(t, err, services.ErrConflict, "the one slot is held")

	claimed, err := e.svc.Jobs.ClaimQueued(
		context.Background(), e.runnerID, 1, models.HarnessClaudeCode, "")
	require.NoError(t, err)
	assert.Nil(t, claimed, "a chat is never claimed as a queued job")
}

func TestChatStopFreesTheSlotAndSealsTheTranscript(t *testing.T) {
	e := newChatEnv(t, 1)
	ctx := context.Background()
	s, err := e.start(t)
	require.NoError(t, err)

	stopped, err := e.svc.Chats.Stop(ctx, e.admin, s.ID, "branch left behind")
	require.NoError(t, err)

	assert.Equal(t, models.ChatStopped, stopped.State)
	assert.Equal(t, "branch left behind", stopped.Report)
	assert.NotNil(t, stopped.StoppedAt)
	_, status := e.jobStatus(t, s.JobID)
	assert.Equal(t, "succeeded", status)
	events, err := e.svc.Chats.Events(ctx, e.admin, s.ID, 0, 0)
	require.NoError(t, err)
	require.Len(t, events, 1)
	assert.Equal(t, models.ChatEventStopped, events[0].Kind)
	assert.Equal(t, "branch left behind", events[0].Payload)
	err = e.svc.Chats.RecordEvents(ctx, e.runnerID, s.ID, "",
		[]models.ChatEventInput{{Kind: "assistant", Payload: "late"}})
	assert.ErrorIs(t, err, services.ErrConflict)
	_, err = e.start(t)
	assert.NoError(t, err, "the slot is free again")
}

func TestChatEventsAreNumberedInOrderAcrossCalls(t *testing.T) {
	e := newChatEnv(t, 2)
	ctx := context.Background()
	s, err := e.start(t)
	require.NoError(t, err)

	require.NoError(t, e.svc.Chats.RecordEvents(ctx, e.runnerID, s.ID,
		models.ChatWorking, []models.ChatEventInput{
			{Kind: "assistant", Payload: "a"}, {Kind: "tool", Payload: "b"},
		}))
	require.NoError(t, e.svc.Chats.RecordEvents(ctx, e.runnerID, s.ID, models.ChatAsking,
		[]models.ChatEventInput{{Kind: "question", Payload: "c"}}))
	require.NoError(t, e.svc.Chats.Answer(ctx, e.admin, s.ID, "d"))

	all, err := e.svc.Chats.Events(ctx, e.admin, s.ID, 0, 0)
	require.NoError(t, err)
	payloads := []string{}
	for i, ev := range all {
		assert.Equal(t, i+1, ev.Seq)
		payloads = append(payloads, ev.Payload)
	}
	assert.Equal(t, []string{"a", "b", "c", "d"}, payloads)
	tail, err := e.svc.Chats.Events(ctx, e.admin, s.ID, 3, 0)
	require.NoError(t, err)
	require.Len(t, tail, 1)
	assert.Equal(t, "d", tail[0].Payload)
	got, err := e.svc.Chats.Get(ctx, e.admin, s.ID)
	require.NoError(t, err)
	assert.Equal(t, 4, got.LastSeq)
	assert.Equal(t, models.ChatWorking, got.State)
}

func TestChatListNewestFirstWithoutStopped(t *testing.T) {
	e := newChatEnv(t, 3)
	ctx := context.Background()
	first, err := e.start(t)
	require.NoError(t, err)
	second, err := e.start(t)
	require.NoError(t, err)
	_, err = e.svc.Chats.Stop(ctx, e.admin, first.ID, "")
	require.NoError(t, err)

	all, err := e.svc.Chats.List(ctx, e.admin, false)
	require.NoError(t, err)
	active, err := e.svc.Chats.List(ctx, e.admin, true)
	require.NoError(t, err)

	require.Len(t, all, 2)
	assert.Equal(t, second.ID, all[0].ID)
	require.Len(t, active, 1)
	assert.Equal(t, second.ID, active[0].ID)
}

func TestLostChatJobStopsItsSession(t *testing.T) {
	e := newChatEnv(t, 1)
	ctx := context.Background()
	s, err := e.start(t)
	require.NoError(t, err)
	_, err = testDB.Exec(ctx, `UPDATE factory.jobs SET last_check_in = $2 WHERE id = $1`,
		s.JobID, time.Now().Add(-time.Hour))
	require.NoError(t, err)

	require.NoError(t, e.svc.Jobs.ExpireLost(ctx))

	got, err := e.svc.Chats.Get(ctx, e.admin, s.ID)
	require.NoError(t, err)
	assert.Equal(t, models.ChatStopped, got.State)
	assert.Contains(t, got.Report, "stopped checking in")
	_, status := e.jobStatus(t, s.JobID)
	assert.Equal(t, "lost", status)
}

func TestChatFeedAndForRunnerCheckWhoAsks(t *testing.T) {
	e := newChatEnv(t, 1)
	ctx := context.Background()
	s, err := e.start(t)
	require.NoError(t, err)
	require.NoError(t, e.svc.Chats.RecordEvents(ctx, e.runnerID, s.ID, "",
		[]models.ChatEventInput{{Kind: "assistant", Payload: "hi"}}))

	session, events, err := e.svc.Chats.Feed(ctx, s.ID, 0, 0)
	require.NoError(t, err)
	assert.Equal(t, s.ID, session.ID)
	require.Len(t, events, 1)
	_, events, err = e.svc.Chats.Feed(ctx, s.ID, 1, 0)
	require.NoError(t, err)
	assert.Empty(t, events)
	_, _, err = e.svc.Chats.Feed(ctx, uuid.New(), 0, 0)
	assert.Error(t, err)

	own, err := e.svc.Chats.ForRunner(ctx, e.runnerID, s.ID)
	require.NoError(t, err)
	assert.Equal(t, s.ID, own.ID)
	_, err = e.svc.Chats.ForRunner(ctx, uuid.New(), s.ID)
	assert.ErrorIs(t, err, services.ErrForbidden)
	_, err = e.svc.Chats.ForRunner(ctx, e.runnerID, uuid.New())
	assert.Error(t, err)
}
