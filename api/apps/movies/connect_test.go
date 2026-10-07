package movies_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"connectrpc.com/connect"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	moviesv1 "tools.xdoubleu.com/gen/movies/v1"
	"tools.xdoubleu.com/gen/movies/v1/moviesv1connect"
)

func code(err error) connect.Code {
	var ce *connect.Error
	if errors.As(err, &ce) {
		return ce.Code()
	}
	return connect.CodeUnknown
}

func add(
	t *testing.T,
	c moviesv1connect.MoviesServiceClient,
	mediaType string,
	id int64,
	status string,
) *moviesv1.BacklogEntry {
	t.Helper()
	resp, err := c.AddTitle(context.Background(), connect.NewRequest(
		&moviesv1.AddTitleRequest{MediaType: mediaType, TmdbId: id, Status: status},
	))
	require.NoError(t, err)
	return resp.Msg.Entry
}

func list(
	t *testing.T,
	c moviesv1connect.MoviesServiceClient,
	req *moviesv1.ListBacklogRequest,
) []string {
	t.Helper()
	resp, err := c.ListBacklog(context.Background(), connect.NewRequest(req))
	require.NoError(t, err)
	titles := make([]string, len(resp.Msg.Entries))
	for i, e := range resp.Msg.Entries {
		titles[i] = e.Title
	}
	return titles
}

func TestAddTitle_WantMovie(t *testing.T) {
	c := newClient(t, userID, fakeTMDB{})

	e := add(t, c, "movie", 603, "want")
	assert.Equal(t, "The Matrix", e.Title)
	assert.Equal(t, "movie", e.MediaType)
	assert.Equal(t, "want", e.Status)
	assert.Equal(t, "1999-03-30", e.ReleaseDate)
	assert.Equal(t, []string{"Action", "Science Fiction"}, e.Genres)
	require.NotNil(t, e.Runtime)
	assert.Equal(t, int32(136), *e.Runtime)
	assert.Nil(t, e.SeasonCount)
	assert.Empty(t, e.WatchedAt)
}

func TestAddTitle_WatchedMovieRecordsToday(t *testing.T) {
	c := newClient(t, userID, fakeTMDB{})

	e := add(t, c, "movie", 603, "watched")
	require.Len(t, e.WatchedAt, 1)
	watched, err := time.Parse(time.RFC3339, e.WatchedAt[0])
	require.NoError(t, err)
	assert.WithinDuration(t, time.Now(), watched, time.Minute)

	// Re-adding keeps the recorded watch instead of adding another.
	again := add(t, c, "movie", 603, "watched")
	assert.Equal(t, e.Id, again.Id)
	assert.Len(t, again.WatchedAt, 1)
}

func TestAddTitle_WatchedSeriesHasNoDates(t *testing.T) {
	c := newClient(t, userID, fakeTMDB{})

	e := add(t, c, "series", 1396, "watched")
	assert.Equal(t, "watched", e.Status)
	assert.Empty(t, e.WatchedAt)
	require.NotNil(t, e.SeasonCount)
	assert.Equal(t, int32(5), *e.SeasonCount)
	assert.Nil(t, e.Runtime)
}

func TestAddTitle_UnreleasedHasNoDate(t *testing.T) {
	c := newClient(t, userID, fakeTMDB{})

	e := add(t, c, "movie", 999, "want")
	assert.Equal(t, "", e.ReleaseDate)
}

func TestAddTitle_Errors(t *testing.T) {
	c := newClient(t, userID, fakeTMDB{})
	ctx := context.Background()

	cases := map[string]struct {
		req  *moviesv1.AddTitleRequest
		want connect.Code
	}{
		"bad media type": {
			&moviesv1.AddTitleRequest{MediaType: "book", TmdbId: 603, Status: "want"},
			connect.CodeInvalidArgument,
		},
		"bad status": {
			&moviesv1.AddTitleRequest{MediaType: "movie", TmdbId: 603, Status: "dropped"},
			connect.CodeInvalidArgument,
		},
		"unknown title": {
			&moviesv1.AddTitleRequest{MediaType: "movie", TmdbId: 1, Status: "want"},
			connect.CodeNotFound,
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			_, err := c.AddTitle(ctx, connect.NewRequest(tc.req))
			assert.Equal(t, tc.want, code(err))
		})
	}
}

