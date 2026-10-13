package taste_test

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"sync"
	"testing"
	"time"

	"connectrpc.com/connect/v2"
	"connectrpc.com/connect/v2/connecthttp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"tools.xdoubleu.com/apps/taste"
	tastev1 "tools.xdoubleu.com/gen/taste/v1"
	"tools.xdoubleu.com/gen/taste/v1/tastev1connect"
	"tools.xdoubleu.com/internal/auth"
	"tools.xdoubleu.com/internal/config"
	"tools.xdoubleu.com/internal/consumption"
	"tools.xdoubleu.com/internal/database/postgres"
	"tools.xdoubleu.com/internal/logging"
	sharedmocks "tools.xdoubleu.com/internal/mocks"
	"tools.xdoubleu.com/internal/openrouter"
	"tools.xdoubleu.com/internal/testhelper"
)

const userID = "taste-test-user"

//nolint:gochecknoglobals //needed for tests
var (
	testDB  postgres.DB
	testCfg config.Config
)

func TestMain(m *testing.M) {
	testCfg = testhelper.NewTestConfig()
	pool := testhelper.ConnectTestDB(testCfg.DBDsn)
	testDB = pool

	ctx := context.Background()
	if _, err := testDB.Exec(ctx, "DROP SCHEMA IF EXISTS taste CASCADE"); err != nil {
		panic(err)
	}
	// Tests share one database; ensure the family tables exist.
	if _, err := testDB.Exec(ctx, `
		CREATE SCHEMA IF NOT EXISTS global;
		CREATE TABLE IF NOT EXISTS global.families (
			id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
			created_at TIMESTAMPTZ NOT NULL DEFAULT now()
		);
		CREATE TABLE IF NOT EXISTS global.family_members (
			user_id TEXT PRIMARY KEY,
			family_id UUID NOT NULL REFERENCES global.families (id) ON DELETE CASCADE,
			joined_at TIMESTAMPTZ NOT NULL DEFAULT now(),
			display_name TEXT NOT NULL DEFAULT ''
		)`); err != nil {
		panic(err)
	}
	a := taste.New(
		sharedmocks.NewMockedAuthService(userID), logging.NewNopLogger(), testCfg, testDB,
		nil, noCatalogs(),
	)
	if err := a.ApplyMigrations(ctx, pool); err != nil {
		panic(err)
	}
	os.Exit(m.Run())
}

// profileCalls counts the profile completions asked for.
func (f *fakeLLM) profileCalls() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	n := 0
	for _, r := range f.requests {
		if r.SchemaName == "taste_profile" {
			n++
		}
	}
	return n
}

// noCatalogs finds nothing in any domain.
func noCatalogs() taste.Catalogs {
	return taste.Catalogs{
		Books: nil, Movies: nil, Games: nil, Podcasts: nil, Feeds: nil,
		WantBook: nil, WantTitle: nil,
	}
}

// newLLM answers profile completions with content and candidate completions
// with no picks.
func newLLM(content string) *fakeLLM {
	//nolint:exhaustruct // the rest start empty
	return &fakeLLM{content: content}
}

// fakeLLM answers every completion with content, or fails with err; during
// runs while the completion is in flight.
type fakeLLM struct {
	mu       sync.Mutex
	content  string
	err      error
	during   func()
	requests []openrouter.Request
}

func (f *fakeLLM) Complete(
	_ context.Context, req openrouter.Request,
) (*openrouter.Response, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.requests = append(f.requests, req)
	if req.SchemaName == "taste_candidates" {
		//nolint:exhaustruct // usage is irrelevant here
		return &openrouter.Response{Model: "served/model", Content: `{"picks": []}`}, nil
	}
	if f.during != nil {
		f.during()
	}
	if f.err != nil {
		return nil, f.err
	}
	//nolint:exhaustruct // usage is irrelevant here
	return &openrouter.Response{Model: "served/model", Content: f.content}, nil
}

// userSource is app name's fixed events for userID only.
type userSource struct {
	name   string
	events []consumption.Event
	err    error
}

func (s userSource) GetName() string { return s.name }

func (s userSource) ListConsumption(
	_ context.Context, uid string,
) ([]consumption.Event, error) {
	if uid != userID {
		return nil, nil
	}
	return s.events, s.err
}

// newClient serves the app to an admin, who may read every source.
func newClient(
	t *testing.T,
	sources ...taste.Source,
) tastev1connect.TasteServiceClient {
	t.Helper()
	return newClientAs(t, sharedmocks.NewMockedAdminAuthService(userID), sources...)
}

