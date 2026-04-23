package backlog_test

import (
	"context"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/xdoubleu/essentia/v3/pkg/logging"
	"tools.xdoubleu.com/apps/backlog/internal/jobs"
	internalmodels "tools.xdoubleu.com/internal/models"
)

type noUsersAuthService struct{}

func (m *noUsersAuthService) Access(next http.HandlerFunc) http.HandlerFunc {
	return next
}

func (m *noUsersAuthService) TemplateAccess(next http.HandlerFunc) http.HandlerFunc {
	return next
}

func (m *noUsersAuthService) AdminAccess(next http.HandlerFunc) http.HandlerFunc {
	return next
}

func (m *noUsersAuthService) AppAccess(
	_ string,
	next http.HandlerFunc,
) http.HandlerFunc {
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

type twoUsersAuthService struct {
	userID  string
	userID2 string
}

func (m *twoUsersAuthService) Access(
	next http.HandlerFunc,
) http.HandlerFunc {
	return next
}

func (m *twoUsersAuthService) TemplateAccess(
	next http.HandlerFunc,
) http.HandlerFunc {
	return next
}

func (m *twoUsersAuthService) AdminAccess(
	next http.HandlerFunc,
) http.HandlerFunc {
	return next
}

func (m *twoUsersAuthService) AppAccess(
	_ string,
	next http.HandlerFunc,
) http.HandlerFunc {
	return next
}

func (m *twoUsersAuthService) SignOut(
	_ string,
	_ bool,
) (*http.Cookie, *http.Cookie, error) {
	return nil, nil, nil
}
func (m *twoUsersAuthService) GetAllUsers() ([]internalmodels.User, error) {
	//nolint:exhaustruct //test stub
	return []internalmodels.User{
		{ID: m.userID},
		{ID: m.userID2},
	}, nil
}

func TestSteamJobNoUsers(t *testing.T) {
	job := jobs.NewSteamJob(
		&noUsersAuthService{},
		testApp.Services.Steam,
		testApp.Services.Progress,
	)
	err := job.Run(context.Background(), logging.NewNopLogger())
	assert.Nil(t, err)
}

func TestSteamJob(t *testing.T) {
	job := jobs.NewSteamJob(
		testApp.Services.Auth,
		testApp.Services.Steam,
		testApp.Services.Progress,
	)
	assert.NotNil(t, job.ID())
	assert.NotNil(t, job.RunEvery())

	err := job.Run(context.Background(), logging.NewNopLogger())
	assert.Nil(t, err)
}
