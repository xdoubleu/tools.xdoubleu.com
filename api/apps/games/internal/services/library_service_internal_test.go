//nolint:exhaustruct // fixtures set only the fields under test
package services

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"tools.xdoubleu.com/apps/games/internal/models"
	"tools.xdoubleu.com/apps/internal/consumption"
	"tools.xdoubleu.com/apps/internal/progresshistory"
	"tools.xdoubleu.com/internal/kit/database"
)

var errStore = errors.New("store failed")

// fakeSteamStore serves canned reads; fail names the method that errors.
type fakeSteamStore struct {
	SteamStore
	games        []models.Game
	backlog      []models.Game
	inProgress   []models.Game
	completed    []models.Game
	delisted     []models.Game
	averaged     []models.Game
	achievements map[int][]models.Achievement
	wishlist     []models.WishlistItem
	fail         string
}

func (f *fakeSteamStore) err(name string) error {
	if f.fail == name {
		return errStore
	}
	return nil
}

func (f *fakeSteamStore) GetAllGames(context.Context, string) ([]models.Game, error) {
	return f.games, f.err("GetAllGames")
}

func (f *fakeSteamStore) GetAveragedGames(
	context.Context, string,
) ([]models.Game, error) {
	return f.averaged, f.err("GetAveragedGames")
}

func (f *fakeSteamStore) GetBacklog(context.Context, string) ([]models.Game, error) {
	return f.backlog, f.err("GetBacklog")
}

func (f *fakeSteamStore) GetInProgress(context.Context, string) ([]models.Game, error) {
	return f.inProgress, f.err("GetInProgress")
}

func (f *fakeSteamStore) GetCompleted(context.Context, string) ([]models.Game, error) {
	return f.completed, f.err("GetCompleted")
}

func (f *fakeSteamStore) GetDelisted(context.Context, string) ([]models.Game, error) {
	return f.delisted, f.err("GetDelisted")
}

func (f *fakeSteamStore) GetGameByID(
	_ context.Context, id int, _ string,
) (*models.Game, error) {
	if err := f.err("GetGameByID"); err != nil {
		return nil, err
	}
	for _, g := range append(f.games, f.backlog...) {
		if g.ID == id {
			return &g, nil
		}
	}
	return nil, database.ErrResourceNotFound
}

func (f *fakeSteamStore) GetAchievementsForGames(
	context.Context, []int, string,
) (map[int][]models.Achievement, error) {
	return f.achievements, f.err("GetAchievementsForGames")
}

func (f *fakeSteamStore) GetWishlist(
	context.Context, string,
) ([]models.WishlistItem, error) {
	return f.wishlist, f.err("GetWishlist")
}

// fakeProgressStore serves a fixed latest value and history.
type fakeProgressStore struct {
	latest    string
	latestErr error
	records   []progresshistory.Record
	fail      bool
}

func (f *fakeProgressStore) GetLatest(context.Context, string) (string, error) {
	return f.latest, f.latestErr
}

func (f *fakeProgressStore) Upsert(
	context.Context, string, []string, []string,
) error {
	return nil
}

func (f *fakeProgressStore) GetByDates(
	context.Context, string, time.Time, time.Time,
) ([]progresshistory.Record, error) {
	if f.fail {
		return nil, errStore
	}
	return f.records, nil
}

func (f *fakeProgressStore) GetLastValueBefore(
	context.Context, string, time.Time,
) (string, error) {
	return "", database.ErrResourceNotFound
}

func newLibrary(
	steam *fakeSteamStore,
	progress *fakeProgressStore,
) *LibraryService {
	//nolint:exhaustruct // only the fields these paths read
	steamSvc := &SteamService{steam: steam}
	return NewLibraryService(steamSvc, NewProgressService(progress, steamSvc))
}

func TestLibraryGetSteam(t *testing.T) {
	ctx := context.Background()
	start := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	end := start.AddDate(0, 0, 1)

	steam := &fakeSteamStore{
		backlog:    []models.Game{{ID: 1}, {ID: 2}},
		inProgress: []models.Game{{ID: 3}},
		completed:  []models.Game{{ID: 4}},
		delisted:   []models.Game{{ID: 5}},
		averaged:   []models.Game{{ID: 3, CompletionRate: "100.00"}},
	}
	progress := &fakeProgressStore{
		latestErr: database.ErrResourceNotFound,
		records:   []progresshistory.Record{{Date: start, Value: "5.00"}},
	}

	o, err := newLibrary(steam, progress).GetSteam(ctx, "u", start, end)
	require.NoError(t, err)
	assert.Equal(t, 3, o.TotalBacklog)
	assert.Len(t, o.Completed, 1)
	assert.Len(t, o.Delisted, 1)
	assert.Equal(t, "0.00", o.CurrentRate)
	assert.Equal(t, 1, o.Distribution[len(o.Distribution)-1])
	assert.NotEmpty(t, o.Labels)
	assert.Equal(t, start, o.DateStart)
	assert.Equal(t, end, o.DateEnd)
}