func TestSearchTitles_MarksBacklogStatus(t *testing.T) {
	c := newClient(t, userID, fakeTMDB{})
	add(t, c, "movie", 603, "watched")

	resp, err := c.SearchTitles(context.Background(), connect.NewRequest(
		&moviesv1.SearchTitlesRequest{Query: "a"},
	))
	require.NoError(t, err)

	byTitle := map[string]*moviesv1.SearchResult{}
	for _, r := range resp.Msg.Results {
		byTitle[r.Title] = r
	}
	require.Contains(t, byTitle, "The Matrix")
	require.NotNil(t, byTitle["The Matrix"].Status)
	assert.Equal(t, "watched", *byTitle["The Matrix"].Status)
	require.Contains(t, byTitle, "Breaking Bad")
	assert.Nil(t, byTitle["Breaking Bad"].Status)
	assert.Equal(t, "series", byTitle["Breaking Bad"].MediaType)
}

func TestSearchTitles_BlankQuery(t *testing.T) {
	c := newClient(t, userID, nil)

	resp, err := c.SearchTitles(context.Background(), connect.NewRequest(
		&moviesv1.SearchTitlesRequest{Query: "  "},
	))
	require.NoError(t, err)
	assert.Empty(t, resp.Msg.Results)
}

func TestNotConfigured(t *testing.T) {
	c := newClient(t, userID, nil)
	ctx := context.Background()

	_, err := c.SearchTitles(ctx, connect.NewRequest(
		&moviesv1.SearchTitlesRequest{Query: "matrix"},
	))
	assert.Equal(t, connect.CodeFailedPrecondition, code(err))

	_, err = c.AddTitle(ctx, connect.NewRequest(
		&moviesv1.AddTitleRequest{MediaType: "movie", TmdbId: 603, Status: "want"},
	))
	assert.Equal(t, connect.CodeFailedPrecondition, code(err))
}

func TestSetStatus(t *testing.T) {
	c := newClient(t, userID, fakeTMDB{})
	ctx := context.Background()
	e := add(t, c, "movie", 129, "want")

	resp, err := c.SetStatus(ctx, connect.NewRequest(
		&moviesv1.SetStatusRequest{Id: e.Id, Status: "watching"},
	))
	require.NoError(t, err)
	assert.Equal(t, "watching", resp.Msg.Entry.Status)
	assert.Empty(t, resp.Msg.Entry.WatchedAt)

	resp, err = c.SetStatus(ctx, connect.NewRequest(
		&moviesv1.SetStatusRequest{Id: e.Id, Status: "watched"},
	))
	require.NoError(t, err)
	assert.Len(t, resp.Msg.Entry.WatchedAt, 1)

	// Leaving and re-entering watched keeps the single recorded watch.
	for _, s := range []string{"dropped", "watched"} {
		resp, err = c.SetStatus(ctx, connect.NewRequest(
			&moviesv1.SetStatusRequest{Id: e.Id, Status: s},
		))
		require.NoError(t, err)
	}
	assert.Len(t, resp.Msg.Entry.WatchedAt, 1)

	_, err = c.SetStatus(ctx, connect.NewRequest(
		&moviesv1.SetStatusRequest{Id: e.Id, Status: "finished"},
	))
	assert.Equal(t, connect.CodeInvalidArgument, code(err))
}

func TestOtherUsersEntriesAreNotFound(t *testing.T) {
	mine := newClient(t, userID, fakeTMDB{})
	theirs := newClient(t, otherUserID, fakeTMDB{})
	ctx := context.Background()
	e := add(t, mine, "movie", 603, "want")

	_, err := theirs.GetTitle(ctx, connect.NewRequest(
		&moviesv1.GetTitleRequest{Id: e.Id},
	))
	assert.Equal(t, connect.CodeNotFound, code(err))

	_, err = theirs.SetStatus(ctx, connect.NewRequest(
		&moviesv1.SetStatusRequest{Id: e.Id, Status: "watched"},
	))
	assert.Equal(t, connect.CodeNotFound, code(err))

	_, err = theirs.RemoveTitle(ctx, connect.NewRequest(
		&moviesv1.RemoveTitleRequest{Id: e.Id},
	))
	assert.Equal(t, connect.CodeNotFound, code(err))

	assert.Empty(t, list(t, theirs, &moviesv1.ListBacklogRequest{}))
}

