package services

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"

	"tools.xdoubleu.com/internal/kit/database"
	"tools.xdoubleu.com/internal/platform/factory/internal/models"
)

const (
	maxAskQuestionLen = 1000
	maxAskOptionLen   = 200
	minAskOptions     = 2
	maxAskOptions     = 4
	askPollEvery      = 500 * time.Millisecond
	askScanBatch      = MaxChatEvents
)

// AskInput is a question the agent puts to the person: a choice between options,
// or with two options a confirmation.
type AskInput struct {
	Question string
	Options  []string
	Multi    bool
}

type askPayload struct {
	Question string   `json:"question"`
	Options  []string `json:"options"`
	Multi    bool     `json:"multi"`
}

type askAnswerPayload struct {
	Ask    int    `json:"ask"`
	Answer string `json:"answer"`
}

// SessionForJob is the chat session a chat job runs, or not found for any other
// job: only a chat's own account may ask the person a question.
func (s *ChatService) SessionForJob(
	ctx context.Context,
	jobID uuid.UUID,
) (*models.ChatSession, error) {
	return s.chats.ForJob(ctx, jobID)
}

// Ask records a question as an event the person's card answers and puts the
// session to asking. It returns the question's id.
func (s *ChatService) Ask(
	ctx context.Context,
	id uuid.UUID,
	in AskInput,
) (int, error) {
	question := strings.TrimSpace(in.Question)
	switch {
	case question == "":
		return 0, badRequest("the question is empty")
	case utf8.RuneCountInString(question) > maxAskQuestionLen:
		return 0, badRequest("the question is too long")
	case len(in.Options) < minAskOptions || len(in.Options) > maxAskOptions:
		return 0, badRequest("a question takes 2 to 4 options")
	}
	options := make([]string, len(in.Options))
	for i, o := range in.Options {
		options[i] = strings.TrimSpace(o)
		if options[i] == "" || utf8.RuneCountInString(options[i]) > maxAskOptionLen {
			return 0, badRequest("an option needs 1 to 200 characters")
		}
	}
	raw, err := json.Marshal(
		askPayload{Question: question, Options: options, Multi: in.Multi})
	if err != nil {
		return 0, err //nolint:wrapcheck // a plain struct always marshals
	}
	seq, err := s.chats.Ask(ctx, id, string(raw))
	return seq, stoppedIsConflict(err)
}

// AwaitAnswer waits up to wait for the person's answer to question ask and
// returns it; ok is false when none came in time. A question that is not this
// session's is not found, and a session that stopped is a conflict.
func (s *ChatService) AwaitAnswer(
	ctx context.Context,
	id uuid.UUID,
	ask int,
	wait time.Duration,
) (string, bool, error) {
	if err := s.requireAsk(ctx, id, ask); err != nil {
		return "", false, err
	}
	deadline := time.NewTimer(wait)
	defer deadline.Stop()
	tick := time.NewTicker(s.askPoll)
	defer tick.Stop()
	cursor := ask
	for {
		answer, ok, next, err := s.scanAnswer(ctx, id, ask, cursor)
		if err != nil || ok {
			return answer, ok, err
		}
		cursor = next
		session, getErr := s.chats.Get(ctx, id)
		if getErr != nil {
			return "", false, getErr
		}
		if session.State == models.ChatStopped {
			return "", false, conflict("the chat was closed before the question was answered")
		}
		select {
		case <-tick.C:
		case <-deadline.C:
			return "", false, nil
		case <-ctx.Done():
			return "", false, nil
		}
	}
}

// scanAnswer looks through the events after cursor for the answer to ask.
func (s *ChatService) scanAnswer(
	ctx context.Context,
	id uuid.UUID,
	ask, cursor int,
) (string, bool, int, error) {
	for {
		events, afterErr := s.chats.After(ctx, id, cursor, askScanBatch)
		if afterErr != nil {
			return "", false, cursor, afterErr
		}
		for i := range events {
			cursor = events[i].Seq
			if got, match := answerTo(&events[i], ask); match {
				return got, true, cursor, nil
			}
		}
		if len(events) < askScanBatch {
			return "", false, cursor, nil
		}
	}
}

