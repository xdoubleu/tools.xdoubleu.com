package tmdb_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"tools.xdoubleu.com/apps/movies/pkg/tmdb"
)

func TestMain(m *testing.M) {
	tmdb.SetBackoffBase(time.Millisecond)
	os.Exit(m.Run())
}

func serve(t *testing.T, handler http.HandlerFunc) tmdb.Client {
	t.Helper()
	srv := httptest.NewServer(handler)
	tmdb.SetBaseURL(srv.URL)
	t.Cleanup(func() {
		srv.Close()
		tmdb.SetBaseURL("https://api.themoviedb.org/3")
	})
	return tmdb.New("token")
}

func TestSearch_KeepsMoviesAndSeries(t *testing.T) {
	c := serve(t, func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/search/multi", r.URL.Path)
		assert.Equal(t, "dune", r.URL.Query().Get("query"))
		assert.Equal(t, "en-US", r.URL.Query().Get("language"))
		assert.Equal(t, "Bearer token", r.Header.Get("Authorization"))
		_, _ = w.Write([]byte(`{"results":[
			{"media_type":"movie","id":438631,"title":"Dune",
			 "original_title":"Dune","release_date":"2021-09-15",
			 "poster_path":"/d.jpg"},
			{"media_type":"person","id":1,"name":"Someone"},
			{"media_type":"tv","id":90228,"name":"Dune: Prophecy",
			 "original_name":"Dune: Prophecy","first_air_date":""}
		]}`))
	})

	got, err := c.Search(context.Background(), "dune")
	require.NoError(t, err)
	require.Len(t, got, 2)

	assert.Equal(t, tmdb.MediaTypeMovie, got[0].MediaType)
	assert.Equal(t, int64(438631), got[0].TMDBID)
	assert.Equal(t, "Dune", got[0].Title)
	assert.Equal(t, "/d.jpg", got[0].PosterPath)
	require.NotNil(t, got[0].ReleaseDate)
	assert.Equal(t, "2021-09-15", got[0].ReleaseDate.Format(time.DateOnly))

	assert.Equal(t, tmdb.MediaTypeSeries, got[1].MediaType)
	assert.Equal(t, "Dune: Prophecy", got[1].Title)
	assert.Nil(t, got[1].ReleaseDate)
}

func TestGetMovie(t *testing.T) {
	c := serve(t, func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/movie/603", r.URL.Path)
		_, _ = w.Write([]byte(`{"id":603,"title":"The Matrix",
			"original_title":"The Matrix","release_date":"1999-03-30",
			"overview":"Neo.","runtime":136,
			"genres":[{"name":"Action"},{"name":"Science Fiction"}]}`))
	})

	got, err := c.GetMovie(context.Background(), 603)
	require.NoError(t, err)
	assert.Equal(t, "The Matrix", got.Title)
	assert.Equal(t, "Neo.", got.Overview)
	assert.Equal(t, []string{"Action", "Science Fiction"}, got.Genres)
	require.NotNil(t, got.Runtime)
	assert.Equal(t, 136, *got.Runtime)
	assert.Nil(t, got.SeasonCount)
}

func TestGetSeries(t *testing.T) {
	c := serve(t, func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/tv/1396", r.URL.Path)
		_, _ = w.Write([]byte(`{"id":1396,"name":"Breaking Bad",
			"original_name":"Breaking Bad","first_air_date":"2008-01-20",
			"number_of_seasons":5,"genres":[{"name":"Drama"},{"name":""}]}`))
	})

	got, err := c.GetSeries(context.Background(), 1396)
	require.NoError(t, err)
	assert.Equal(t, tmdb.MediaTypeSeries, got.MediaType)
	assert.Equal(t, "Breaking Bad", got.Title)
	require.NotNil(t, got.SeasonCount)
	assert.Equal(t, 5, *got.SeasonCount)
	assert.Nil(t, got.Runtime)
}

func TestGetMovie_ZeroRuntimeIsUnknown(t *testing.T) {
	c := serve(t, func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"id":1,"title":"Short","runtime":0}`))
	})

	got, err := c.GetMovie(context.Background(), 1)
	require.NoError(t, err)
	assert.Nil(t, got.Runtime)
}

func TestGet_NotFound(t *testing.T) {
	c := serve(t, func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	})

	_, err := c.GetMovie(context.Background(), 1)
	require.ErrorIs(t, err, tmdb.ErrNotFound)
}

func TestGet_RetriesRateLimit(t *testing.T) {
	var calls atomic.Int32
	c := serve(t, func(w http.ResponseWriter, _ *http.Request) {
		if calls.Add(1) == 1 {
			w.WriteHeader(http.StatusTooManyRequests)
			return
		}
		_, _ = w.Write([]byte(`{"id":1,"title":"Ok"}`))
	})

	got, err := c.GetMovie(context.Background(), 1)
	require.NoError(t, err)
	assert.Equal(t, "Ok", got.Title)
	assert.Equal(t, int32(2), calls.Load())
}

func TestGet_ClientErrorNotRetried(t *testing.T) {
	var calls atomic.Int32
	c := serve(t, func(w http.ResponseWriter, _ *http.Request) {
		calls.Add(1)
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"status_message":"Invalid API key"}`))
	})

	_, err := c.Search(context.Background(), "x")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "401")
	assert.Equal(t, int32(1), calls.Load())
}

func TestGet_GivesUpAfterMaxAttempts(t *testing.T) {
	var calls atomic.Int32
	c := serve(t, func(w http.ResponseWriter, _ *http.Request) {
		calls.Add(1)
		w.WriteHeader(http.StatusBadGateway)
	})

	_, err := c.GetSeries(context.Background(), 1)
	require.Error(t, err)
	assert.Equal(t, int32(4), calls.Load())
}

func TestGet_TimeoutRetriesUntilDeadline(t *testing.T) {
	release := make(chan struct{})
	c := serve(t, func(_ http.ResponseWriter, _ *http.Request) {
		<-release
	})
	// Registered after serve so it runs first: srv.Close waits on handlers.
	t.Cleanup(func() { close(release) })
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()

	_, err := c.GetMovie(ctx, 1)
	require.ErrorIs(t, err, context.DeadlineExceeded)
}

func TestGet_CanceledContext(t *testing.T) {
	c := serve(t, func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	})
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := c.GetMovie(ctx, 1)
	require.ErrorIs(t, err, context.Canceled)
}
