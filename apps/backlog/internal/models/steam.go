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
	Playtime       int    `json:"playtime"`
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

	game.CompletionRate = CalculateAvgCompletionRate(float64(achieved), total)

	const percentage = 100.0
	game.Contribution = fmt.Sprintf("%.4f", percentage/float64(totalGames*total))
}

func CalculateAvgCompletionRate(percentageSum float64, totalGames int) string {
	const percentagePrecision = 100
	const doublePercentagePrecision = 10000
	return fmt.Sprintf(
		"%.2f",
		math.Floor(
			percentageSum/float64(totalGames)*doublePercentagePrecision,
		)/percentagePrecision,
	)
}
