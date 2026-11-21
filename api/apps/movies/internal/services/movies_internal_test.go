//nolint:exhaustruct // sparse fakes
package services

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"tools.xdoubleu.com/apps/internal/consumption"
	"tools.xdoubleu.com/apps/movies/internal/models"
	"tools.xdoubleu.com/apps/movies/pkg/tmdb"
)

// fakeStore serves one entry; embedding the interface leaves any other call
// to panic.
type fakeStore struct {
	moviesStore
	entry   models.Entry
	seasons []models.Season
	ticked  map[int][]*time.Time
	status  string
}

func (f *fakeStore) GetEntry(
	context.Context, string, uuid.UUID,
) (*models.Entry, error) {
	e := f.entry
	if f.status != "" {
		e.Status = f.status
	}
	return &e, nil
}

func (f *fakeStore) ListProviders(
	context.Context, uuid.UUID,
) ([]models.Provider, error) {
	return nil, nil
}

func (f *fakeStore) GetProviderIDs(context.Context, string) ([]int64, error) {
	return nil, nil
}

func (f *fakeStore) ListSeasons(
	context.Context, string, uuid.UUID,
) ([]models.Season, error) {
	out := make([]models.Season, len(f.seasons))
	for i, s := range f.seasons {
		s.WatchedAt = f.ticked[s.Number]
		out[i] = s
	}
	return out, nil
}

func (f *fakeStore) SetStatus(
	_ context.Context, _ string, _ uuid.UUID, st string, _ bool,
) error {
	f.status = st
	return nil
}

func (f *fakeStore) SetSeasonWatchedAt(
	_ context.Context, _ string, _ uuid.UUID, number int, at []*time.Time,
) error {
	f.ticked[number] = at
	return nil
}

type fakeFamily struct {
	id    uuid.UUID
	err   error
	calls int
}

func (f *fakeFamily) EnsureFamily(context.Context, string) (uuid.UUID, error) {
	f.calls++
	return f.id, f.err
}

func day(s string) *time.Time {
	d, _ := time.Parse(time.DateOnly, s)
	return &d
}

var clock = day("2024-06-01") //nolint:gochecknoglobals // fixed test clock

func seriesStore() *fakeStore {
	return &fakeStore{
		entry: models.Entry{
			ID: uuid.New(),
			Title: models.Title{
				MediaType: tmdb.MediaTypeSeries, ProvidersFetched: true,
			},
			Status: models.StatusWant,
		},
		seasons: []models.Season{
			{Number: 0, Name: "Specials", AirDate: day("2020-01-01")},
			{Number: 1, Name: "One", AirDate: day("2023-01-01")},
			{Number: 2, Name: "Two", AirDate: day("2025-01-01")},
			{Number: 3, Name: "Undated"},
		},
		ticked: map[int][]*time.Time{},
	}
}

func newTestService(
	store *fakeStore, fam *fakeFamily, now time.Time,
) *MovieService {
	return NewMovieService(store, fam, nil, func() time.Time { return now })
}

func TestOwner(t *testing.T) {
	ctx := context.Background()
	famID := uuid.New()

	t.Run("solo skips the family", func(t *testing.T) {
		fam := &fakeFamily{id: famID}
		owner, err := newTestService(nil, fam, *clock).Owner(ctx, "u1", false)
		require.NoError(t, err)
		assert.Equal(t, "u1", owner)
		assert.Zero(t, fam.calls)
	})

	t.Run("shared is the family subject", func(t *testing.T) {
		fam := &fakeFamily{id: famID}
		owner, err := newTestService(nil, fam, *clock).Owner(ctx, "u1", true)
		require.NoError(t, err)
		assert.Equal(t, consumption.SharedSubject(famID), owner)
		assert.Equal(t, 1, fam.calls)
	})

	t.Run("family failure", func(t *testing.T) {
		boom := errors.New("boom")
		_, err := newTestService(nil, &fakeFamily{err: boom}, *clock).
			Owner(ctx, "u1", true)
		require.ErrorIs(t, err, boom)
	})
}

func TestGet_FlagsAiredSeasonsByClock(t *testing.T) {
	ctx := context.Background()
	store := seriesStore()

	aired := func(now time.Time) []bool {
		entry, err := newTestService(store, nil, now).Get(ctx, "u1", store.entry.ID)
		require.NoError(t, err)
		out := make([]bool, len(entry.Seasons))
		for i, s := range entry.Seasons {
			out[i] = s.HasAired
		}
		return out
	}

	assert.Equal(t, []bool{true, true, false, false}, aired(*clock))
	assert.Equal(t, []bool{true, true, true, false}, aired(*day("2025-01-01")))
}

