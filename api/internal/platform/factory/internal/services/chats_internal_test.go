package services

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"tools.xdoubleu.com/internal/kit/database"
	"tools.xdoubleu.com/internal/platform/factory/internal/models"
)

// memChats is an in-memory ChatStore with the rules the repository enforces.
type memChats struct {
	sessions map[uuid.UUID]*models.ChatSession
	events   map[uuid.UUID][]models.ChatEvent
	started  []models.ChatSession
	claimed  map[uuid.UUID]bool
	full     bool
}

func newMemChats() *memChats {
	m := &memChats{
		sessions: map[uuid.UUID]*models.ChatSession{},
		events:   map[uuid.UUID][]models.ChatEvent{},
		started:  nil, claimed: map[uuid.UUID]bool{}, full: false,
	}
	return m
}

func (m *memChats) Start(
	_ context.Context, c models.ChatSession, createdBy string,
) (*models.ChatSession, error) {
	if m.full {
		return nil, database.ErrResourceConflict
	}
	c.ID, c.JobID, c.CreatedBy = uuid.New(), uuid.New(), createdBy
	c.State = models.ChatIdle
	m.sessions[c.ID] = &c
	m.started = append(m.started, c)
	out := c
	return &out, nil
}

func (m *memChats) Get(_ context.Context, id uuid.UUID) (*models.ChatSession, error) {
	c, ok := m.sessions[id]
	if !ok {
		return nil, database.ErrResourceNotFound
	}
	out := *c
	return &out, nil
}

func (m *memChats) List(
	_ context.Context, activeOnly bool, _ int,
) ([]models.ChatSession, error) {
	out := []models.ChatSession{}
	for _, c := range m.sessions {
		if !activeOnly || c.State != models.ChatStopped {
			out = append(out, *c)
		}
	}
	return out, nil
}

func (m *memChats) Transition(
	_ context.Context, id uuid.UUID, from []models.ChatState, to models.ChatState,
) (*models.ChatSession, error) {
	c, ok := m.sessions[id]
	if !ok {
		return nil, database.ErrResourceNotFound
	}
	for _, f := range from {
		if c.State == f {
			c.State = to
			out := *c
			return &out, nil
		}
	}
	return nil, database.ErrResourceConflict
}

func (m *memChats) Append(
	_ context.Context, id uuid.UUID, events []models.ChatEventInput,
) error {
	c, ok := m.sessions[id]
	if !ok || c.State == models.ChatStopped {
		return database.ErrResourceConflict
	}
	for _, e := range events {
		c.LastSeq++
		m.events[id] = append(m.events[id], models.ChatEvent{
			SessionID: id, Seq: c.LastSeq, Kind: e.Kind, Payload: e.Payload,
			CreatedAt: time.Time{},
		})
	}
	return nil
}

func (m *memChats) After(
	_ context.Context, id uuid.UUID, afterSeq, limit int,
) ([]models.ChatEvent, error) {
	out := []models.ChatEvent{}
	for _, e := range m.events[id] {
		if e.Seq > afterSeq && len(out) < limit {
			out = append(out, e)
		}
	}
	return out, nil
}

func (m *memChats) Ask(
	ctx context.Context, id uuid.UUID, payload string,
) (int, error) {
	if err := m.Append(ctx, id, []models.ChatEventInput{
		{Kind: models.ChatEventAsk, Payload: payload},
	}); err != nil {
		return 0, err
	}
	m.sessions[id].State = models.ChatAsking
	return m.sessions[id].LastSeq, nil
}

func (m *memChats) ForJob(
	_ context.Context, jobID uuid.UUID,
) (*models.ChatSession, error) {
	for _, c := range m.sessions {
		if c.JobID == jobID {
			out := *c
			return &out, nil
		}
	}
	return nil, database.ErrResourceNotFound
}

