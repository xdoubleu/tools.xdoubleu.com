package backlog_test

import (
	"context"
	"net/http"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/xdoubleu/essentia/v3/pkg/logging"
	"tools.xdoubleu.com/apps/backlog"
	"tools.xdoubleu.com/apps/backlog/internal/jobs"
	"tools.xdoubleu.com/apps/backlog/internal/mocks"
	backlogmodels "tools.xdoubleu.com/apps/backlog/internal/models"
	"tools.xdoubleu.com/apps/backlog/pkg/goodreads"
	"tools.xdoubleu.com/apps/backlog/pkg/steam"
	sharedmocks "tools.xdoubleu.com/internal/mocks"
	internalmodels "tools.xdoubleu.com/internal/models"
	"tools.xdoubleu.com/internal/templates"
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

// failFirstGoodreadsClient fails GetUserID on the first call, succeeds after.
type failFirstGoodreadsClient struct {
	mu        sync.Mutex
	callCount int
	inner     goodreads.Client
}

func (c *failFirstGoodreadsClient) GetUserID(url string) (*string, error) {
	c.mu.Lock()
	c.callCount++
	n := c.callCount
	c.mu.Unlock()
	if n == 1 {
		return nil, errSimulated
	}
	return c.inner.GetUserID(url)
}

func (c *failFirstGoodreadsClient) GetBooks(
	ctx context.Context,
	uid string,
) ([]goodreads.Book, error) {
	return c.inner.GetBooks(ctx, uid)
}

var errSimulated = assert.AnError

func TestGoodreadsJobNoUsers(t *testing.T) {
	job := jobs.NewGoodreadsJob(
		&noUsersAuthService{},
		testApp.Services.Goodreads,
		testApp.Services.Progress,
	)
	err := job.Run(context.Background(), logging.NewNopLogger())
	assert.Nil(t, err)
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

func TestGoodreadsJob(t *testing.T) {
	job := jobs.NewGoodreadsJob(
		testApp.Services.Auth,
		testApp.Services.Goodreads,
		testApp.Services.Progress,
	)
	assert.NotNil(t, job.ID())
	assert.NotNil(t, job.RunEvery())

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

// TestGoodreadsJobContinuesAfterUserError verifies that a failure for one user
// does not prevent subsequent users from being processed.
func TestGoodreadsJobContinuesAfterUserError(t *testing.T) {
	const user2ID = "bbbbbbbb-0000-0000-0000-000000000001"

	// Give both users a Goodreads URL so the client is invoked for each.
	require.NoError(t, testApp.SaveIntegrations(
		context.Background(),
		userID,
		backlog.Integrations{
			SteamAPIKey:  "",
			SteamUserID:  "",
			GoodreadsURL: "test-url",
		},
	))
	require.NoError(t, testApp.SaveIntegrations(
		context.Background(),
		user2ID,
		backlog.Integrations{
			SteamAPIKey:  "",
			SteamUserID:  "",
			GoodreadsURL: "test-url",
		},
	))
	t.Cleanup(func() {
		//nolint:exhaustruct //intentionally empty to restore state
		_ = testApp.SaveIntegrations(
			context.Background(),
			userID,
			backlog.Integrations{},
		)
		//nolint:exhaustruct //intentionally empty to clean up
		_ = testApp.SaveIntegrations(
			context.Background(),
			user2ID,
			backlog.Integrations{},
		)
	})

	twoAuth := &twoUsersAuthService{userID: userID, userID2: user2ID}
	failClient := &failFirstGoodreadsClient{
		mu:        sync.Mutex{},
		callCount: 0,
		inner:     mocks.NewMockGoodreadsClient(),
	}

	app2 := backlog.NewInner(
		context.Background(),
		sharedmocks.NewMockedAuthService(userID),
		logging.NewNopLogger(),
		testCfg,
		testDB,
		backlog.Clients{
			SteamFactory: func(_ string) steam.Client { return mocks.NewMockSteamClient() },
			Goodreads:    failClient,
		},
		templates.LoadShared(testCfg),
	)

	job := jobs.NewGoodreadsJob(
		twoAuth,
		app2.Services.Goodreads,
		app2.Services.Progress,
	)
	err := job.Run(context.Background(), logging.NewNopLogger())

	// Job must report the error from user1.
	require.Error(t, err)

	// user2 must still have been processed (progress saved).
	labels, _, err2 := app2.Services.Progress.GetByTypeIDAndDates(
		context.Background(),
		backlogmodels.GoodreadsTypeID,
		user2ID,
		time.Now().AddDate(-2, 0, 0),
		time.Now().AddDate(1, 0, 0),
	)
	require.NoError(t, err2)
	assert.NotEmpty(t, labels, "user2 should be processed even though user1 failed")
}
