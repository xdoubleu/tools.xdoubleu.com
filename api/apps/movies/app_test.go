package movies_test

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"tools.xdoubleu.com/apps/movies"
	"tools.xdoubleu.com/apps/movies/pkg/tmdb"
	"tools.xdoubleu.com/gen/movies/v1/moviesv1connect"
	"tools.xdoubleu.com/internal/config"
	"tools.xdoubleu.com/internal/database/postgres"
	"tools.xdoubleu.com/internal/logging"
	sharedmocks "tools.xdoubleu.com/internal/mocks"
	"tools.xdoubleu.com/internal/testhelper"
)

const (
	userID      = "movies-test-user"
	otherUserID = "movies-test-other-user"
)

//nolint:gochecknoglobals //needed for tests
var (
	testDB  postgres.DB
	testCfg config.Config
)

// fakeTMDB serves a fixed catalog; unknown IDs are tmdb.ErrNotFound.
type fakeTMDB struct{}

func date(s string) *time.Time {
	d, _ := time.Parse(time.DateOnly, s)
	return &d
}

func intPtr(n int) *int { return &n }

//nolint:gochecknoglobals //fixed fake catalog
var catalog = []tmdb.Title{
	{
		MediaType: tmdb.MediaTypeMovie, TMDBID: 603, Title: "The Matrix",
		OriginalTitle: "The Matrix", ReleaseDate: date("1999-03-30"),
		PosterPath: "/matrix.jpg", Overview: "Neo learns the truth.",
		Genres: []string{"Action", "Science Fiction"}, Runtime: intPtr(136),
		SeasonCount: nil, Seasons: nil,
	},
	{
		MediaType: tmdb.MediaTypeMovie, TMDBID: 129, Title: "Spirited Away",
		OriginalTitle: "千と千尋の神隠し", ReleaseDate: date("2001-07-20"),
		PosterPath: "/spirited.jpg", Overview: "A girl in a spirit world.",
		Genres: []string{"Animation"}, Runtime: intPtr(125), SeasonCount: nil,
		Seasons: nil,
	},
	{
		MediaType: tmdb.MediaTypeSeries, TMDBID: 1396, Title: "Breaking Bad",
		OriginalTitle: "Breaking Bad", ReleaseDate: date("2008-01-20"),
		PosterPath: "/bb.jpg", Overview: "A teacher turns cook.",
		Genres: []string{"Drama"}, Runtime: nil, SeasonCount: intPtr(4),
		Seasons: []tmdb.Season{
			{Number: 0, Name: "Specials", AirDate: nil, EpisodeCount: 9},
			{Number: 1, Name: "Season 1", AirDate: date("2008-01-20"), EpisodeCount: 7},
			{Number: 2, Name: "Season 2", AirDate: date("2009-03-08"), EpisodeCount: 13},
			{Number: 3, Name: "Season 3", AirDate: date("2099-01-01"), EpisodeCount: 0},
			{Number: 4, Name: "Season 4", AirDate: nil, EpisodeCount: 0},
		},
	},
	{
		MediaType: tmdb.MediaTypeSeries, TMDBID: 70523, Title: "Dark",
		OriginalTitle: "Dark", ReleaseDate: date("2017-12-01"), PosterPath: "",
		Overview: "", Genres: nil, Runtime: nil, SeasonCount: intPtr(2),
		Seasons: []tmdb.Season{
			{Number: 1, Name: "Season 1", AirDate: date("2017-12-01"), EpisodeCount: 10},
			{Number: 2, Name: "Season 2", AirDate: date("2019-06-21"), EpisodeCount: 8},
		},
	},
	{
		MediaType: tmdb.MediaTypeSeries, TMDBID: 136315, Title: "The Bear",
		OriginalTitle: "The Bear", ReleaseDate: date("2022-06-23"), PosterPath: "",
		Overview: "", Genres: nil, Runtime: nil, SeasonCount: intPtr(2),
		Seasons: []tmdb.Season{
			{Number: 1, Name: "Season 1", AirDate: date("2022-06-23"), EpisodeCount: 8},
			{Number: 2, Name: "Season 2", AirDate: date("2099-06-22"), EpisodeCount: 0},
		},
	},
	{
		MediaType: tmdb.MediaTypeSeries, TMDBID: 95396, Title: "Severance",
		OriginalTitle: "Severance", ReleaseDate: date("2022-02-18"), PosterPath: "",
		Overview: "", Genres: nil, Runtime: nil, SeasonCount: intPtr(1),
		Seasons: []tmdb.Season{
			{Number: 1, Name: "Season 1", AirDate: date("2022-02-18"), EpisodeCount: 9},
		},
	},
	{
		MediaType: tmdb.MediaTypeSeries, TMDBID: 83867, Title: "Andor",
		OriginalTitle: "Andor", ReleaseDate: date("2022-09-21"), PosterPath: "",
		Overview: "", Genres: nil, Runtime: nil, SeasonCount: intPtr(1),
		Seasons: []tmdb.Season{
			{Number: 1, Name: "Season 1", AirDate: date("2022-09-21"), EpisodeCount: 12},
		},
	},
	{
		MediaType: tmdb.MediaTypeMovie, TMDBID: 999, Title: "Avatar 5",
		OriginalTitle: "Avatar 5", ReleaseDate: nil, PosterPath: "",
		Overview: "", Genres: nil, Runtime: nil, SeasonCount: nil, Seasons: nil,
	},
}

