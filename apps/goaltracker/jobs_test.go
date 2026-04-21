package goaltracker_test

import (
	"context"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/xdoubleu/essentia/v3/pkg/logging"
	"tools.xdoubleu.com/apps/goaltracker/internal/dtos"
	"tools.xdoubleu.com/apps/goaltracker/internal/jobs"
	"tools.xdoubleu.com/apps/goaltracker/internal/models"
	internalmodels "tools.xdoubleu.com/internal/models"
)

type noUsersAuthService struct{}

func (m *noUsersAuthService) Access(next http.HandlerFunc) http.HandlerFunc {
	return next
}

func (m *noUsersAuthService) TemplateAccess(next http.HandlerFunc) http.HandlerFunc {
	return next
}

func (m *noUsersAuthService) GetAllUsers() ([]internalmodels.User, error) {
	return []internalmodels.User{}, nil
}

func (m *noUsersAuthService) SignOut(
	_ string,
	_ bool,
) (*http.Cookie, *http.Cookie, error) {
	return nil, nil, nil
}

func TestGoodreadsJobNoUsers(t *testing.T) {
	job := jobs.NewGoodreadsJob(
		&noUsersAuthService{},
		testApp.Services.Goodreads,
		testApp.Services.Goals,
	)
	err := job.Run(context.Background(), logging.NewNopLogger())
	assert.Nil(t, err)
}

func TestSteamJobNoUsers(t *testing.T) {
	job := jobs.NewSteamJob(
		&noUsersAuthService{},
		testApp.Services.Steam,
		testApp.Services.Goals,
	)
	err := job.Run(context.Background(), logging.NewNopLogger())
	assert.Nil(t, err)
}

func TestTodoistJobNoUsers(t *testing.T) {
	job := jobs.NewTodoistJob(&noUsersAuthService{}, testApp.Services.Goals)
	err := job.Run(context.Background(), logging.NewNopLogger())
	assert.Nil(t, err)
}

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
	assert.NotNil(t, job.ID())
	assert.NotNil(t, job.RunEvery())

	err = job.Run(context.Background(), logging.NewNopLogger())
	assert.Nil(t, err)
}

func TestSteamJob(t *testing.T) {
	job := jobs.NewSteamJob(
		testApp.Services.Auth,
		testApp.Services.Steam,
		testApp.Services.Goals,
	)
	assert.NotNil(t, job.ID())
	assert.NotNil(t, job.RunEvery())

	err := job.Run(context.Background(), logging.NewNopLogger())
	assert.Nil(t, err)
}

func TestTodoistJob(t *testing.T) {
	job := jobs.NewTodoistJob(testApp.Services.Auth, testApp.Services.Goals)
	assert.NotNil(t, job.ID())
	assert.NotNil(t, job.RunEvery())

	err := job.Run(context.Background(), logging.NewNopLogger())
	assert.Nil(t, err)
}
