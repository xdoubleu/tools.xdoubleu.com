package goaltracker_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/xdoubleu/essentia/v2/pkg/logging"
	"tools.xdoubleu.com/apps/goaltracker/internal/dtos"
	"tools.xdoubleu.com/apps/goaltracker/internal/jobs"
	"tools.xdoubleu.com/apps/goaltracker/internal/models"
)

func TestGoodreadsJob(t *testing.T) {
	err := testApp.Services.Goals.ImportGoalsFromTodoist(context.Background(), userID)
	assert.Nil(t, err)

	val := int64(12)
	val1 := "tag1"
	err = testApp.Services.Goals.LinkGoal(
		context.Background(),
		goalID,
		userID,
		&dtos.LinkGoalDto{
			TypeID:      models.BooksFromSpecificTag.ID,
			TargetValue: &val,
			Tag:         &val1,
		},
	)
	assert.Nil(t, err)

	job := jobs.NewGoodreadsJob(
		testApp.Services.Auth,
		testApp.Services.Goodreads,
		testApp.Services.Goals,
	)
	job.ID()
	job.RunEvery()

	err = job.Run(context.Background(), logging.NewNopLogger())
	assert.Nil(t, err)
}

func TestSteamJob(t *testing.T) {
	job := jobs.NewSteamJob(
		testApp.Services.Auth,
		testApp.Services.Steam,
		testApp.Services.Goals,
	)
	job.ID()
	job.RunEvery()

	err := job.Run(context.Background(), logging.NewNopLogger())
	assert.Nil(t, err)
}

func TestTodoistJob(t *testing.T) {
	job := jobs.NewTodoistJob(testApp.Services.Auth, testApp.Services.Goals)
	job.ID()
	job.RunEvery()

	err := job.Run(context.Background(), logging.NewNopLogger())
	assert.Nil(t, err)
}
