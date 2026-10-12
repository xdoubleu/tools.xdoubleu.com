package games

import (
	"context"
	"slices"
	"strconv"
	"time"

	"tools.xdoubleu.com/apps/games/internal/models"
	"tools.xdoubleu.com/internal/consumption"
)

// bouncedMinutes is the playtime under which a game counts as bounced off.
const bouncedMinutes = 60

// fullCompletion is models.Game.CompletionRate at every achievement.
const fullCompletion = 100.0

// ListConsumption returns userID's played Steam games as consumption events,
// for the taste app. Playtime is judged against the user's own played games,
// so a top-quartile game is one they spent unusually long on.
func (a *Games) ListConsumption(
	ctx context.Context,
	userID string,
) ([]consumption.Event, error) {
	games, err := a.Services.Steam.GetAllGames(ctx, userID)
	if err != nil {
		return nil, err
	}

	played := slices.DeleteFunc(games, func(g models.Game) bool {
		return g.Playtime <= 0
	})
	topQuartile := upperQuartile(played)

	events := make([]consumption.Event, 0, len(played))
	for _, g := range played {
		signal, reason := gameSignal(g, topQuartile)
		var at time.Time
		if g.LastPlayed != nil {
			at = *g.LastPlayed
		}
		events = append(events, consumption.Event{
			Domain:  consumption.DomainGames,
			ItemID:  strconv.Itoa(g.ID),
			Title:   g.Name,
			Creator: "",
			Tags:    nil,
			At:      at,
			Signal:  signal,
			Reason:  reason,
		})
	}
	return events, nil
}

func gameSignal(g models.Game, topQuartile int) (consumption.Signal, string) {
	rate, _ := strconv.ParseFloat(g.CompletionRate, 64)
	switch {
	case g.Favourite:
		return consumption.StrongPositive, "favourite"
	case rate >= fullCompletion:
		return consumption.StrongPositive, "every achievement"
	case g.Playtime < bouncedMinutes:
		return consumption.Negative, "under an hour played"
	case g.Playtime >= topQuartile:
		return consumption.Positive, "among your longest played"
	default:
		return consumption.Neutral, "played"
	}
}

// upperQuartile returns the 75th-percentile playtime of games.
func upperQuartile(games []models.Game) int {
	if len(games) == 0 {
		return 0
	}
	playtimes := make([]int, len(games))
	for i, g := range games {
		playtimes[i] = g.Playtime
	}
	slices.Sort(playtimes)
	const quartile = 3
	return playtimes[len(playtimes)*quartile/4]
}