func (m *memChats) Stop(
	_ context.Context, id uuid.UUID, report string, _ models.JobStatus,
) (*models.ChatSession, error) {
	c, ok := m.sessions[id]
	if !ok || c.State == models.ChatStopped {
		return nil, database.ErrResourceConflict
	}
	c.State, c.Report = models.ChatStopped, report
	out := *c
	return &out, nil
}

func (m *memChats) StopForJob(
	ctx context.Context, jobID uuid.UUID, report string,
) (*models.ChatSession, error) {
	for id, c := range m.sessions {
		if c.JobID == jobID {
			return m.Stop(ctx, id, report, models.JobLost)
		}
	}
	return nil, database.ErrResourceNotFound
}

func (m *memChats) ClaimFor(
	_ context.Context, runnerID uuid.UUID,
) (*models.ChatSession, error) {
	for _, c := range m.sessions {
		if c.RunnerID == runnerID && !m.claimed[c.ID] && c.State != models.ChatStopped {
			m.claimed[c.ID] = true
			out := *c
			return &out, nil
		}
	}
	return nil, database.ErrResourceNotFound
}

func (m *memChats) AppendReport(_ context.Context, id uuid.UUID, text string) error {
	c, ok := m.sessions[id]
	if !ok {
		return database.ErrResourceNotFound
	}
	if c.Report != "" {
		c.Report += "\n\n"
	}
	c.Report += text
	return nil
}

type chatRepos struct {
	RepoStore
	repo models.Repo
}

func (r chatRepos) Get(context.Context, uuid.UUID) (*models.Repo, error) {
	return &r.repo, nil
}

type chatTasks struct {
	TaskStore
	task     models.Task
	comments *[]string
}

func (t chatTasks) AddComment(
	_ context.Context, _ uuid.UUID, author, body string,
) (*models.Comment, error) {
	*t.comments = append(*t.comments, author+": "+body)
	return nil, nil //nolint:nilnil // unused by the service
}

func (t chatTasks) Get(context.Context, uuid.UUID) (*models.Task, error) {
	return &t.task, nil
}

type chatRunners struct {
	RunnerStore
	runners []models.Runner
}

func (r chatRunners) List(context.Context) ([]models.Runner, error) {
	return r.runners, nil
}

func chatActor(kind models.ActorKind, browser, admin bool) models.Actor {
	var a models.Actor
	a.Kind, a.ID, a.Browser, a.Admin = kind, "u1", browser, admin
	return a
}

func startIn(repoID, runnerID uuid.UUID, h models.Harness) StartChat {
	var in StartChat
	in.RepoID, in.RunnerID, in.Harness = repoID, runnerID, h
	return in
}

func runnerSeen(id uuid.UUID, paired, seen time.Time) models.Runner {
	var r models.Runner
	r.ID, r.PairedAt, r.LastSeen = id, &paired, &seen
	return r
}

func withRunners(runners ...models.Runner) chatRunners {
	var r chatRunners
	r.runners = runners
	return r
}

func withTask(task models.Task) chatTasks {
	var t chatTasks
	t.task = task
	t.comments = new([]string)
	return t
}

func taskOf(repoID uuid.UUID, number int64, title string) models.Task {
	var t models.Task
	t.ID, t.RepoID, t.Number, t.Title = uuid.New(), repoID, number, title
	return t
}

type chatEnv struct {
	svc    *ChatService
	chats  *memChats
	repo   models.Repo
	runner models.Runner
	admin  models.Actor
}

func newChatEnv(t *testing.T) chatEnv {
	t.Helper()
	now := time.Date(2030, 1, 2, 12, 0, 0, 0, time.UTC)
	runner := runnerSeen(uuid.New(), now.Add(-time.Hour), now.Add(-time.Second))
	var repo models.Repo
	repo.ID = uuid.New()
	chats := newMemChats()
	svc := &ChatService{
		chats: chats, repos: nil, tasks: nil, runners: nil, askPoll: time.Millisecond,
		now: func() time.Time { return now },
	}
	var repos chatRepos
	repos.repo = repo
	svc.repos = repos
	svc.tasks = withTask(taskOf(repo.ID, 7, "Fix it"))
	svc.runners = withRunners(runner)
	return chatEnv{
		svc: svc, chats: chats, repo: repo, runner: runner,
		admin: chatActor(models.ActorHuman, true, true),
	}
}