func answerTo(e *models.ChatEvent, ask int) (string, bool) {
	if e.Kind != models.ChatEventAskAnswer {
		return "", false
	}
	var p askAnswerPayload
	if json.Unmarshal([]byte(e.Payload), &p) != nil || p.Ask != ask {
		return "", false
	}
	return p.Answer, true
}

// requireAsk checks that event ask is a question of this session.
func (s *ChatService) requireAsk(ctx context.Context, id uuid.UUID, ask int) error {
	events, err := s.chats.After(ctx, id, ask-1, 1)
	if err != nil {
		return err
	}
	if len(events) == 0 || events[0].Seq != ask || events[0].Kind != models.ChatEventAsk {
		return database.ErrResourceNotFound
	}
	return nil
}

// AnswerAsk is the person answering the card of question ask: the agent's
// waiting chat_ask call returns the text. A question already answered is a
// conflict.
func (s *ChatService) AnswerAsk(
	ctx context.Context,
	actor models.Actor,
	id uuid.UUID,
	ask int,
	text string,
) error {
	if err := requireChatActor(actor); err != nil {
		return err
	}
	text = strings.TrimSpace(text)
	if text == "" {
		return badRequest("the answer is empty")
	}
	if utf8.RuneCountInString(text) > maxChatMessageLen {
		return badRequest("the answer is too long")
	}
	if _, err := s.chats.Get(ctx, id); err != nil {
		return err
	}
	if err := s.requireAsk(ctx, id, ask); err != nil {
		return err
	}
	if _, answered, _, err := s.scanAnswer(ctx, id, ask, ask); err != nil || answered {
		if err != nil {
			return err
		}
		return conflict("the question is already answered")
	}
	_, err := s.chats.Transition(
		ctx, id, []models.ChatState{models.ChatAsking}, models.ChatWorking)
	if errors.Is(err, database.ErrResourceConflict) {
		return conflict("the session is not waiting for an answer")
	}
	if err != nil {
		return err
	}
	raw, err := json.Marshal(askAnswerPayload{Ask: ask, Answer: text})
	if err != nil {
		return err //nolint:wrapcheck // a plain struct always marshals
	}
	return s.chats.Append(ctx, id,
		[]models.ChatEventInput{{Kind: models.ChatEventAskAnswer, Payload: string(raw)}})
}

// AskReply is where a chat_ask call stands: the question's id and chat, and the
// answer once the person gave it.
type AskReply struct {
	ChatID   uuid.UUID
	AskID    int
	Answer   string
	Answered bool
}

// AskPerson is chat_ask for the chat job jobID: with askID zero it puts a new
// question to the person, else it keeps waiting for the answer to that one. It
// calls asked once a new question is recorded. It returns when the person
// answered or after wait, so a call stays within a request's deadline and the
// agent calls again with the question's id.
func (s *ChatService) AskPerson(
	ctx context.Context,
	jobID uuid.UUID,
	askID int,
	in AskInput,
	wait time.Duration,
	asked func(chatID uuid.UUID),
) (AskReply, error) {
	session, err := s.chats.ForJob(ctx, jobID)
	if errors.Is(err, database.ErrResourceNotFound) {
		return AskReply{}, forbidden("only a chat session can ask the person a question")
	}
	if err != nil {
		return AskReply{}, err
	}
	reply := AskReply{ChatID: session.ID, AskID: askID, Answer: "", Answered: false}
	if askID == 0 {
		if reply.AskID, err = s.Ask(ctx, session.ID, in); err != nil {
			return reply, err
		}
		asked(session.ID)
	}
	reply.Answer, reply.Answered, err = s.AwaitAnswer(ctx, session.ID, reply.AskID, wait)
	return reply, err
}
