package services

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"tools.xdoubleu.com/apps/games/internal/models"
	"tools.xdoubleu.com/apps/games/pkg/steam"
)

func discardLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

// stubSteamClient implements steam.Client with per-call overrides.
type stubSteamClient struct {
	ownedGames func(
		ctx context.Context,
		steamID string,
	) (*steam.OwnedGamesResponse, error)
	playerAchievements func(
		ctx context.Context,
		steamID string,
		appID int,
	) (*steam.AchievementsResponse, error)
	schemaForGame func(
		ctx context.Context,
		appID int,
	) (*steam.GetSchemaForGameResponse, error)
	globalPercents func(
		ctx context.Context,
		appID int,
	) (*steam.GlobalAchievementPercentagesResponse, error)
}

func (c stubSteamClient) GetOwnedGames(
	ctx context.Context,
	steamID string,
) (*steam.OwnedGamesResponse, error) {
	return c.ownedGames(ctx, steamID)
}

func (c stubSteamClient) GetPlayerAchievements(
	ctx context.Context,
	steamID string,
	appID int,
) (*steam.AchievementsResponse, error) {
	return c.playerAchievements(ctx, steamID, appID)
}

func (c stubSteamClient) GetSchemaForGame(
	ctx context.Context,
	appID int,
) (*steam.GetSchemaForGameResponse, error) {
	return c.schemaForGame(ctx, appID)
}

func (c stubSteamClient) GetGlobalAchievementPercentagesForApp(
	ctx context.Context,
	appID int,
) (*steam.GlobalAchievementPercentagesResponse, error) {
	return c.globalPercents(ctx, appID)
}

func gameWith(id int, delisted bool) *models.Game {
	//nolint:exhaustruct //only the fields the membership rule reads
	return &models.Game{ID: id, IsDelisted: delisted}
}

func rows(names ...string) []models.Achievement {
	out := make([]models.Achievement, 0, len(names))
	for _, name := range names {
		//nolint:exhaustruct //only the field the membership rule reads
		out = append(out, models.Achievement{Name: name})
	}
	return out
}

// TestMarkCompletionAverageMembership covers the cases a game must not be
// read as superseded (partial overlap, set spread across games) plus the
// accepted false positive (docs/adr-0018-completion-average-population.md).
func TestMarkCompletionAverageMembership(t *testing.T) {
	const listed, other, delisted = 1, 2, 3

	tests := []struct {
		name         string
		listedRows   []models.Achievement
		otherRows    []models.Achievement
		delistedRows []models.Achievement
		want         bool
	}{
		{
			name:         "taken over by a listed game",
			listedRows:   rows("BASE_1", "BASE_2", "EP_1", "EP_2"),
			otherRows:    rows("O_1"),
			delistedRows: rows("EP_1", "EP_2"),
			want:         false,
		},
		{
			name:         "nothing carries its achievements",
			listedRows:   rows("BASE_1", "BASE_2"),
			otherRows:    rows("O_1"),
			delistedRows: rows("GMS_1", "GMS_2"),
			want:         true,
		},
		{
			name:         "only part of it was taken over",
			listedRows:   rows("BASE_1", "EP_1"),
			otherRows:    rows("O_1"),
			delistedRows: rows("EP_1", "EP_2"),
			want:         true,
		},
		{
			name:         "its achievements are spread over two listed games",
			listedRows:   rows("EP_1"),
			otherRows:    rows("EP_2"),
			delistedRows: rows("EP_1", "EP_2"),
			want:         true,
		},
		{
			// Accepted limitation: per-app name uniqueness allows this
			// false positive.
			name:         "wholly contained in one game, however generic",
			listedRows:   rows("ACH_01", "ACH_02", "ACH_03"),
			otherRows:    rows("O_1"),
			delistedRows: rows("ACH_01", "ACH_02"),
			want:         false,
		},
		{
			name:         "no stored achievements at all",
			listedRows:   rows("BASE_1"),
			otherRows:    rows("O_1"),
			delistedRows: nil,
			want:         true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gamesMap := map[int]*models.Game{
				listed:   gameWith(listed, false),
				other:    gameWith(other, false),
				delisted: gameWith(delisted, true),
			}
			achievements := map[int][]models.Achievement{
				listed:   tt.listedRows,
				other:    tt.otherRows,
				delisted: tt.delistedRows,
			}

			markCompletionAverageMembership(gamesMap, achievements)

			assert.True(t, gamesMap[listed].InCompletionAverage,
				"a game Steam still lists always counts")
			assert.True(t, gamesMap[other].InCompletionAverage,
				"a game Steam still lists always counts")
			assert.Equal(t, tt.want, gamesMap[delisted].InCompletionAverage)
		})
	}
}

