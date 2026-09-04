package services

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"tools.xdoubleu.com/apps/games/internal/models"
)

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

// TestMarkCompletionAverageMembership covers the cases the sync-level tests
// cannot reach cheaply, above all the two ways a game must *not* be read as
// superseded: a partial overlap, and a set spread across several listed games.
// Getting either wrong silently drops a game from the library-wide average
// (docs/adr-0018-completion-average-population.md). The last case pins the
// accepted false positive at the other end.
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
			// Accepted limitation: names are only unique per app, so a
			// delisted game whose whole set happens to sit inside an
			// unrelated listed game's does read as a takeover. Requiring
			// one whole set inside one game keeps that unlikely.
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

// TestAveragedAchievementsKeepsGamesBeingRefreshed pins the carve-out for a
// game absent from gamesMap: it is mid-refresh, not excluded.
func TestAveragedAchievementsKeepsGamesBeingRefreshed(t *testing.T) {
	gamesMap := map[int]*models.Game{1: gameWith(1, false)}
	gamesMap[1].InCompletionAverage = true

	out := averagedAchievements(map[int][]models.Achievement{
		1: rows("A"),
		9: rows("B"),
	}, gamesMap)

	assert.Len(t, out, 2, "a game not yet read back is kept")
}
