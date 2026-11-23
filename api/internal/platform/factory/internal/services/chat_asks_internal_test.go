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

func abChoice() AskInput {
	return AskInput{Question: "A or B?", Options: []string{"A", "B"}, Multi: false}
}

func TestAskRecordsTheCardAndWaitsForTheAnswer(t *testing.T) {
	e := newChatEnv(t)
	ctx := context.Background()
	s := e.start(t)

	var asked uuid.UUID
	got := make(chan AskReply, 1)
	go func() {
		reply, err := e.svc.AskPerson(ctx, s.JobID, 0, abChoice(), 5*time.Second,
			func(id uuid.UUID) { asked = id })
		assert.NoError(t, err)
		got <- reply
	}()

	var ask models.ChatEvent
	require.Eventually(t, func() bool {
		events, err := e.svc.Events(ctx, e.admin, s.ID, 0, 0)
		require.NoError(t, err)
		if len(events) == 0 {
			return false
		}
		ask = events[0]
		return true
	}, time.Second, time.Millisecond)
	assert.Equal(t, models.ChatEventAsk, ask.Kind)
	assert.JSONEq(t,
		`{"question":"A or B?","options":["A","B"],"multi":false}`, ask.Payload)
	state, err := e.svc.Get(ctx, e.admin, s.ID)
	require.NoError(t, err)
	assert.Equal(t, models.ChatAsking, state.State)

	require.NoError(t, e.svc.AnswerAsk(ctx, e.admin, s.ID, ask.Seq, " B "))

	reply := <-got
	assert.True(t, reply.Answered)
	assert.Equal(t, "B", reply.Answer)
	assert.Equal(t, ask.Seq, reply.AskID)
	assert.Equal(t, s.ID, asked)
	state, err = e.svc.Get(ctx, e.admin, s.ID)
	require.NoError(t, err)
	assert.Equal(t, models.ChatWorking, state.State, "the turn goes on")
}

func TestAnUnansweredAskKeepsWaitingOnTheSameQuestion(t *testing.T) {
	e := newChatEnv(t)
	ctx := context.Background()
	s := e.start(t)

	first, err := e.svc.AskPerson(ctx, s.JobID, 0, abChoice(), 5*time.Millisecond,
		func(uuid.UUID) {})
	require.NoError(t, err)
	assert.False(t, first.Answered)
	require.NotZero(t, first.AskID)

	require.NoError(t, e.svc.AnswerAsk(ctx, e.admin, s.ID, first.AskID, "A"))
	var none AskInput
	again, err := e.svc.AskPerson(ctx, s.JobID, first.AskID, none, time.Second,
		func(uuid.UUID) { t.Fatal("a repeated call records no new question") })
	require.NoError(t, err)
	assert.True(t, again.Answered)
	assert.Equal(t, "A", again.Answer)
	assert.Equal(t, first.AskID, again.AskID)
}

func TestAskFromANonChatJobIsForbidden(t *testing.T) {
	e := newChatEnv(t)

	_, err := e.svc.AskPerson(context.Background(), uuid.New(), 0, abChoice(),
		time.Millisecond, func(uuid.UUID) {})

	assert.ErrorIs(t, err, ErrForbidden)
}

func TestAskValidation(t *testing.T) {
	e := newChatEnv(t)
	ctx := context.Background()
	s := e.start(t)
	long := strings.Repeat("x", maxAskOptionLen+1)

	for name, in := range map[string]AskInput{
		"no question": {Question: " ", Options: []string{"A", "B"}, Multi: false},
		"long question": {
			Question: strings.Repeat("q", maxAskQuestionLen+1),
			Options:  []string{"A", "B"}, Multi: false,
		},
		"one option": {Question: "q", Options: []string{"A"}, Multi: false},
		"five": {
			Question: "q", Options: []string{"1", "2", "3", "4", "5"}, Multi: false,
		},
		"empty":       {Question: "q", Options: []string{"A", " "}, Multi: false},
		"long option": {Question: "q", Options: []string{"A", long}, Multi: false},
	} {
		_, err := e.svc.Ask(ctx, s.ID, in)
		assert.ErrorIs(t, err, ErrInvalid, name)
	}
	events, err := e.svc.Events(ctx, e.admin, s.ID, 0, 0)
	require.NoError(t, err)
	assert.Empty(t, events)
}

func TestAnswerAskRules(t *testing.T) {
	e := newChatEnv(t)
	ctx := context.Background()
	s := e.start(t)
	ask, err := e.svc.Ask(ctx, s.ID, abChoice())
	require.NoError(t, err)

	assert.ErrorIs(t, e.svc.AnswerAsk(ctx, e.admin, s.ID, ask, " "), ErrInvalid)
	assert.ErrorIs(t,
		e.svc.AnswerAsk(ctx, e.admin, s.ID, ask, strings.Repeat("a", maxChatMessageLen+1)),
		ErrInvalid)
	assert.ErrorIs(t,
		e.svc.AnswerAsk(ctx, chatActor(models.ActorAgent, false, false), s.ID, ask, "A"),
		ErrForbidden)
	assert.ErrorIs(t, e.svc.AnswerAsk(ctx, e.admin, s.ID, ask+1, "A"),
		database.ErrResourceNotFound, "not a question")
	assert.ErrorIs(t, e.svc.AnswerAsk(ctx, e.admin, uuid.New(), ask, "A"),
		database.ErrResourceNotFound)

	require.NoError(t, e.svc.AnswerAsk(ctx, e.admin, s.ID, ask, "A"))
	assert.ErrorIs(t, e.svc.AnswerAsk(ctx, e.admin, s.ID, ask, "B"), ErrConflict,
		"answered once")

	events, err := e.svc.Events(ctx, e.admin, s.ID, ask, 0)
	require.NoError(t, err)
	require.Len(t, events, 1)
	assert.Equal(t, models.ChatEventAskAnswer, events[0].Kind)
	assert.JSONEq(t, `{"ask":1,"answer":"A"}`, events[0].Payload)
}

func TestAnAnswerNeedsTheSessionToBeAsking(t *testing.T) {
	e := newChatEnv(t)
	ctx := context.Background()
	s := e.start(t)
	ask, err := e.svc.Ask(ctx, s.ID, abChoice())
	require.NoError(t, err)
	require.NoError(t, e.svc.RecordEvents(ctx, e.runner.ID, s.ID, models.ChatIdle, nil))

	assert.ErrorIs(t, e.svc.AnswerAsk(ctx, e.admin, s.ID, ask, "A"), ErrConflict)
}

func TestAWaitingAskFailsWhenTheChatStops(t *testing.T) {
	e := newChatEnv(t)
	ctx := context.Background()
	s := e.start(t)
	ask, err := e.svc.Ask(ctx, s.ID, abChoice())
	require.NoError(t, err)
	_, err = e.svc.Stop(ctx, e.admin, s.ID, "")
	require.NoError(t, err)

	_, _, err = e.svc.AwaitAnswer(ctx, s.ID, ask, time.Second)
	assert.ErrorIs(t, err, ErrConflict)
	_, err = e.svc.Ask(ctx, s.ID, abChoice())
	assert.ErrorIs(t, err, ErrConflict, "nobody to ask")
}

func TestAWaitingAskStopsWithItsContext(t *testing.T) {
	e := newChatEnv(t)
	s := e.start(t)
	ask, err := e.svc.Ask(context.Background(), s.ID, abChoice())
	require.NoError(t, err)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, ok, err := e.svc.AwaitAnswer(ctx, s.ID, ask, time.Minute)

	require.NoError(t, err)
	assert.False(t, ok)
}