// TestAveragedAchievementsKeepsGamesBeingRefreshed: a game absent from
// gamesMap is mid-refresh, not excluded.
func TestAveragedAchievementsKeepsGamesBeingRefreshed(t *testing.T) {
	gamesMap := map[int]*models.Game{1: gameWith(1, false)}
	gamesMap[1].InCompletionAverage = true

	out := averagedAchievements(map[int][]models.Achievement{
		1: rows("A"),
		9: rows("B"),
	}, gamesMap)

	assert.Len(t, out, 2, "a game not yet read back is kept")
}

func TestRtimeLastPlayedToTime(t *testing.T) {
	assert.Nil(t, rtimeLastPlayedToTime(0), "0 means never played")

	got := rtimeLastPlayedToTime(1700000000)
	require.NotNil(t, got)
	assert.Equal(t, int64(1700000000), got.Unix())
	assert.Equal(t, time.UTC, got.Location())
}

func TestPercentPtr(t *testing.T) {
	percents := map[string]float64{"TEST": 42.5}

	got := percentPtr(percents, "TEST")
	require.NotNil(t, got)
	assert.InEpsilon(t, 42.5, *got, 0.0001)

	assert.Nil(t, percentPtr(percents, "MISSING"))
}

func TestContainsAll(t *testing.T) {
	tests := map[string]struct {
		super map[string]struct{}
		sub   map[string]struct{}
		want  bool
	}{
		"empty sub is always contained": {
			super: map[string]struct{}{"A": {}},
			sub:   map[string]struct{}{},
			want:  true,
		},
		"sub larger than super": {
			super: map[string]struct{}{"A": {}},
			sub:   map[string]struct{}{"A": {}, "B": {}},
			want:  false,
		},
		"equal sets": {
			super: map[string]struct{}{"A": {}, "B": {}},
			sub:   map[string]struct{}{"A": {}, "B": {}},
			want:  true,
		},
		"super has extra, sub fully contained": {
			super: map[string]struct{}{"A": {}, "B": {}, "C": {}},
			sub:   map[string]struct{}{"A": {}, "B": {}},
			want:  true,
		},
		"same size, missing member": {
			super: map[string]struct{}{"A": {}, "C": {}},
			sub:   map[string]struct{}{"A": {}, "B": {}},
			want:  false,
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			assert.Equal(t, tt.want, containsAll(tt.super, tt.sub))
		})
	}
}

func TestBuildAchievementRows_UsesPlayerAchievements(t *testing.T) {
	unlockTime := int64(1700000000)
	//nolint:exhaustruct //only the fields buildAchievementRows reads
	player := []steam.Achievement{
		{APIName: "A_ACHIEVED", Achieved: 1, UnlockTime: unlockTime},
		{APIName: "A_NOT_ACHIEVED", Achieved: 0, UnlockTime: 0},
	}
	//nolint:exhaustruct //only the fields buildAchievementRows reads
	schemas := []steam.AchievementSchema{
		{Name: "A_ACHIEVED", DisplayName: "Achieved", Description: "d1", Icon: "i1"},
		{
			Name:        "A_NOT_ACHIEVED",
			DisplayName: "Not Achieved",
			Description: "d2",
			Icon:        "i2",
		},
	}
	schemaMap := map[string]steam.AchievementSchema{}
	for _, s := range schemas {
		schemaMap[s.Name] = s
	}
	globalPercents := map[string]float64{"A_ACHIEVED": 10}

	rows := buildAchievementRows(player, schemas, schemaMap, globalPercents, 42)

	require.Len(t, rows, 2)

	achieved := rows[0]
	assert.Equal(t, "A_ACHIEVED", achieved.Name)
	assert.Equal(t, "Achieved", achieved.DisplayName)
	assert.Equal(t, 42, achieved.GameID)
	assert.True(t, achieved.Achieved)
	require.NotNil(t, achieved.UnlockTime)
	assert.Equal(t, unlockTime, achieved.UnlockTime.Unix())
	require.NotNil(t, achieved.GlobalPercent)
	assert.InEpsilon(t, 10.0, *achieved.GlobalPercent, 0.0001)

	notAchieved := rows[1]
	assert.False(t, notAchieved.Achieved)
	assert.Nil(t, notAchieved.UnlockTime)
	assert.Nil(t, notAchieved.GlobalPercent)
}