func newClientAs(
	t *testing.T,
	authService auth.Service,
	sources ...taste.Source,
) tastev1connect.TasteServiceClient {
	t.Helper()
	a := taste.NewInner(
		authService, logging.NewNopLogger(), testCfg, testDB, newLLM(""), nil, noCatalogs(),
		sources...,
	)
	ts := httptest.NewServer(testhelper.BuildMux(a))
	t.Cleanup(ts.Close)
	return tastev1connect.NewTasteServiceClient(
		connect.NewClient(
			connecthttp.NewTransport(
				http.DefaultClient,
				ts.URL,
				connecthttp.WithReadMaxBytes(0),
			),
		),
	)
}

//nolint:gochecknoglobals // fixed fixture
var (
	read = time.Date(2026, 3, 4, 5, 6, 7, 0, time.UTC)
	book = consumption.Event{
		Domain: consumption.DomainBooks, ItemID: "b1", Title: "Dune",
		Creator: "Frank Herbert", Tags: []string{"sci-fi"}, At: read,
		Signal: consumption.StrongPositive, Reason: "rated 5★",
	}
	game = consumption.Event{
		Domain: consumption.DomainGames, ItemID: "g1", Title: "Outer Wilds",
		Creator: "", Tags: nil, At: time.Time{},
		Signal: consumption.Negative, Reason: "under an hour played",
	}
)

func TestAppMetadata(t *testing.T) {
	a := taste.New(
		sharedmocks.NewMockedAuthService(userID), logging.NewNopLogger(), testCfg, testDB,
		nil, noCatalogs(),
	)
	assert.Equal(t, "taste", a.GetName())
	assert.Equal(t, "Taste", a.GetDisplayName())
	require.NoError(t, a.Start())
}

func TestGetStats(t *testing.T) {
	c := newClient(t,
		userSource{name: "books", events: []consumption.Event{book}, err: nil},
		userSource{name: "games", events: []consumption.Event{game}, err: nil},
	)
	resp, err := c.GetStats(context.Background(), &tastev1.GetStatsRequest{})
	require.NoError(t, err)

	require.Len(t, resp.Domains, 6)
	books := resp.Domains[0]
	assert.Equal(t, "books", books.Domain)
	assert.Equal(t, int32(1), books.Consumed)
	assert.Equal(t, int32(1), books.Positive)
	assert.Equal(t, "2026-03-04T05:06:07Z", books.LastAt)
	games := resp.Domains[2]
	assert.Equal(t, int32(1), games.Negative)
	assert.Empty(t, games.LastAt)

	assert.Len(t, resp.Months, 12)
	require.Len(t, resp.TopTags, 1)
	assert.Equal(t, "sci-fi", resp.TopTags[0].Name)
	assert.Equal(t, []string{"books"}, resp.TopTags[0].Domains)
	require.Len(t, resp.TopCreators, 1)
	assert.Equal(t, "Frank Herbert", resp.TopCreators[0].Name)
}

func TestGetStats_MonthCounts(t *testing.T) {
	recent := book
	recent.At = time.Now()
	c := newClient(
		t,
		userSource{name: "books", events: []consumption.Event{recent}, err: nil},
	)
	resp, err := c.GetStats(context.Background(), &tastev1.GetStatsRequest{})
	require.NoError(t, err)
	last := resp.Months[11]
	require.Len(t, last.Counts, 1)
	assert.Equal(t, "books", last.Counts[0].Domain)
	assert.Equal(t, int32(1), last.Counts[0].Count)
	assert.Empty(t, resp.Months[0].Counts)
}

func TestListEvents(t *testing.T) {
	c := newClient(
		t,
		userSource{name: "books", events: []consumption.Event{game, book}, err: nil},
	)
	resp, err := c.ListEvents(
		context.Background(),
		&tastev1.ListEventsRequest{Domain: "", Limit: 1, Offset: 0},
	)
	require.NoError(t, err)
	require.Len(t, resp.Events, 1)
	assert.True(t, resp.HasMore)
	e := resp.Events[0]
	assert.Equal(t, "Dune", e.Title)
	assert.Equal(t, "books", e.Domain)
	assert.Equal(t, "b1", e.ItemId)
	assert.Equal(t, "Frank Herbert", e.Creator)
	assert.Equal(t, []string{"sci-fi"}, e.Tags)
	assert.Equal(t, "2026-03-04T05:06:07Z", e.At)
	assert.Equal(t, int32(2), e.Signal)
	assert.Equal(t, "rated 5★", e.Reason)

	resp, err = c.ListEvents(
		context.Background(),
		&tastev1.ListEventsRequest{Domain: "", Limit: 1, Offset: 1},
	)
	require.NoError(t, err)
	require.Len(t, resp.Events, 1)
	assert.Equal(t, "Outer Wilds", resp.Events[0].Title)
	assert.Empty(t, resp.Events[0].At)
	assert.Equal(t, int32(-1), resp.Events[0].Signal)
	assert.False(t, resp.HasMore)

	resp, err = c.ListEvents(
		context.Background(),
		&tastev1.ListEventsRequest{Domain: "games", Limit: 0, Offset: 0},
	)
	require.NoError(t, err)
	require.Len(t, resp.Events, 1)
	assert.Equal(t, "Outer Wilds", resp.Events[0].Title)

	resp, err = c.ListEvents(
		context.Background(),
		&tastev1.ListEventsRequest{Domain: "", Limit: 0, Offset: 10},
	)
	require.NoError(t, err)
	assert.Empty(t, resp.Events)
	assert.False(t, resp.HasMore)
}