func (e chatEnv) start(t *testing.T) *models.ChatSession {
	t.Helper()
	s, err := e.svc.Start(context.Background(), e.admin,
		startIn(e.repo.ID, e.runner.ID, models.HarnessClaudeCode))
	require.NoError(t, err)
	return s
}

func TestChatStartPinsTheRunnerAndNamesABranch(t *testing.T) {
	e := newChatEnv(t)

	s := e.start(t)

	assert.Equal(t, e.runner.ID, s.RunnerID)
	assert.Equal(t, models.ChatIdle, s.State)
	assert.True(t, strings.HasPrefix(s.Branch, "agent/chat-"), s.Branch)
	assert.Equal(t, "human:u1", s.CreatedBy)
}

func TestChatStartOnATaskUsesTheTasksBranch(t *testing.T) {
	e := newChatEnv(t)

	in := startIn(e.repo.ID, e.runner.ID, models.HarnessOpenCode)
	in.TaskID = uuid.New()

	s, err := e.svc.Start(context.Background(), e.admin, in)

	require.NoError(t, err)
	assert.Equal(t, "agent/t7-fix-it", s.Branch)
}

func TestChatStartRefusesATaskOfAnotherRepo(t *testing.T) {
	e := newChatEnv(t)
	e.svc.tasks = withTask(taskOf(uuid.New(), 1, "x"))
	in := startIn(e.repo.ID, e.runner.ID, models.HarnessClaudeCode)
	in.TaskID = uuid.New()

	_, err := e.svc.Start(context.Background(), e.admin, in)

	assert.ErrorIs(t, err, ErrInvalid)
}

func TestChatStartRules(t *testing.T) {
	e := newChatEnv(t)
	ctx := context.Background()
	in := startIn(e.repo.ID, e.runner.ID, models.HarnessClaudeCode)

	_, err := e.svc.Start(ctx, chatActor(models.ActorAgent, false, false), in)
	assert.ErrorIs(t, err, ErrForbidden, "agents get no shell")
	_, err = e.svc.Start(ctx, chatActor(models.ActorHuman, false, true), in)
	assert.ErrorIs(t, err, ErrForbidden, "a browser is required")

	bad := in
	bad.Harness = "vim"
	_, err = e.svc.Start(ctx, e.admin, bad)
	assert.ErrorIs(t, err, ErrInvalid)

	bad = in
	bad.RunnerID = uuid.New()
	_, err = e.svc.Start(ctx, e.admin, bad)
	assert.ErrorIs(t, err, database.ErrResourceNotFound, "unknown runner")

	assert.Empty(t, e.chats.started, "nothing was started by a refusal")
}

func TestChatStartNeedsAnOnlineRunnerWithAFreeSlot(t *testing.T) {
	e := newChatEnv(t)
	in := startIn(e.repo.ID, e.runner.ID, models.HarnessClaudeCode)

	stale := e.svc.now().Add(-time.Hour)
	e.svc.runners = withRunners(runnerSeen(e.runner.ID, *e.runner.PairedAt, stale))
	_, err := e.svc.Start(context.Background(), e.admin, in)
	assert.ErrorIs(t, err, ErrConflict, "offline")

	e.svc.runners = withRunners(e.runner)
	e.chats.full = true
	_, err = e.svc.Start(context.Background(), e.admin, in)
	assert.ErrorIs(t, err, ErrConflict, "at its job cap")
}