// TestBuildAchievementRows_FallsBackToSchema: with no player state, the
// schema defines the (unachieved) rows.
func TestBuildAchievementRows_FallsBackToSchema(t *testing.T) {
	//nolint:exhaustruct //only the fields buildAchievementRows reads
	schemas := []steam.AchievementSchema{
		{Name: "A", DisplayName: "A name", Description: "d", Icon: "i"},
	}

	rows := buildAchievementRows(nil, schemas, map[string]steam.AchievementSchema{
		"A": schemas[0],
	}, map[string]float64{}, 7)

	require.Len(t, rows, 1)
	assert.Equal(t, "A", rows[0].Name)
	assert.Equal(t, 7, rows[0].GameID)
	assert.False(t, rows[0].Achieved)
	assert.Nil(t, rows[0].UnlockTime)
}

func TestFetchGlobalPercents(t *testing.T) {
	//nolint:exhaustruct //only the field this method reads
	service := &SteamService{logger: discardLogger()}

	t.Run("success, filters unparsable percentages", func(t *testing.T) {
		//nolint:exhaustruct //only the field this test exercises
		client := stubSteamClient{
			globalPercents: func(
				context.Context,
				int,
			) (*steam.GlobalAchievementPercentagesResponse, error) {
				//nolint:exhaustruct //anonymous inner struct
				resp := &steam.GlobalAchievementPercentagesResponse{}
				resp.AchievementPercentages.Achievements = []steam.GlobalAchievementPercent{
					{Name: "GOOD", Percent: "12.5"},
					{Name: "BAD", Percent: "not-a-number"},
				}
				return resp, nil
			},
		}

		got := service.fetchGlobalPercents(context.Background(), client, 1)

		require.Contains(t, got, "GOOD")
		assert.InEpsilon(t, 12.5, got["GOOD"], 0.0001)
		assert.NotContains(t, got, "BAD")
	})

	t.Run("error returns empty map", func(t *testing.T) {
		//nolint:exhaustruct //only the field this test exercises
		client := stubSteamClient{
			globalPercents: func(
				context.Context,
				int,
			) (*steam.GlobalAchievementPercentagesResponse, error) {
				return nil, errors.New("boom")
			},
		}

		got := service.fetchGlobalPercents(context.Background(), client, 1)

		assert.Empty(t, got)
	})
}

