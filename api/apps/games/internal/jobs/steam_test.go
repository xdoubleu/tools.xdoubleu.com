package jobs_test

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"tools.xdoubleu.com/apps/games/internal/jobs"
	"tools.xdoubleu.com/internal/models"
)

type fakeAuthService struct {
	users []models.User
	err   error
}

func (f fakeAuthService) Access(next http.HandlerFunc) http.HandlerFunc { return next }

func (f fakeAuthService) TemplateAccess(
	next http.HandlerFunc,
) http.HandlerFunc {
	return next
}

func (f fakeAuthService) AdminAccess(
	next http.HandlerFunc,
) http.HandlerFunc {
	return next
}

func (f fakeAuthService) AppAccess(_ string, next http.HandlerFunc) http.HandlerFunc {
	return next
}

func (f fakeAuthService) GetAllUsers(_ context.Context) ([]models.User, error) {
	return f.users, f.err
}

func (f fakeAuthService) SignOut(
	_ context.Context,
	_ string,
	_ bool,
) (*http.Cookie, *http.Cookie, error) {
	return nil, nil, nil
}

type fakeSteamSyncer struct {
	mu       sync.Mutex
	failFor  map[string]bool
	attempts []string
}

func (f *fakeSteamSyncer) SyncUser(_ context.Context, userID string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.attempts = append(f.attempts, userID)
	if f.failFor[userID] {
		return errors.New("sync failed")
	}
	return nil
}

func TestSteamJobRunSucceedsWhenOneUserFails(t *testing.T) {
	auth := fakeAuthService{
		//nolint:exhaustruct //only ID matters for this test
		users: []models.User{{ID: "user-1"}, {ID: "user-2"}},
		err:   nil,
	}
	//nolint:exhaustruct //mu, attempts are zero-value sync.Mutex/nil slice
	syncer := &fakeSteamSyncer{failFor: map[string]bool{"user-1": true}}

	job := jobs.NewSteamJob(auth, syncer)
	err := job.Run(t.Context(), slog.Default())

	require.NoError(t, err)
	assert.ElementsMatch(t, []string{"user-1", "user-2"}, syncer.attempts)
}

func TestSteamJobRunFailsWhenGetAllUsersFails(t *testing.T) {
	wantErr := errors.New("boom")
	auth := fakeAuthService{users: nil, err: wantErr}
	//nolint:exhaustruct //mu, attempts are zero-value sync.Mutex/nil slice
	syncer := &fakeSteamSyncer{failFor: nil}

	job := jobs.NewSteamJob(auth, syncer)
	err := job.Run(t.Context(), slog.Default())

	require.ErrorIs(t, err, wantErr)
	assert.Empty(t, syncer.attempts)
}
