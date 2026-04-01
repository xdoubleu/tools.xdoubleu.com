package models

import (
	"fmt"
	"math"
	"time"
)

type Game struct {
	ID             int    `json:"id"`
	Name           string `json:"name"`
	IsDelisted     bool   `json:"isDelisted"`
	CompletionRate string `json:"completionRate"`
	Contribution   string `json:"contribution"`
}

type Achievement struct {
	Name       string     `json:"name"`
	GameID     int        `json:"gameId"`
	Achieved   bool       `json:"achieved"`
	UnlockTime *time.Time `json:"unlockTime"`
}

func (game *Game) SetCalculatedInfo(achievements []Achievement, totalGames int) {
	achieved := 0
	total := 0

	for _, achievement := range achievements {
		total++

		if achievement.Achieved {
			achieved++
		}
	}

	if total == 0 {
		game.CompletionRate = "0.00"
		game.Contribution = "0.0000"
		return
	}

	//nolint:mnd // this is a percentage
	game.CompletionRate = fmt.Sprintf(
		"%.2f",
		math.Floor(float64(achieved)/float64(total)*10000)/100,
	)
	//nolint:mnd // this is a percentage
	game.Contribution = fmt.Sprintf("%.4f", 100.0/float64(totalGames*total))
}