func TestSetStatusWatched_TicksOnlySeasonsAiredByClock(t *testing.T) {
	store := seriesStore()
	s := newTestService(store, nil, *clock)

	entry, err := s.SetStatus(
		context.Background(), "u1", store.entry.ID, models.StatusWatched, false,
	)
	require.NoError(t, err)

	assert.Len(t, store.ticked[1], 1, "aired season ticked")
	assert.Empty(t, store.ticked[0], "specials never count")
	assert.Empty(t, store.ticked[2], "future season stays unticked")
	assert.Empty(t, store.ticked[3], "undated season stays unticked")
	require.NotNil(t, store.ticked[1][0])
	assert.True(t, store.ticked[1][0].Equal(*clock), "new watch is dated by the clock")
	assert.Equal(t, models.StatusWatched, entry.Status)
}

func TestSetSeasonWatched_StatusFollowsAiredSeasons(t *testing.T) {
	store := seriesStore()
	s := newTestService(store, nil, *clock)

	entry, err := s.SetSeasonWatched(
		context.Background(), "u1", store.entry.ID, 1, true, true,
	)
	require.NoError(t, err)
	assert.Equal(t, models.StatusWatched, entry.Status, "the only aired season is ticked")

	entry, err = newTestService(store, nil, *day("2025-02-01")).SetSeasonWatched(
		context.Background(), "u1", store.entry.ID, 2, false, true,
	)
	require.NoError(t, err)
	assert.Equal(t, models.StatusWatched, entry.Status,
		"unticking an unticked season is a no-op")
}

func TestNewTitleChange(t *testing.T) {
	five, six := 5, 6

	cases := map[string]struct {
		status    string
		rating    *int
		watchedAt string
		want      string
	}{
		"empty is fine":  {},
		"valid":          {status: "watched", rating: &five, watchedAt: "2024-01-02"},
		"bad status":     {status: "seen", want: "status must be"},
		"bad rating":     {rating: &six, want: "rating must be"},
		"bad date":       {watchedAt: "yesterday", want: "YYYY-MM-DD"},
		"zero clears":    {rating: new(int)},
		"negative":       {rating: new(-1), want: "rating must be"},
		"valid watching": {status: "watching"},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			change, err := NewTitleChange(tc.status, tc.rating, tc.watchedAt)
			if tc.want != "" {
				require.ErrorContains(t, err, tc.want)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tc.status, change.Status)
			assert.Equal(t, tc.watchedAt != "", change.WatchedAt != nil)
		})
	}
}

func TestApplyChange_Rules(t *testing.T) {
	movie := &models.Entry{
		ID:     uuid.New(),
		Title:  models.Title{MediaType: tmdb.MediaTypeMovie},
		Status: models.StatusWant,
	}
	series := &models.Entry{
		ID:     uuid.New(),
		Title:  models.Title{MediaType: tmdb.MediaTypeSeries},
		Status: models.StatusWatched,
	}
	s := newTestService(nil, nil, *clock)
	ctx := context.Background()

	_, err := s.applyChange(ctx, "u1", series, TitleChange{WatchedAt: day("2024-01-01")})
	require.ErrorContains(t, err, "watched_at is for movies")

	_, err = s.applyChange(ctx, "u1", movie, TitleChange{WatchedAt: day("2024-01-01")})
	require.ErrorContains(t, err, "needs status watched")

	_, err = s.applyChange(ctx, "u1", movie, TitleChange{
		Status: models.StatusWant, WatchedAt: day("2024-01-01"),
	})
	require.ErrorContains(t, err, "needs status watched")

	// An unchanged status with no other change touches nothing.
	got, err := s.applyChange(ctx, "u1", movie, TitleChange{Status: models.StatusWant})
	require.NoError(t, err)
	assert.Same(t, movie, got)
}

func TestTickSeason_Rules(t *testing.T) {
	movie := &models.Entry{Title: models.Title{MediaType: tmdb.MediaTypeMovie}}
	series := &models.Entry{Title: models.Title{MediaType: tmdb.MediaTypeSeries}}
	s := newTestService(nil, nil, *clock)
	ctx := context.Background()

	_, err := s.tickSeason(ctx, "u1", movie, SeasonTick{Number: 1, Watched: true}, nil)
	require.ErrorContains(t, err, "only applies to series")

	_, err = s.tickSeason(
		ctx, "u1", series, SeasonTick{Number: 1, Watched: false}, day("2024-01-01"),
	)
	require.ErrorContains(t, err, "needs season_watched true")
}
