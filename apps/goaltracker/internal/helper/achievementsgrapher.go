package helper

import (
	"fmt"
	"math"
	"slices"
	"time"

	"tools.xdoubleu.com/apps/goaltracker/internal/models"
)

type AchievementsGrapher struct {
	dateStrings               []string
	achievementsPerGamePerDay []map[int]int
	totalAchievementsPerGame  map[int]int
}

func NewAchievementsGrapher(totalAchievementsPerGame map[int]int) AchievementsGrapher {
	grapher := AchievementsGrapher{
		dateStrings:               []string{},
		achievementsPerGamePerDay: []map[int]int{},
		totalAchievementsPerGame:  totalAchievementsPerGame,
	}

	// need this so that the value at
	// the current date is always shown, even if nothing changed
	grapher.dateStrings = append(
		grapher.dateStrings,
		time.Now().UTC().Format(models.ProgressDateFormat),
	)
	grapher.achievementsPerGamePerDay = append(
		grapher.achievementsPerGamePerDay,
		make(map[int]int),
	)

	return grapher
}

func (grapher *AchievementsGrapher) AddPoint(date time.Time, gameID int) {
	dateStr := date.Format(models.ProgressDateFormat)
	dateIndex := slices.Index(grapher.dateStrings, dateStr)

	if dateIndex == -1 {
		grapher.addDays(dateStr)
		dateIndex = slices.Index(grapher.dateStrings, dateStr)
	}

	grapher.updateDays(dateIndex, gameID)
}

func (grapher *AchievementsGrapher) addDays(dateStr string) {
	dateDay, _ := time.Parse(models.ProgressDateFormat, dateStr)
	smallestDate, _ := time.Parse(models.ProgressDateFormat, grapher.dateStrings[0])
	largestDate, _ := time.Parse(
		models.ProgressDateFormat,
		grapher.dateStrings[len(grapher.dateStrings)-1],
	)

	i := smallestDate
	for i.After(dateDay) {
		i = i.AddDate(0, 0, -1)

		grapher.dateStrings = append(
			[]string{i.Format(models.ProgressDateFormat)},
			grapher.dateStrings...)
		grapher.achievementsPerGamePerDay = append(
			[]map[int]int{{}},
			grapher.achievementsPerGamePerDay...)
	}

	i = largestDate
	for i.Before(dateDay) {
		i = i.AddDate(0, 0, 1)

		grapher.dateStrings = append(
			grapher.dateStrings,
			i.Format(models.ProgressDateFormat),
		)

		indexOfI := slices.Index(
			grapher.dateStrings,
			i.Format(models.ProgressDateFormat),
		)

		grapher.achievementsPerGamePerDay = append(
			grapher.achievementsPerGamePerDay,
			copyMap(
				grapher.achievementsPerGamePerDay[indexOfI-1],
			),
		)
	}
}

func copyMap(original map[int]int) map[int]int {
	target := map[int]int{}

	for k, v := range original {
		target[k] = v
	}

	return target
}

func (grapher *AchievementsGrapher) updateDays(dateIndex int, gameID int) {
	for i := dateIndex; i < len(grapher.dateStrings); i++ {
		if _, ok := grapher.achievementsPerGamePerDay[i][gameID]; !ok {
			grapher.achievementsPerGamePerDay[i][gameID] = 0
		}

		grapher.achievementsPerGamePerDay[i][gameID]++
	}
}

func (grapher AchievementsGrapher) ToSlices() ([]string, []string) {
	percentages := []string{}

	for _, achievementsPerGame := range grapher.achievementsPerGamePerDay {
		games := 0
		totalPercentageDay := 0.0

		for gameID, achievements := range achievementsPerGame {
			games++

			totalAchievements := grapher.totalAchievementsPerGame[gameID]

			completionRate := calculateCompletionRate(
				achievements,
				totalAchievements,
			)

			if !math.IsNaN(completionRate) {
				totalPercentageDay += completionRate
			}
		}

		avgCompletionRate := calculateAvgCompletionRate(
			totalPercentageDay,
			games,
		)

		percentages = append(percentages, avgCompletionRate)
	}

	return grapher.dateStrings, percentages
}

func calculateCompletionRate(achieved int, total int) float64 {
	return float64(achieved) / float64(total)
}

func calculateAvgCompletionRate(percentageSum float64, totalGames int) string {
	//nolint:mnd //no magic number
	return fmt.Sprintf("%.2f", math.Floor(percentageSum/float64(totalGames)*10000)/100)
}