func TestFetchAchievementsForGame(t *testing.T) {
	//nolint:exhaustruct //only the field this method reads
	service := &SteamService{logger: discardLogger()}

	t.Run("player achievements error", func(t *testing.T) {
		wantErr := errors.New("player achievements failed")
		//nolint:exhaustruct //only the field this test exercises
		client := stubSteamClient{
			playerAchievements: func(
				context.Context,
				string,
				int,
			) (*steam.AchievementsResponse, error) {
				return nil, wantErr
			},
		}

		_, err := service.fetchAchievementsForGame(
			context.Background(),
			client,
			"steam-id",
			1,
		)
		assert.ErrorIs(t, err, wantErr)
	})

	t.Run("schema error", func(t *testing.T) {
		wantErr := errors.New("schema failed")
		//nolint:exhaustruct //only the fields this test exercises
		client := stubSteamClient{
			playerAchievements: func(
				context.Context,
				string,
				int,
			) (*steam.AchievementsResponse, error) {
				//nolint:exhaustruct //zero value is fine
				return &steam.AchievementsResponse{}, nil
			},
			schemaForGame: func(
				context.Context,
				int,
			) (*steam.GetSchemaForGameResponse, error) {
				return nil, wantErr
			},
		}

		_, err := service.fetchAchievementsForGame(
			context.Background(),
			client,
			"steam-id",
			1,
		)
		assert.ErrorIs(t, err, wantErr)
	})

	t.Run("success", func(t *testing.T) {
		//nolint:exhaustruct //only the fields this test exercises
		client := stubSteamClient{
			playerAchievements: func(
				context.Context,
				string,
				int,
			) (*steam.AchievementsResponse, error) {
				//nolint:exhaustruct //zero values are fine for unused fields
				return &steam.AchievementsResponse{
					PlayerStats: steam.PlayerStats{
						Achievements: []steam.Achievement{
							{APIName: "A", Achieved: 1, UnlockTime: 1700000000},
						},
					},
				}, nil
			},
			schemaForGame: func(
				context.Context,
				int,
			) (*steam.GetSchemaForGameResponse, error) {
				//nolint:exhaustruct //zero values are fine for unused fields
				return &steam.GetSchemaForGameResponse{
					Game: steam.GameSchema{
						AvailableGameStats: steam.AvailableGameStats{
							Achievements: []steam.AchievementSchema{
								{Name: "A", DisplayName: "A name"},
							},
						},
					},
				}, nil
			},
			globalPercents: func(
				context.Context,
				int,
			) (*steam.GlobalAchievementPercentagesResponse, error) {
				return nil, errors.New("global percents unavailable")
			},
		}

		rows, err := service.fetchAchievementsForGame(
			context.Background(),
			client,
			"steam-id",
			9,
		)

		require.NoError(t, err)
		require.Len(t, rows, 1)
		assert.Equal(t, "A", rows[0].Name)
		assert.Equal(t, "A name", rows[0].DisplayName)
		assert.Equal(t, 9, rows[0].GameID)
		assert.True(t, rows[0].Achieved)
	})
}

// TestFetchAchievements: a failed game is omitted, not fatal.
func TestFetchAchievements(t *testing.T) {
	//nolint:exhaustruct //only the field this method reads
	service := &SteamService{logger: discardLogger()}

	const okGame, failGame = 1, 2

	//nolint:exhaustruct //only the fields this test exercises
	client := stubSteamClient{
		playerAchievements: func(
			_ context.Context,
			_ string,
			appID int,
		) (*steam.AchievementsResponse, error) {
			if appID == failGame {
				return nil, errors.New("boom")
			}
			//nolint:exhaustruct //zero values are fine for unused fields
			return &steam.AchievementsResponse{
				PlayerStats: steam.PlayerStats{
					Achievements: []steam.Achievement{{APIName: "A", Achieved: 1}},
				},
			}, nil
		},
		schemaForGame: func(
			context.Context,
			int,
		) (*steam.GetSchemaForGameResponse, error) {
			//nolint:exhaustruct //zero value is fine
			return &steam.GetSchemaForGameResponse{}, nil
		},
		globalPercents: func(
			context.Context,
			int,
		) (*steam.GlobalAchievementPercentagesResponse, error) {
			//nolint:exhaustruct //zero value is fine
			return &steam.GlobalAchievementPercentagesResponse{}, nil
		},
	}

	gamesMap := map[int]*models.Game{
		okGame:   gameWith(okGame, false),
		failGame: gameWith(failGame, false),
	}

	got := service.fetchAchievements(context.Background(), client, "steam-id", gamesMap)

	assert.Len(t, got, 1, "the failing game must be omitted, not zero-valued")
	assert.Contains(t, got, okGame)
	assert.NotContains(t, got, failGame)
}

// TestBuildProgress: only Achieved rows with an UnlockTime become points.
func TestBuildProgress(t *testing.T) {
	now := time.Now().UTC()
	//nolint:exhaustruct //only the fields the point filter reads
	fetched := map[int][]models.Achievement{
		1: {
			{Name: "counts", Achieved: true, UnlockTime: &now},
			{Name: "not achieved", Achieved: false, UnlockTime: &now},
			{Name: "no unlock time", Achieved: true, UnlockTime: nil},
		},
	}

	labels, values := buildProgress(fetched)

	require.Len(t, labels, 1, "every point falls on today, the grapher's seeded date")
	require.Len(t, values, 1)
	assert.Equal(t, "33.33", values[0], "1 of 3 total achievements for the game")
}