func TestChatMessageAndAnswerNeedTheStateThatWaitsForThem(t *testing.T) {
	e := newChatEnv(t)
	ctx := context.Background()
	s := e.start(t)

	assert.ErrorIs(t, e.svc.Answer(ctx, e.admin, s.ID, "yes"), ErrConflict,
		"nothing was asked")
	require.NoError(t, e.svc.SendMessage(ctx, e.admin, s.ID, "  hello "))
	assert.ErrorIs(t, e.svc.SendMessage(ctx, e.admin, s.ID, "again"), ErrConflict,
		"the harness is working")

	require.NoError(t, e.svc.RecordEvents(ctx, e.runner.ID, s.ID, models.ChatAsking, nil))
	require.NoError(t, e.svc.Answer(ctx, e.admin, s.ID, "yes"))

	events, err := e.svc.Events(ctx, e.admin, s.ID, 0, 0)
	require.NoError(t, err)
	require.Len(t, events, 2)
	assert.Equal(t, models.ChatEventMessage, events[0].Kind)
	assert.Equal(t, "hello", events[0].Payload)
	assert.Equal(t, models.ChatEventAnswer, events[1].Kind)
	got, err := e.svc.Get(ctx, e.admin, s.ID)
	require.NoError(t, err)
	assert.Equal(t, models.ChatWorking, got.State)
}

func TestChatMessageValidation(t *testing.T) {
	e := newChatEnv(t)
	ctx := context.Background()
	s := e.start(t)

	assert.ErrorIs(t, e.svc.SendMessage(ctx, e.admin, s.ID, " \n"), ErrInvalid)
	assert.ErrorIs(t,
		e.svc.SendMessage(ctx, e.admin, s.ID, strings.Repeat("a", maxChatMessageLen+1)),
		ErrInvalid)
	assert.ErrorIs(t, e.svc.SendMessage(ctx, e.admin, uuid.New(), "hi"),
		database.ErrResourceNotFound)
	assert.ErrorIs(t,
		e.svc.SendMessage(ctx, chatActor(models.ActorAgent, false, false), s.ID, "hi"),
		ErrForbidden)
	assert.Equal(
		t,
		models.ChatIdle,
		e.chats.sessions[s.ID].State,
		"refusals change nothing",
	)
}

func TestChatStopKeepsTheReportAndOnlyOnce(t *testing.T) {
	e := newChatEnv(t)
	ctx := context.Background()
	s := e.start(t)

	stopped, err := e.svc.Stop(ctx, e.admin, s.ID, "  left a branch  ", "")

	require.NoError(t, err)
	assert.Equal(t, models.ChatStopped, stopped.State)
	assert.Equal(t, "left a branch", stopped.Report)
	_, err = e.svc.Stop(ctx, e.admin, s.ID, "", "")
	assert.ErrorIs(t, err, ErrConflict)
	assert.ErrorIs(t, e.svc.SendMessage(ctx, e.admin, s.ID, "hi"), ErrConflict)
	_, err = e.svc.Stop(ctx, e.admin, s.ID, strings.Repeat("a", maxChatReportLen+1), "")
	assert.ErrorIs(t, err, ErrInvalid)
	_, err = e.svc.Stop(ctx, e.admin, uuid.New(), "", "")
	assert.ErrorIs(t, err, database.ErrResourceNotFound)
	_, err = e.svc.Stop(ctx, chatActor("", false, false), s.ID, "", "")
	assert.ErrorIs(t, err, ErrForbidden)
}

func TestChatListHidesStoppedWhenAskedTo(t *testing.T) {
	e := newChatEnv(t)
	ctx := context.Background()
	live, gone := e.start(t), e.start(t)
	_, err := e.svc.Stop(ctx, e.admin, gone.ID, "", "")
	require.NoError(t, err)

	all, err := e.svc.List(ctx, e.admin, false)
	require.NoError(t, err)
	active, err := e.svc.List(ctx, e.admin, true)
	require.NoError(t, err)

	assert.Len(t, all, 2)
	require.Len(t, active, 1)
	assert.Equal(t, live.ID, active[0].ID)
	_, err = e.svc.List(ctx, chatActor("", false, false), false)
	assert.ErrorIs(t, err, ErrForbidden)
}