func TestListEvents_InvalidArguments(t *testing.T) {
	c := newClient(t)
	for _, req := range []*tastev1.ListEventsRequest{
		{Domain: "music", Limit: 0, Offset: 0},
		{Domain: "", Limit: 0, Offset: -1},
	} {
		_, err := c.ListEvents(context.Background(), req)
		assert.Equal(t, connect.CodeInvalidArgument, connect.CodeOf(err))
	}
}

func TestSourceErrorIsInternal(t *testing.T) {
	c := newClient(t, userSource{name: "books", events: nil, err: errors.New("db down")})
	_, err := c.GetStats(context.Background(), &tastev1.GetStatsRequest{})
	assert.Equal(t, connect.CodeInternal, connect.CodeOf(err))
	_, err = c.ListEvents(
		context.Background(),
		&tastev1.ListEventsRequest{Domain: "", Limit: 0, Offset: 0},
	)
	assert.Equal(t, connect.CodeInternal, connect.CodeOf(err))
}

// TestSourcesNeedAppAccess: a user sees only the apps they may open; the
// mocked user has recipes access but not books.
func TestSourcesNeedAppAccess(t *testing.T) {
	c := newClientAs(t, sharedmocks.NewMockedAuthService(userID),
		userSource{name: "books", events: []consumption.Event{book}, err: nil},
		userSource{name: "recipes", events: []consumption.Event{game}, err: nil},
		userSource{name: "feeds", events: nil, err: errors.New("never called")},
	)
	resp, err := c.ListEvents(
		context.Background(),
		&tastev1.ListEventsRequest{Domain: "", Limit: 0, Offset: 0},
	)
	require.NoError(t, err)
	require.Len(t, resp.Events, 1)
	assert.Equal(t, "Outer Wilds", resp.Events[0].Title)
}

func TestNew_WarnsOnlyWithoutKey(t *testing.T) {
	var logs bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&logs, nil))
	taste.New(
		sharedmocks.NewMockedAuthService(userID),
		logger,
		testCfg,
		testDB,
		nil,
		noCatalogs(),
	)
	assert.Contains(t, logs.String(), "OPENROUTER_API_KEY not set")

	logs.Reset()
	cfg := testCfg
	cfg.OpenRouterAPIKey = "key"
	taste.New(
		sharedmocks.NewMockedAuthService(userID),
		logger,
		cfg,
		testDB,
		nil,
		noCatalogs(),
	)
	assert.Empty(t, logs.String())
}

// sharedSource is app name's fixed events for family subjects only.
type sharedSource struct {
	name   string
	events []consumption.Event
}

func (s sharedSource) GetName() string { return s.name }

func (s sharedSource) ListConsumption(
	_ context.Context, uid string,
) ([]consumption.Event, error) {
	if !consumption.IsSharedSubject(uid) {
		return nil, nil
	}
	return s.events, nil
}

func TestGetStats_SharedReadsOnlyTheSharedMoviesList(t *testing.T) {
	movie := consumption.Event{
		Domain: consumption.DomainMovies, ItemID: "movie:603", Title: "The Matrix",
		Creator: "", Tags: []string{"sci-fi"}, At: read,
		Signal: consumption.Positive, Reason: "watched",
	}
	c := newClient(t,
		sharedSource{name: "movies", events: []consumption.Event{movie}},
		sharedSource{name: "books", events: []consumption.Event{book}},
		userSource{name: "games", events: []consumption.Event{game}, err: nil},
	)
	ctx := context.Background()

	shared, err := c.ListEvents(ctx, &tastev1.ListEventsRequest{Shared: true})
	require.NoError(t, err)
	require.Len(t, shared.Events, 1, "only movies feed the shared taste")
	assert.Equal(t, "The Matrix", shared.Events[0].Title)

	solo, err := c.ListEvents(ctx, &tastev1.ListEventsRequest{})
	require.NoError(t, err)
	require.Len(t, solo.Events, 1, "shared watches never feed the solo taste")
	assert.Equal(t, "Outer Wilds", solo.Events[0].Title)
}