func TestGetTitle_IncludesOverview(t *testing.T) {
	c := newClient(t, userID, fakeTMDB{})
	e := add(t, c, "series", 1396, "want")

	resp, err := c.GetTitle(context.Background(), connect.NewRequest(
		&moviesv1.GetTitleRequest{Id: e.Id},
	))
	require.NoError(t, err)
	assert.Equal(t, "A teacher turns cook.", resp.Msg.Overview)
	assert.Equal(t, "Breaking Bad", resp.Msg.Entry.Title)
}

func TestInvalidID(t *testing.T) {
	c := newClient(t, userID, fakeTMDB{})
	ctx := context.Background()

	_, err := c.GetTitle(ctx, connect.NewRequest(&moviesv1.GetTitleRequest{Id: "x"}))
	assert.Equal(t, connect.CodeInvalidArgument, code(err))
	_, err = c.SetStatus(ctx, connect.NewRequest(
		&moviesv1.SetStatusRequest{Id: "x", Status: "want"},
	))
	assert.Equal(t, connect.CodeInvalidArgument, code(err))
	_, err = c.RemoveTitle(ctx, connect.NewRequest(&moviesv1.RemoveTitleRequest{Id: "x"}))
	assert.Equal(t, connect.CodeInvalidArgument, code(err))
	_, err = c.GetTitle(ctx, connect.NewRequest(
		&moviesv1.GetTitleRequest{Id: uuid.NewString()},
	))
	assert.Equal(t, connect.CodeNotFound, code(err))
}

func TestRemoveTitle(t *testing.T) {
	c := newClient(t, userID, fakeTMDB{})
	e := add(t, c, "movie", 603, "want")

	_, err := c.RemoveTitle(context.Background(), connect.NewRequest(
		&moviesv1.RemoveTitleRequest{Id: e.Id},
	))
	require.NoError(t, err)
	assert.Empty(t, list(t, c, &moviesv1.ListBacklogRequest{}))
}

func TestListBacklog_FiltersAndSorts(t *testing.T) {
	c := newClient(t, userID, fakeTMDB{})
	add(t, c, "movie", 603, "watched")
	add(t, c, "movie", 129, "want")
	add(t, c, "series", 1396, "want")
	add(t, c, "movie", 999, "want")

	assert.Equal(t,
		[]string{"Avatar 5", "Breaking Bad", "Spirited Away", "The Matrix"},
		list(t, c, &moviesv1.ListBacklogRequest{}),
		"default sort is newest added first",
	)
	assert.Equal(t,
		[]string{"Avatar 5", "Breaking Bad", "Spirited Away", "The Matrix"},
		list(t, c, &moviesv1.ListBacklogRequest{Sort: "title"}),
	)
	assert.Equal(t,
		[]string{"Breaking Bad", "Spirited Away", "The Matrix", "Avatar 5"},
		list(t, c, &moviesv1.ListBacklogRequest{Sort: "release"}),
		"newest release first, undated last",
	)
	assert.Equal(t,
		[]string{"Avatar 5", "Breaking Bad", "Spirited Away"},
		list(t, c, &moviesv1.ListBacklogRequest{Status: "want"}),
	)
	assert.Equal(t,
		[]string{"Breaking Bad"},
		list(t, c, &moviesv1.ListBacklogRequest{MediaType: "series"}),
	)

	resp, err := c.ListBacklog(context.Background(), connect.NewRequest(
		&moviesv1.ListBacklogRequest{Limit: 2},
	))
	require.NoError(t, err)
	assert.Len(t, resp.Msg.Entries, 2)
	assert.True(t, resp.Msg.HasMore)
}

func TestListBacklog_RejectsUnknownFilters(t *testing.T) {
	c := newClient(t, userID, fakeTMDB{})
	ctx := context.Background()

	for _, req := range []*moviesv1.ListBacklogRequest{
		{Status: "finished"},
		{MediaType: "book"},
		{Sort: "rating"},
	} {
		_, err := c.ListBacklog(ctx, connect.NewRequest(req))
		assert.Equal(t, connect.CodeInvalidArgument, code(err))
	}
}
