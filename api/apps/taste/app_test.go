package taste_test

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"connectrpc.com/connect"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"tools.xdoubleu.com/apps/taste"
	tastev1 "tools.xdoubleu.com/gen/taste/v1"
	"tools.xdoubleu.com/gen/taste/v1/tastev1connect"
	"tools.xdoubleu.com/internal/consumption"
	"tools.xdoubleu.com/internal/logging"
	sharedmocks "tools.xdoubleu.com/internal/mocks"
	"tools.xdoubleu.com/internal/testhelper"
)

const userID = "taste-test-user"

// userSource serves fixed events for userID only.
type userSource struct {
	events []consumption.Event
	err    error
}

func (s userSource) ListConsumption(
	_ context.Context, uid string,
) ([]consumption.Event, error) {
	if uid != userID {
		return nil, nil
	}
	return s.events, s.err
}

func newClient(
	t *testing.T,
	sources ...taste.Source,
) tastev1connect.TasteServiceClient {
	t.Helper()
	a := taste.New(
		sharedmocks.NewMockedAuthService(userID), logging.NewNopLogger(),
		testhelper.NewTestConfig(), sources...,
	)
	ts := httptest.NewServer(testhelper.BuildMux(a))
	t.Cleanup(ts.Close)
	return tastev1connect.NewTasteServiceClient(http.DefaultClient, ts.URL)
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
		sharedmocks.NewMockedAuthService(userID), logging.NewNopLogger(),
		testhelper.NewTestConfig(),
	)
	assert.Equal(t, "taste", a.GetName())
	assert.Equal(t, "Taste", a.GetDisplayName())
	require.NoError(t, a.Start())
	require.NoError(t, a.ApplyMigrations(context.Background(), nil))
}

func TestGetStats(t *testing.T) {
	c := newClient(t,
		userSource{events: []consumption.Event{book}, err: nil},
		userSource{events: []consumption.Event{game}, err: nil},
	)
	resp, err := c.GetStats(context.Background(), connect.NewRequest(
		&tastev1.GetStatsRequest{},
	))
	require.NoError(t, err)

	require.Len(t, resp.Msg.Domains, 6)
	books := resp.Msg.Domains[0]
	assert.Equal(t, "books", books.Domain)
	assert.Equal(t, int32(1), books.Consumed)
	assert.Equal(t, int32(1), books.Positive)
	assert.Equal(t, "2026-03-04T05:06:07Z", books.LastAt)
	games := resp.Msg.Domains[2]
	assert.Equal(t, int32(1), games.Negative)
	assert.Empty(t, games.LastAt)

	assert.Len(t, resp.Msg.Months, 12)
	require.Len(t, resp.Msg.TopTags, 1)
	assert.Equal(t, "sci-fi", resp.Msg.TopTags[0].Name)
	assert.Equal(t, []string{"books"}, resp.Msg.TopTags[0].Domains)
	require.Len(t, resp.Msg.TopCreators, 1)
	assert.Equal(t, "Frank Herbert", resp.Msg.TopCreators[0].Name)
}

func TestGetStats_MonthCounts(t *testing.T) {
	recent := book
	recent.At = time.Now()
	c := newClient(t, userSource{events: []consumption.Event{recent}, err: nil})
	resp, err := c.GetStats(context.Background(), connect.NewRequest(
		&tastev1.GetStatsRequest{},
	))
	require.NoError(t, err)
	last := resp.Msg.Months[11]
	require.Len(t, last.Counts, 1)
	assert.Equal(t, "books", last.Counts[0].Domain)
	assert.Equal(t, int32(1), last.Counts[0].Count)
	assert.Empty(t, resp.Msg.Months[0].Counts)
}

func TestListEvents(t *testing.T) {
	c := newClient(t, userSource{events: []consumption.Event{game, book}, err: nil})
	resp, err := c.ListEvents(context.Background(), connect.NewRequest(
		&tastev1.ListEventsRequest{Domain: "", Limit: 1, Offset: 0},
	))
	require.NoError(t, err)
	require.Len(t, resp.Msg.Events, 1)
	assert.True(t, resp.Msg.HasMore)
	e := resp.Msg.Events[0]
	assert.Equal(t, "Dune", e.Title)
	assert.Equal(t, "books", e.Domain)
	assert.Equal(t, "b1", e.ItemId)
	assert.Equal(t, "Frank Herbert", e.Creator)
	assert.Equal(t, []string{"sci-fi"}, e.Tags)
	assert.Equal(t, "2026-03-04T05:06:07Z", e.At)
	assert.Equal(t, int32(2), e.Signal)
	assert.Equal(t, "rated 5★", e.Reason)

	resp, err = c.ListEvents(context.Background(), connect.NewRequest(
		&tastev1.ListEventsRequest{Domain: "", Limit: 1, Offset: 1},
	))
	require.NoError(t, err)
	require.Len(t, resp.Msg.Events, 1)
	assert.Equal(t, "Outer Wilds", resp.Msg.Events[0].Title)
	assert.Empty(t, resp.Msg.Events[0].At)
	assert.Equal(t, int32(-1), resp.Msg.Events[0].Signal)
	assert.False(t, resp.Msg.HasMore)

	resp, err = c.ListEvents(context.Background(), connect.NewRequest(
		&tastev1.ListEventsRequest{Domain: "games", Limit: 0, Offset: 0},
	))
	require.NoError(t, err)
	require.Len(t, resp.Msg.Events, 1)
	assert.Equal(t, "Outer Wilds", resp.Msg.Events[0].Title)

	resp, err = c.ListEvents(context.Background(), connect.NewRequest(
		&tastev1.ListEventsRequest{Domain: "", Limit: 0, Offset: 10},
	))
	require.NoError(t, err)
	assert.Empty(t, resp.Msg.Events)
	assert.False(t, resp.Msg.HasMore)
}

func TestListEvents_InvalidArguments(t *testing.T) {
	c := newClient(t)
	for _, req := range []*tastev1.ListEventsRequest{
		{Domain: "music", Limit: 0, Offset: 0},
		{Domain: "", Limit: 0, Offset: -1},
	} {
		_, err := c.ListEvents(context.Background(), connect.NewRequest(req))
		assert.Equal(t, connect.CodeInvalidArgument, connect.CodeOf(err))
	}
}

func TestSourceErrorIsInternal(t *testing.T) {
	c := newClient(t, userSource{events: nil, err: errors.New("db down")})
	_, err := c.GetStats(context.Background(), connect.NewRequest(
		&tastev1.GetStatsRequest{},
	))
	assert.Equal(t, connect.CodeInternal, connect.CodeOf(err))
	_, err = c.ListEvents(context.Background(), connect.NewRequest(
		&tastev1.ListEventsRequest{Domain: "", Limit: 0, Offset: 0},
	))
	assert.Equal(t, connect.CodeInternal, connect.CodeOf(err))
}