func TestLibraryGetSteam_Errors(t *testing.T) {
	ctx := context.Background()
	now := time.Now()

	for _, fail := range []string{
		"GetBacklog", "GetInProgress", "GetAveragedGames", "GetCompleted",
		"GetDelisted",
	} {
		t.Run(fail, func(t *testing.T) {
			_, err := newLibrary(
				&fakeSteamStore{fail: fail}, &fakeProgressStore{},
			).GetSteam(ctx, "u", now, now)
			require.ErrorIs(t, err, errStore)
		})
	}

	t.Run("latest rate", func(t *testing.T) {
		_, err := newLibrary(
			&fakeSteamStore{}, &fakeProgressStore{latestErr: errStore},
		).GetSteam(ctx, "u", now, now)
		require.ErrorIs(t, err, errStore)
	})

	t.Run("history", func(t *testing.T) {
		_, err := newLibrary(
			&fakeSteamStore{}, &fakeProgressStore{fail: true},
		).GetSteam(ctx, "u", now, now)
		require.ErrorIs(t, err, errStore)
	})
}

func TestLibraryGetSteamGame_SortsAchievements(t *testing.T) {
	ctx := context.Background()
	pct := func(v float64) *float64 { return &v }
	steam := &fakeSteamStore{
		games: []models.Game{{ID: 1, Name: "G"}},
		achievements: map[int][]models.Achievement{1: {
			{DisplayName: "none-b"},
			{DisplayName: "rare", GlobalPercent: pct(2)},
			{DisplayName: "none-a"},
			{DisplayName: "common", GlobalPercent: pct(90)},
		}},
	}

	d, err := newLibrary(steam, &fakeProgressStore{}).GetSteamGame(ctx, "u", 1)
	require.NoError(t, err)
	assert.Equal(t, "G", d.Game.Name)
	names := make([]string, len(d.Achievements))
	for i, a := range d.Achievements {
		names[i] = a.DisplayName
	}
	assert.Equal(t, []string{"common", "rare", "none-a", "none-b"}, names)

	_, err = newLibrary(steam, &fakeProgressStore{}).GetSteamGame(ctx, "u", 99)
	require.ErrorIs(t, err, database.ErrResourceNotFound)

	steam.fail = "GetAchievementsForGames"
	_, err = newLibrary(steam, &fakeProgressStore{}).GetSteamGame(ctx, "u", 1)
	require.ErrorIs(t, err, errStore)
}

func TestLibraryKnows(t *testing.T) {
	ctx := context.Background()
	steam := &fakeSteamStore{
		games:    []models.Game{{ID: 1}},
		wishlist: []models.WishlistItem{{ID: 2}},
	}
	lib := newLibrary(steam, &fakeProgressStore{})

	for id, want := range map[int]bool{1: true, 2: true, 3: false} {
		got, err := lib.Knows(ctx, "u", id)
		require.NoError(t, err)
		assert.Equal(t, want, got, id)
	}

	steam.fail = "GetWishlist"
	_, err := lib.Knows(ctx, "u", 3)
	require.ErrorIs(t, err, errStore)

	steam.fail = "GetGameByID"
	_, err = lib.Knows(ctx, "u", 1)
	require.ErrorIs(t, err, errStore)
}

func TestLibraryListConsumption(t *testing.T) {
	ctx := context.Background()
	played := time.Date(2026, 4, 1, 20, 0, 0, 0, time.UTC)
	steam := &fakeSteamStore{
		games: []models.Game{
			{
				ID: 1, Name: "Favourite", Playtime: 600, CompletionRate: "10.00",
				Favourite: true, InCompletionAverage: true, LastPlayed: &played,
			},
			{ID: 2, Name: "Bounced", Playtime: 30, InCompletionAverage: true},
			{ID: 3, Name: "Unplayed", InCompletionAverage: true},
			{ID: 4, Name: "Superseded", Playtime: 900},
		},
		wishlist: []models.WishlistItem{{ID: 9, Name: "Wished", AddedAt: &played}},
	}

	events, err := newLibrary(steam, &fakeProgressStore{}).ListConsumption(ctx, "u")
	require.NoError(t, err)

	got := map[string]consumption.Event{}
	for _, e := range events {
		got[e.Title] = e
	}
	require.Len(t, got, 3)
	assert.Equal(t, consumption.StrongPositive, got["Favourite"].Signal)
	assert.Equal(t, played, got["Favourite"].At)
	assert.Equal(t, consumption.Negative, got["Bounced"].Signal)
	assert.True(t, got["Bounced"].At.IsZero())
	assert.Equal(t, consumption.Positive, got["Wished"].Signal)
	assert.Equal(t, "9", got["Wished"].ItemID)

	steam.fail = "GetWishlist"
	_, err = newLibrary(steam, &fakeProgressStore{}).ListConsumption(ctx, "u")
	require.ErrorIs(t, err, errStore)

	steam.fail = "GetAllGames"
	_, err = newLibrary(steam, &fakeProgressStore{}).ListConsumption(ctx, "u")
	require.ErrorIs(t, err, errStore)
}
