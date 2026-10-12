package services

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"tools.xdoubleu.com/apps/taste/internal/models"
	"tools.xdoubleu.com/internal/consumption"
	"tools.xdoubleu.com/internal/openrouter"
)

// memStore records what the service saved.
type memStore struct {
	profile    *models.Profile
	accept     bool
	notBefore  time.Time
	staleAfter time.Time
	saved      string
	note       string
	startedAt  time.Time
	err        error
}

func (m *memStore) Get(context.Context, string) (*models.Profile, error) {
	return m.profile, m.err
}

func (m *memStore) Request(
	_ context.Context, _ string, _, notBefore time.Time,
) (bool, error) {
	m.notBefore = notBefore
	return m.accept, m.err
}

func (m *memStore) SaveGenerated(
	_ context.Context, _ string, s models.ProfileSections, model string,
	startedAt time.Time,
) error {
	m.saved, m.note, m.startedAt = s.Summary+"|"+model, "", startedAt
	return m.err
}

func (m *memStore) SaveError(
	_ context.Context, _, note string, startedAt time.Time,
) error {
	m.saved, m.note, m.startedAt = "error", note, startedAt
	return m.err
}

func (m *memStore) SaveEmpty(
	_ context.Context, _, note string, startedAt time.Time,
) error {
	m.saved, m.note, m.startedAt = "empty", note, startedAt
	return m.err
}

func (m *memStore) Due(
	_ context.Context, ids []string, staleBefore time.Time,
) ([]string, error) {
	m.staleAfter = staleBefore
	return ids, m.err
}

type stubLLM struct {
	content string
	err     error
}

func answer(content string) stubLLM { return stubLLM{content: content, err: nil} }

func failing(err error) stubLLM { return stubLLM{content: "", err: err} }

func (s stubLLM) Complete(
	_ context.Context, req openrouter.Request,
) (*openrouter.Response, error) {
	//nolint:exhaustruct // only the answer matters
	return &openrouter.Response{Model: req.Model + "@served", Content: s.content}, s.err
}

type eventsSource struct {
	events []consumption.Event
	err    error
}

func (eventsSource) GetName() string { return "books" }

func (s eventsSource) ListConsumption(
	context.Context, string,
) ([]consumption.Event, error) {
	return s.events, s.err
}

//nolint:gochecknoglobals // fixed clock
var clock = time.Date(2026, 10, 12, 8, 0, 0, 0, time.UTC)

func service(store *memStore, llm openrouter.Client, src eventsSource) *ProfileService {
	s := NewProfileService(NewLogService([]Source{src}), store, llm, "gen/model", true)
	s.now = func() time.Time { return clock }
	return s
}

func all(string) bool { return true }

//nolint:gochecknoglobals // fixture
var oneBook = eventsSource{events: []consumption.Event{{
	Domain: consumption.DomainBooks, ItemID: "1", Title: "Dune", Creator: "",
	Tags: nil, At: clock, Signal: consumption.StrongPositive, Reason: "rated 5★",
}}, err: nil}

func TestProfileService_Request(t *testing.T) {
	store := &memStore{accept: true} //nolint:exhaustruct // the rest start empty
	ok, err := service(store, answer(""), oneBook).Request(context.Background(), "u")
	require.NoError(t, err)
	assert.True(t, ok)
	assert.Equal(t, clock.Add(-RefreshCooldown), store.notBefore)

	unconfigured := NewProfileService(NewLogService(nil), store, answer(""), "m", false)
	assert.False(t, unconfigured.Configured())
	_, err = unconfigured.Request(context.Background(), "u")
	require.ErrorIs(t, err, ErrNotConfigured)
}

func TestProfileService_DueAndGet(t *testing.T) {
	//nolint:exhaustruct // the rest start empty
	store := &memStore{profile: &models.Profile{UserID: "u"}}
	s := service(store, answer(""), oneBook)
	assert.True(t, s.Configured())

	due, err := s.Due(context.Background(), []string{"a"})
	require.NoError(t, err)
	assert.Equal(t, []string{"a"}, due)
	assert.Equal(t, clock.Add(-ProfileMaxAge), store.staleAfter)

	p, err := s.Get(context.Background(), "u")
	require.NoError(t, err)
	assert.Equal(t, "u", p.UserID)
}

func TestProfileService_Generate(t *testing.T) {
	tests := []struct {
		name  string
		src   eventsSource
		llm   stubLLM
		saved string
		note  string
		fails bool
	}{
		{
			"written", oneBook,
			answer(`{"summary": "S", "know": [], "like": [], "drifting": [], "bounce": []}`),
			"S|gen/model@served", "", false,
		},
		{
			"empty log", eventsSource{events: nil, err: nil}, answer(""),
			"empty", nothingConsumedNote, false,
		},
		{
			"source down", eventsSource{events: nil, err: errors.New("db")}, answer(""),
			"error", "Your history couldn't be read.", true,
		},
		{
			"model down", oneBook, failing(errors.New("503")),
			"error", "The model service failed.", true,
		},
		{
			"bad answer", oneBook, answer(`nope`),
			"error", "The model's answer couldn't be read.", true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			store := &memStore{} //nolint:exhaustruct // the rest start empty
			err := service(store, tt.llm, tt.src).Generate(context.Background(), "u", all)
			assert.Equal(t, tt.fails, err != nil, "error: %v", err)
			assert.Equal(t, tt.saved, store.saved)
			assert.Equal(t, tt.note, store.note)
			assert.Equal(t, clock, store.startedAt)
		})
	}
}

func TestProfileService_GenerateReportsSaveFailure(t *testing.T) {
	store := &memStore{err: errors.New("disk full")} //nolint:exhaustruct // rest empty
	err := service(store, failing(errors.New("503")), oneBook).
		Generate(context.Background(), "u", all)
	require.ErrorContains(t, err, "503")
	require.ErrorContains(t, err, "disk full")
}

func TestProfileService_GenerateSendsTheDigest(t *testing.T) {
	var got openrouter.Request
	llm := recordLLM{got: &got}
	store := &memStore{} //nolint:exhaustruct // the rest start empty
	require.NoError(
		t,
		service(store, llm, oneBook).Generate(context.Background(), "u", all),
	)
	assert.Equal(t, "gen/model", got.Model)
	assert.Equal(t, "taste_profile", got.SchemaName)
	assert.Equal(t, profileMaxTokens, got.MaxTokens)
	assert.Nil(t, got.WebSearch)
	require.Len(t, got.Messages, 2)
	assert.Equal(t, "system", got.Messages[0].Role)
	assert.Equal(t, profileSystemPrompt, got.Messages[0].Content)
	assert.Equal(t, "user", got.Messages[1].Role)
	assert.Contains(t, got.Messages[1].Content, "- Dune (books, +2 rated 5★, 2026-10)")
}

type recordLLM struct {
	got *openrouter.Request
}

func (r recordLLM) Complete(
	_ context.Context, req openrouter.Request,
) (*openrouter.Response, error) {
	*r.got = req
	//nolint:exhaustruct // only the answer matters
	return &openrouter.Response{Model: "m", Content: `{"summary": "S"}`}, nil
}
