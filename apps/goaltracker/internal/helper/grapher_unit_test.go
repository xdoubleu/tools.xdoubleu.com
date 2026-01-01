package helper_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"tools.xdoubleu.com/apps/goaltracker/internal/helper"
)

func TestAchievementsGrapher(t *testing.T) {
	totalAchievementsPerGame := map[int]int{
		1: 10, // no achievements achieved
		2: 20, // 10 achievements achieved
		3: 30, // 20 achievements achieved
	}

	grapher := helper.NewAchievementsGrapher(totalAchievementsPerGame)

	dateNow := time.Now().UTC()
	for i := 0; i < 10; i++ {
		grapher.AddPoint(dateNow.AddDate(0, 0, i), 2)
	}

	for i := 0; i < 20; i++ {
		grapher.AddPoint(dateNow.AddDate(0, 0, -1*i), 3)
	}

	dateSlice, valueSlice := grapher.ToSlices()

	assert.Equal(t, 29, len(dateSlice))
	assert.Equal(t, 29, len(valueSlice))

	assert.Equal(t, "58.33", valueSlice[28])
}
