package services

import (
	"context"
	"errors"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"

	"tools.xdoubleu.com/internal/kit/database"
	"tools.xdoubleu.com/internal/platform/factory/internal/models"
)

const (
	maxChatMessageLen = 20000
	maxChatReportLen  = 20000
	maxChatBatch      = 200
	// DefaultChatEvents and MaxChatEvents bound one replay.
	DefaultChatEvents = 200
	MaxChatEvents     = 500
	// MaxChatSessions is the most sessions one list returns.
	MaxChatSessions = 100
)

// ChatService runs chat sessions: a person talking to a harness on one runner.
type ChatService struct {
	chats   ChatStore
	repos   RepoStore
	tasks   TaskStore
	runners RunnerStore
	now     func() time.Time
	// askPoll is how often a waiting chat_ask looks for the answer.
	askPoll time.Duration
}

// StartChat is what a new session needs; TaskID is uuid.Nil for none.
type StartChat struct {
	RepoID   uuid.UUID
	TaskID   uuid.UUID
	Harness  models.Harness
	RunnerID uuid.UUID
}

// requireChatActor limits chats to a human admin in a browser: a session is a
// shell on the runner's Mac, so it is as privileged as running a task.
func requireChatActor(actor models.Actor) error {
	if !actor.IsBrowserAdmin() {
		return forbidden("only a human admin in a browser session can use chat")
	}
	return nil
}

// Start opens a session pinned to the runner. The runner must be online and have
// a free job slot, which the session then holds until it stops.
func (s *ChatService) Start(
	ctx context.Context,
	actor models.Actor,
	in StartChat,
) (*models.ChatSession, error) {
	if err := requireChatActor(actor); err != nil {
		return nil, err
	}
	if !in.Harness.Valid() {
		return nil, badRequest("invalid harness")
	}
	repo, err := s.repos.Get(ctx, in.RepoID)
	if err != nil {
		return nil, err
	}
	branch := "agent/chat-" + uuid.NewString()[:8]
	if in.TaskID != uuid.Nil {
		task, taskErr := s.tasks.Get(ctx, in.TaskID)
		if taskErr != nil {
			return nil, taskErr
		}
		if task.RepoID != repo.ID {
			return nil, badRequest("the task belongs to another repo")
		}
		branch = branchFor(task)
	}
	if err = s.requireOnline(ctx, in.RunnerID); err != nil {
		return nil, err
	}

	session, err := s.chats.Start(ctx, models.ChatSession{
		ID: uuid.Nil, RepoID: repo.ID, TaskID: in.TaskID, Harness: in.Harness,
		RunnerID: in.RunnerID, JobID: uuid.Nil, State: "", Branch: branch,
		Report: "", LastSeq: 0, CreatedBy: "", CreatedAt: time.Time{},
		UpdatedAt: time.Time{}, StoppedAt: nil,
	}, actor.String())
	if errors.Is(err, database.ErrResourceConflict) {
		return nil, conflict("the runner is already running as many jobs as it allows")
	}
	return session, err
}

func (s *ChatService) requireOnline(ctx context.Context, runnerID uuid.UUID) error {
	runners, err := s.runners.List(ctx)
	if err != nil {
		return err
	}
	for i := range runners {
		if runners[i].ID == runnerID {
			if !runners[i].Online(s.now()) {
				return conflict("the runner is offline")
			}
			return nil
		}
	}
	return database.ErrResourceNotFound
}

// Get returns one session.
func (s *ChatService) Get(
	ctx context.Context,
	actor models.Actor,
	id uuid.UUID,
) (*models.ChatSession, error) {
	if err := requireChatActor(actor); err != nil {
		return nil, err
	}
	return s.chats.Get(ctx, id)
}

// List returns the newest sessions first, without the stopped ones when
// activeOnly.
func (s *ChatService) List(
	ctx context.Context,
	actor models.Actor,
	activeOnly bool,
) ([]models.ChatSession, error) {
	if err := requireChatActor(actor); err != nil {
		return nil, err
	}
	return s.chats.List(ctx, activeOnly, MaxChatSessions)
}