func TestChatEventsReplayFromACursor(t *testing.T) {
	e := newChatEnv(t)
	ctx := context.Background()
	s := e.start(t)
	require.NoError(
		t,
		e.svc.RecordEvents(ctx, e.runner.ID, s.ID, "", []models.ChatEventInput{
			{Kind: "assistant", Payload: "a"}, {Kind: "assistant", Payload: "b"},
			{Kind: "tool", Payload: "c"},
		}),
	)

	all, err := e.svc.Events(ctx, e.admin, s.ID, -5, 0)
	require.NoError(t, err)
	rest, err := e.svc.Events(ctx, e.admin, s.ID, 1, 0)
	require.NoError(t, err)
	capped, err := e.svc.Events(ctx, e.admin, s.ID, 0, 2)
	require.NoError(t, err)

	require.Len(t, all, 3)
	assert.Equal(t, []int{1, 2, 3}, []int{all[0].Seq, all[1].Seq, all[2].Seq})
	require.Len(t, rest, 2)
	assert.Equal(t, 2, rest[0].Seq)
	assert.Len(t, capped, 2)
	_, err = e.svc.Events(ctx, e.admin, uuid.New(), 0, 0)
	assert.ErrorIs(t, err, database.ErrResourceNotFound)
	_, err = e.svc.Events(ctx, chatActor("", false, false), s.ID, 0, 0)
	assert.ErrorIs(t, err, ErrForbidden)
}

func TestChatRecordEventsRules(t *testing.T) {
	e := newChatEnv(t)
	ctx := context.Background()
	s := e.start(t)
	event := []models.ChatEventInput{{Kind: "assistant", Payload: "x"}}

	assert.ErrorIs(t, e.svc.RecordEvents(ctx, uuid.New(), s.ID, "", event), ErrForbidden,
		"another runner")
	assert.ErrorIs(t, e.svc.RecordEvents(ctx, e.runner.ID, s.ID, "", nil), ErrInvalid,
		"nothing to record")
	assert.ErrorIs(t, e.svc.RecordEvents(ctx, e.runner.ID, s.ID, "stopped", event),
		ErrInvalid, "stopping is the person's call")
	for _, kind := range []string{"", strings.Repeat("k", maxKindLen+1),
		models.ChatEventMessage, models.ChatEventAnswer, models.ChatEventStopped,
		models.ChatEventAsk, models.ChatEventAskAnswer} {
		assert.ErrorIs(t, e.svc.RecordEvents(ctx, e.runner.ID, s.ID, "",
			[]models.ChatEventInput{{Kind: kind, Payload: "x"}}), ErrInvalid, kind)
	}
	assert.ErrorIs(
		t,
		e.svc.RecordEvents(
			ctx,
			e.runner.ID,
			s.ID,
			"",
			make([]models.ChatEventInput, maxChatBatch+1),
		),
		ErrInvalid,
	)
	assert.ErrorIs(t, e.svc.RecordEvents(ctx, e.runner.ID, uuid.New(), "", event),
		database.ErrResourceNotFound)
	assert.Empty(t, e.chats.events[s.ID], "refusals append nothing")

	long := []models.ChatEventInput{
		{Kind: "assistant", Payload: strings.Repeat("a", maxEventBytes+10)},
	}
	require.NoError(
		t,
		e.svc.RecordEvents(ctx, e.runner.ID, s.ID, models.ChatWorking, long),
	)
	assert.Len(t, e.chats.events[s.ID][0].Payload, maxEventBytes)
	assert.Equal(t, models.ChatWorking, e.chats.sessions[s.ID].State)

	_, err := e.svc.Stop(ctx, e.admin, s.ID, "", "")
	require.NoError(t, err)
	assert.ErrorIs(t, e.svc.RecordEvents(ctx, e.runner.ID, s.ID, "", event), ErrConflict)
}