func (fakeTMDB) Search(_ context.Context, query string) ([]tmdb.Title, error) {
	var out []tmdb.Title
	for _, t := range catalog {
		if strings.Contains(strings.ToLower(t.Title), strings.ToLower(query)) {
			t.Overview, t.Genres, t.Runtime, t.SeasonCount = "", nil, nil, nil
			t.Seasons = nil
			out = append(out, t)
		}
	}
	return out, nil
}

func (fakeTMDB) find(mediaType string, id int64) (*tmdb.Title, error) {
	for _, t := range catalog {
		if t.MediaType == mediaType && t.TMDBID == id {
			return &t, nil
		}
	}
	return nil, tmdb.ErrNotFound
}

func (f fakeTMDB) GetMovie(_ context.Context, id int64) (*tmdb.Title, error) {
	return f.find(tmdb.MediaTypeMovie, id)
}

func (f fakeTMDB) GetSeries(_ context.Context, id int64) (*tmdb.Title, error) {
	return f.find(tmdb.MediaTypeSeries, id)
}

// seasonlessTMDB returns series without seasons, as a pre-seasons add did.
type seasonlessTMDB struct{ fakeTMDB }

func (f seasonlessTMDB) GetSeries(ctx context.Context, id int64) (*tmdb.Title, error) {
	t, err := f.fakeTMDB.GetSeries(ctx, id)
	if t != nil {
		t.Seasons = nil
	}
	return t, err
}

// downTMDB fails every series lookup.
type downTMDB struct{ fakeTMDB }

func (downTMDB) GetSeries(context.Context, int64) (*tmdb.Title, error) {
	return nil, errors.New("tmdb down")
}

func TestMain(m *testing.M) {
	testCfg = testhelper.NewTestConfig()
	pool := testhelper.ConnectTestDB(testCfg.DBDsn)
	testDB = pool

	ctx := context.Background()
	if _, err := testDB.Exec(ctx, "DROP SCHEMA IF EXISTS movies CASCADE"); err != nil {
		panic(err)
	}
	if err := newApp(userID, fakeTMDB{}).ApplyMigrations(ctx, pool); err != nil {
		panic(err)
	}

	os.Exit(m.Run())
}

func newApp(uid string, client tmdb.Client) *movies.Movies {
	return movies.NewWithTMDB(
		sharedmocks.NewMockedAuthService(uid),
		logging.NewNopLogger(),
		testCfg,
		testDB,
		client,
	)
}

// newClient serves an app acting as uid; the user's backlog is emptied first.
func newClient(
	t *testing.T,
	uid string,
	client tmdb.Client,
) moviesv1connect.MoviesServiceClient {
	t.Helper()
	_, err := testDB.Exec(context.Background(),
		"DELETE FROM movies.user_titles WHERE user_id = $1", uid)
	if err != nil {
		t.Fatal(err)
	}
	return newKeepingClient(t, uid, client)
}

// newKeepingClient serves an app acting as uid without clearing its backlog.
func newKeepingClient(
	t *testing.T,
	uid string,
	client tmdb.Client,
) moviesv1connect.MoviesServiceClient {
	t.Helper()
	ts := httptest.NewServer(testhelper.BuildMux(newApp(uid, client)))
	t.Cleanup(ts.Close)
	return moviesv1connect.NewMoviesServiceClient(http.DefaultClient, ts.URL)
}

func TestAppMetadata(t *testing.T) {
	a := newApp(userID, nil)
	assert.Equal(t, "movies", a.GetName())
	assert.Equal(t, "Movies & Series", a.GetDisplayName())
	assert.NoError(t, a.Start())
}

func TestNew_WithoutKeyDisablesTMDB(t *testing.T) {
	cfg := testCfg
	cfg.TMDBAPIKey = ""
	a := movies.New(
		sharedmocks.NewMockedAuthService(userID), logging.NewNopLogger(), cfg, testDB,
	)
	assert.NotNil(t, a)

	cfg.TMDBAPIKey = "token"
	assert.NotNil(t, movies.New(
		sharedmocks.NewMockedAuthService(userID), logging.NewNopLogger(), cfg, testDB,
	))
}