// SendMessage hands the harness a message while it waits for one.
func (s *ChatService) SendMessage(
	ctx context.Context,
	actor models.Actor,
	id uuid.UUID,
	text string,
) error {
	return s.reply(ctx, actor, id, text, models.ChatIdle, models.ChatEventMessage)
}

// Answer replies to a question the harness asked.
func (s *ChatService) Answer(
	ctx context.Context,
	actor models.Actor,
	id uuid.UUID,
	text string,
) error {
	return s.reply(ctx, actor, id, text, models.ChatAsking, models.ChatEventAnswer)
}

// reply records the person's text and puts the session to work, when it is in the
// state that waits for it.
func (s *ChatService) reply(
	ctx context.Context,
	actor models.Actor,
	id uuid.UUID,
	text string,
	waiting models.ChatState,
	kind string,
) error {
	if err := requireChatActor(actor); err != nil {
		return err
	}
	text = strings.TrimSpace(text)
	if text == "" {
		return badRequest("the message is empty")
	}
	if utf8.RuneCountInString(text) > maxChatMessageLen {
		return badRequest("the message is too long")
	}
	// Not found stays distinct from a session in the wrong state.
	if _, err := s.chats.Get(ctx, id); err != nil {
		return err
	}
	_, err := s.chats.Transition(
		ctx, id, []models.ChatState{waiting}, models.ChatWorking)
	if errors.Is(err, database.ErrResourceConflict) {
		return conflict("the session is not waiting for a " + describeWait(waiting))
	}
	if err != nil {
		return err
	}
	return s.chats.Append(ctx, id,
		[]models.ChatEventInput{{Kind: kind, Payload: text}})
}

func describeWait(state models.ChatState) string {
	if state == models.ChatAsking {
		return "answer"
	}
	return "message"
}

// Stop ends the session, keeping the report of what it left behind, and frees
// the runner's slot.
func (s *ChatService) Stop(
	ctx context.Context,
	actor models.Actor,
	id uuid.UUID,
	report string,
) (*models.ChatSession, error) {
	if err := requireChatActor(actor); err != nil {
		return nil, err
	}
	report = strings.TrimSpace(report)
	if utf8.RuneCountInString(report) > maxChatReportLen {
		return nil, badRequest("the report is too long")
	}
	if _, err := s.chats.Get(ctx, id); err != nil {
		return nil, err
	}
	session, err := s.chats.Stop(ctx, id, report, models.JobSucceeded)
	if errors.Is(err, database.ErrResourceConflict) {
		return nil, conflict("the session has already stopped")
	}
	return session, err
}

// Events replays a session's events with seq above afterSeq, oldest first, so a
// client resumes from the last seq it saw.
func (s *ChatService) Events(
	ctx context.Context,
	actor models.Actor,
	id uuid.UUID,
	afterSeq, limit int,
) ([]models.ChatEvent, error) {
	if err := requireChatActor(actor); err != nil {
		return nil, err
	}
	if _, err := s.chats.Get(ctx, id); err != nil {
		return nil, err
	}
	if limit <= 0 {
		limit = DefaultChatEvents
	}
	return s.chats.After(ctx, id, max(afterSeq, 0), min(limit, MaxChatEvents))
}

