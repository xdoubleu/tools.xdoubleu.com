package games

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"tools.xdoubleu.com/apps/games/internal/models"
	"tools.xdoubleu.com/internal/consumption"
)

// game builds a played game with only what gameSignal reads.
func game(playtime int, rate string, favourite bool) models.Game {
	//nolint:exhaustruct // only what gameSignal reads
	return models.Game{Playtime: playtime, CompletionRate: rate, Favourite: favourite}
}

func TestGameSignal(t *testing.T) {
	const top = 600
	tests := []struct {
		name   string
		game   models.Game
		signal consumption.Signal
		reason string
	}{
		{"favourite", game(10, "0.00", true), consumption.StrongPositive, "favourite"},
		{
			"complete",
			game(10, "100.00", false),
			consumption.StrongPositive,
			"every achievement",
		},
		{
			"almost complete",
			game(top, "99.99", false),
			consumption.Positive,
			"among your longest played",
		},
		{
			"bounced",
			game(bouncedMinutes-1, "0.00", false),
			consumption.Negative,
			"under an hour played",
		},
		{"an hour", game(bouncedMinutes, "0.00", false), consumption.Neutral, "played"},
		{"just under the top", game(top-1, "0.00", false), consumption.Neutral, "played"},
		{
			"top quartile",
			game(top, "0.00", false),
			consumption.Positive,
			"among your longest played",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			signal, reason := gameSignal(tt.game, top)
			assert.Equal(t, tt.signal, signal)
			assert.Equal(t, tt.reason, reason)
		})
	}
}

func TestUpperQuartile(t *testing.T) {
	assert.Equal(t, 0, upperQuartile(nil))
	//nolint:exhaustruct // playtime only
	games := []models.Game{{Playtime: 40}, {Playtime: 10}, {Playtime: 30}, {Playtime: 20}}
	assert.Equal(t, 40, upperQuartile(games))
}