func TestStoppedIsConflictOnlyMapsConflicts(t *testing.T) {
	assert.NoError(t, stoppedIsConflict(nil))
	assert.ErrorIs(t, stoppedIsConflict(database.ErrResourceConflict), ErrConflict)
	assert.ErrorIs(t, stoppedIsConflict(database.ErrResourceNotFound),
		database.ErrResourceNotFound)
}

func TestChatEndStopsASessionTheRunnerEnded(t *testing.T) {
	e := newChatEnv(t)
	s := e.start(t)

	stopped, err := e.svc.End(context.Background(), e.runner.ID, s.ID, " left 2 files ")

	require.NoError(t, err)
	require.NotNil(t, stopped)
	assert.Equal(t, models.ChatStopped, stopped.State)
	assert.Equal(t, "left 2 files", stopped.Report)
}

func TestChatEndAddsToAPersonsReport(t *testing.T) {
	e := newChatEnv(t)
	s := e.start(t)
	_, err := e.svc.Stop(context.Background(), e.admin, s.ID, "done for today", "")
	require.NoError(t, err)

	stopped, err := e.svc.End(context.Background(), e.runner.ID, s.ID, "1 commit unpushed")

	require.NoError(t, err)
	assert.Nil(t, stopped)
	got, err := e.chats.Get(context.Background(), s.ID)
	require.NoError(t, err)
	assert.Equal(t, "done for today\n\n1 commit unpushed", got.Report)
}

func TestChatEndWithNothingLeftChangesNothing(t *testing.T) {
	e := newChatEnv(t)
	s := e.start(t)
	_, err := e.svc.Stop(context.Background(), e.admin, s.ID, "bye", "")
	require.NoError(t, err)

	_, err = e.svc.End(context.Background(), e.runner.ID, s.ID, "")

	require.NoError(t, err)
	got, err := e.chats.Get(context.Background(), s.ID)
	require.NoError(t, err)
	assert.Equal(t, "bye", got.Report)
}

func TestChatEndIsForTheirOwnRunnerOnly(t *testing.T) {
	e := newChatEnv(t)
	s := e.start(t)

	_, err := e.svc.End(context.Background(), uuid.New(), s.ID, "x")

	require.ErrorIs(t, err, ErrForbidden)
}

func TestChatStopPostsTheSummaryOnTheTask(t *testing.T) {
	e := newChatEnv(t)
	ctx := context.Background()
	in := startIn(e.repo.ID, e.runner.ID, models.HarnessClaudeCode)
	in.TaskID = uuid.New()
	s, err := e.svc.Start(ctx, e.admin, in)
	require.NoError(t, err)

	_, err = e.svc.Stop(ctx, e.admin, s.ID, "", "  Did the thing.  ")

	require.NoError(t, err)
	comments := postedComments(t, e.svc)
	assert.Equal(
		t,
		[]string{"runner:" + e.runner.ID.String() + ": Did the thing."},
		comments,
	)
}

func TestChatStopWithoutATaskOrSummaryPostsNothing(t *testing.T) {
	e := newChatEnv(t)
	ctx := context.Background()
	s := e.start(t)
	_, err := e.svc.Stop(ctx, e.admin, s.ID, "", "summary")
	require.NoError(t, err)
	assert.Empty(t, postedComments(t, e.svc))

	in := startIn(e.repo.ID, e.runner.ID, models.HarnessClaudeCode)
	in.TaskID = uuid.New()
	s, err = e.svc.Start(ctx, e.admin, in)
	require.NoError(t, err)
	_, err = e.svc.Stop(ctx, e.admin, s.ID, "", "")
	require.NoError(t, err)
	assert.Empty(t, postedComments(t, e.svc))
}

// postedComments is what the service commented on its fake task store.
func postedComments(t *testing.T, svc *ChatService) []string {
	t.Helper()
	tasks, ok := svc.tasks.(chatTasks)
	require.True(t, ok)
	return *tasks.comments
}