// RecordEvents appends what the pinned runner saw and, when state is not empty,
// moves the session to it. A runner may only write to its own sessions, and may
// not forge what the person said.
func (s *ChatService) RecordEvents(
	ctx context.Context,
	runnerID, id uuid.UUID,
	state models.ChatState,
	events []models.ChatEventInput,
) error {
	if len(events) == 0 && state == "" {
		return badRequest("nothing to record")
	}
	if len(events) > maxChatBatch {
		return badRequest("too many events in one call")
	}
	if state != "" && state != models.ChatIdle && state != models.ChatWorking &&
		state != models.ChatAsking {
		return badRequest("invalid state")
	}
	cleaned, err := cleanChatEvents(events)
	if err != nil {
		return err
	}
	session, err := s.chats.Get(ctx, id)
	if err != nil {
		return err
	}
	if session.RunnerID != runnerID {
		return forbidden("the session belongs to another runner")
	}
	if session.State == models.ChatStopped {
		return conflict("the session has already stopped")
	}
	if len(cleaned) > 0 {
		if err = s.chats.Append(ctx, id, cleaned); err != nil {
			return stoppedIsConflict(err)
		}
	}
	if state == "" {
		return nil
	}
	_, err = s.chats.Transition(ctx, id, []models.ChatState{
		models.ChatIdle, models.ChatWorking, models.ChatAsking,
	}, state)
	return stoppedIsConflict(err)
}

func stoppedIsConflict(err error) error {
	if errors.Is(err, database.ErrResourceConflict) {
		return conflict("the session has already stopped")
	}
	return err
}

// cleanChatEvents checks the runner's events and cuts oversized payloads.
func cleanChatEvents(
	events []models.ChatEventInput,
) ([]models.ChatEventInput, error) {
	out := make([]models.ChatEventInput, 0, len(events))
	for _, e := range events {
		switch {
		case e.Kind == "" || len(e.Kind) > maxKindLen:
			return nil, badRequest("an event needs a kind of up to 40 characters")
		case e.Kind == models.ChatEventMessage || e.Kind == models.ChatEventAnswer ||
			e.Kind == models.ChatEventStopped || e.Kind == models.ChatEventAsk ||
			e.Kind == models.ChatEventAskAnswer:
			return nil, badRequest("the kind " + e.Kind + " is reserved")
		}
		out = append(out, models.ChatEventInput{Kind: e.Kind, Payload: truncate(e.Payload)})
	}
	return out, nil
}

// Feed returns a session with its events above afterSeq, oldest first. It checks
// no actor: the caller has authenticated the person or the pinned runner.
func (s *ChatService) Feed(
	ctx context.Context,
	id uuid.UUID,
	afterSeq, limit int,
) (*models.ChatSession, []models.ChatEvent, error) {
	session, err := s.chats.Get(ctx, id)
	if err != nil {
		return nil, nil, err
	}
	if limit <= 0 {
		limit = DefaultChatEvents
	}
	events, err := s.chats.After(ctx, id, max(afterSeq, 0), min(limit, MaxChatEvents))
	return session, events, err
}

// ForRunner returns a session when it is pinned to the runner.
func (s *ChatService) ForRunner(
	ctx context.Context,
	runnerID, id uuid.UUID,
) (*models.ChatSession, error) {
	session, err := s.chats.Get(ctx, id)
	if err != nil {
		return nil, err
	}
	if session.RunnerID != runnerID {
		return nil, forbidden("the session belongs to another runner")
	}
	return session, nil
}

// End is the pinned runner closing a session it ran: it stops one that is still
// going, which the runner does when its harness died, and adds the report of what
// the session left behind to what a person's stop kept. It returns the stopped
// session when it stopped one.
func (s *ChatService) End(
	ctx context.Context,
	runnerID, id uuid.UUID,
	report string,
) (*models.ChatSession, error) {
	report = strings.TrimSpace(report)
	if utf8.RuneCountInString(report) > maxChatReportLen {
		report = string([]rune(report)[:maxChatReportLen])
	}
	session, err := s.ForRunner(ctx, runnerID, id)
	if err != nil {
		return nil, err
	}
	if session.State != models.ChatStopped {
		stopped, stopErr := s.chats.Stop(ctx, id, report, models.JobSucceeded)
		if !errors.Is(stopErr, database.ErrResourceConflict) {
			return stopped, stopErr
		}
		// Stopped meanwhile: the report is added to the person's.
	}
	if report == "" {
		return nil, nil //nolint:nilnil // nothing to add
	}
	return nil, s.chats.AppendReport(ctx, id, report)
}
