package main

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"tools.xdoubleu.com/internal/auth"
	"tools.xdoubleu.com/internal/mailer"
	"tools.xdoubleu.com/internal/mocks"
	"tools.xdoubleu.com/internal/models"
	"tools.xdoubleu.com/internal/testhelper"
)

// TestResolveToken_EnrichmentFailureNotCached: a transient DB failure while
// enriching a user fails this resolution only; it's never cached as
// "no access".
func TestResolveToken_EnrichmentFailureNotCached(t *testing.T) {
	ctx := context.Background()

	cfg := testhelper.NewTestConfig()
	cfg.AuthCacheTTL = 60

	svc := auth.NewService(
		cfg, auth.NewRepository(testApp.db), testApp.appUsersRepo,
		nil, mailer.New("", "", ""),
	)

	require.NoError(
		t,
		testApp.appUsersRepo.Upsert(ctx, testUserID, "user@example.com"),
	)
	require.NoError(
		t,
		testApp.appUsersRepo.SetRole(ctx, testUserID, models.RoleAdmin),
	)
	t.Cleanup(func() {
		require.NoError(
			t,
			testApp.appUsersRepo.SetRole(ctx, testUserID, models.RoleUser),
		)
	})

	// A canceled context makes enrichment queries fail.
	canceledCtx, cancel := context.WithCancel(ctx)
	cancel()

	_, err := svc.ResolveToken(canceledCtx, accessToken.Value)
	require.Error(t, err)

	// The failure wasn't cached: a healthy call resolves the real admin role.
	user, err := svc.ResolveToken(ctx, accessToken.Value)
	require.NoError(t, err)
	assert.Equal(t, models.RoleAdmin, user.Role)
}

// TestTemplateAccess_RefreshEnrichmentFailure: a failed enrichment on the
// refresh-token path is treated as unauthenticated.
func TestTemplateAccess_RefreshEnrichmentFailure(t *testing.T) {
	require.NoError(
		t,
		testApp.appUsersRepo.Upsert(
			context.Background(),
			testUserID,
			"user@example.com",
		),
	)

	var called bool
	handler := testApp.auth.TemplateAccess(
		func(_ http.ResponseWriter, _ *http.Request) {
			called = true
		},
	)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	req := httptest.NewRequest(http.MethodGet, "/", nil).WithContext(ctx)
	req.AddCookie(&http.Cookie{Name: "refreshToken", Value: "refresh"})
	rr := httptest.NewRecorder()
	handler(rr, req)

	assert.False(t, called)
	assert.Equal(t, http.StatusUnauthorized, rr.Code)
}

// TestResolveToken_NoAppUsersRepo: with no repo, the user is returned
// unchanged without error.
func TestResolveToken_NoAppUsersRepo(t *testing.T) {
	svc := auth.NewService(
		testhelper.NewTestConfig(),
		auth.NewRepository(testApp.db),
		nil,
		nil,
		mailer.New("", "", ""),
	)

	user, err := svc.ResolveToken(context.Background(), accessToken.Value)
	require.NoError(t, err)
	assert.Equal(t, models.RoleUser, user.Role)
}

// TestResolveToken_GetByIDFailure uses a fake store to fail only GetByID.
func TestResolveToken_GetByIDFailure(t *testing.T) {
	//nolint:exhaustruct //only GetByIDErr matters for this test
	store := &mocks.FakeAppUsersStore{
		GetByIDErr: errors.New("db read failed"),
	}
	svc := auth.NewService(
		testhelper.NewTestConfig(),
		auth.NewRepository(testApp.db),
		store,
		nil,
		mailer.New("", "", ""),
	)

	_, err := svc.ResolveToken(context.Background(), accessToken.Value)
	require.Error(t, err)
}

// TestFakeAppUsersStore_SuccessPaths covers the fake's success paths.
func TestFakeAppUsersStore_SuccessPaths(t *testing.T) {
	//nolint:exhaustruct //UpsertErr/GetByIDErr default to nil (success)
	store := &mocks.FakeAppUsersStore{
		User: models.User{
			ID:        testUserID,
			Role:      models.RoleAdmin,
			AppAccess: []string{},
		},
	}
	svc := auth.NewService(
		testhelper.NewTestConfig(),
		auth.NewRepository(testApp.db),
		store,
		nil,
		mailer.New("", "", ""),
	)

	user, err := svc.ResolveToken(context.Background(), accessToken.Value)
	require.NoError(t, err)
	assert.Equal(t, models.RoleAdmin, user.Role)

	all, err := svc.GetAllUsers(context.Background())
	require.NoError(t, err)
	assert.Len(t, all, 1)
}
