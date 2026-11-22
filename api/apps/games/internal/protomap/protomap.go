// Package protomap maps the games domain models to their proto messages, for
// the Connect and MCP adapters.
package protomap

import (
	"time"

	"tools.xdoubleu.com/apps/games/internal/models"
	gamesv1 "tools.xdoubleu.com/gen/games/v1"
)

// Game maps a game.
func Game(g models.Game) *gamesv1.Game {
	return &gamesv1.Game{
		Id:                  int32(g.ID), //nolint:gosec // int32 safe for domain values
		Name:                g.Name,
		IsDelisted:          g.IsDelisted,
		InCompletionAverage: g.InCompletionAverage,
		CompletionRate:      g.CompletionRate,
		Contribution:        g.Contribution,
		Playtime:            int32(g.Playtime), //nolint:gosec // safe for domain values
		ImageUrl:            g.ImageURL,
		LastSyncedAt:        g.LastSyncedAt.Format(time.RFC3339),
		Favourite:           g.Favourite,
	}
}

// Games maps games.
func Games(games []models.Game) []*gamesv1.Game {
	result := make([]*gamesv1.Game, len(games))
	for i, g := range games {
		result[i] = Game(g)
	}
	return result
}

// RecentGames maps recently active games.
func RecentGames(games []models.RecentGame) []*gamesv1.RecentGame {
	result := make([]*gamesv1.RecentGame, len(games))
	for i, g := range games {
		result[i] = &gamesv1.RecentGame{
			Id:             int32(g.ID), //nolint:gosec // int32 safe for domain values
			Name:           g.Name,
			CompletionRate: g.CompletionRate,
			ImageUrl:       g.ImageURL,
			LastPlayedAt:   g.LastPlayed.Format(models.ProgressDateFormat),
			Playtime:       int32(g.Playtime), //nolint:gosec // safe for domain values
		}
	}
	return result
}

// Achievements maps achievements.
func Achievements(achievements []models.Achievement) []*gamesv1.Achievement {
	result := make([]*gamesv1.Achievement, len(achievements))
	for i, a := range achievements {
		result[i] = &gamesv1.Achievement{
			Name:          a.Name,
			DisplayName:   a.DisplayName,
			Description:   a.Description,
			IconUrl:       a.IconURL,
			Achieved:      a.Achieved,
			GlobalPercent: a.GlobalPercent,
		}
	}
	return result
}

func intSlice(ints []int) []int32 {
	result := make([]int32, len(ints))
	for i, v := range ints {
		result[i] = int32(v) //nolint:gosec // int32 safe for domain values
	}
	return result
}

// Wishlist maps wishlist entries.
func Wishlist(items []models.WishlistItem) []*gamesv1.WishlistGame {
	games := make([]*gamesv1.WishlistGame, len(items))
	for i, it := range items {
		games[i] = &gamesv1.WishlistGame{
			Id:       int32(it.ID), //nolint:gosec // int32 safe for domain values
			Name:     it.Name,
			ImageUrl: it.ImageURL,
			Priority: int32(it.Priority), //nolint:gosec // safe for domain values
			AddedAt:  "",
		}
		if it.AddedAt != nil {
			games[i].AddedAt = it.AddedAt.Format(time.RFC3339)
		}
	}
	return games
}

// Steam maps a user's library overview.
func Steam(o *models.SteamOverview) *gamesv1.SteamResponse {
	return &gamesv1.SteamResponse{
		NotStarted: Games(o.NotStarted),
		InProgress: Games(o.InProgress),
		Completed:  Games(o.Completed),
		Delisted:   Games(o.Delisted),
		//nolint:gosec // safe for domain counts
		TotalBacklog: int32(o.TotalBacklog),
		Distribution: intSlice(o.Distribution),
		CurrentRate:  o.CurrentRate,
		Labels:       o.Labels,
		Values:       o.Values,
		DateStart:    o.DateStart.Format(models.ProgressDateFormat),
		DateEnd:      o.DateEnd.Format(models.ProgressDateFormat),
	}
}

// SteamGame maps one game with its achievements.
func SteamGame(d *models.SteamGameDetail) *gamesv1.SteamGameResponse {
	return &gamesv1.SteamGameResponse{
		Game:         Game(d.Game),
		Achievements: Achievements(d.Achievements),
	}
}

// DateRange parses an optional window, defaulting to the last year.
func DateRange(dateStart, dateEnd string) (time.Time, time.Time) {
	end := time.Now()
	start := end.AddDate(-1, 0, 0)

	if dateStart != "" {
		if t, err := time.Parse(models.ProgressDateFormat, dateStart); err == nil {
			start = t
		}
	}
	if dateEnd != "" {
		if t, err := time.Parse(models.ProgressDateFormat, dateEnd); err == nil {
			end = t
		}
	}
	return start, end
}
