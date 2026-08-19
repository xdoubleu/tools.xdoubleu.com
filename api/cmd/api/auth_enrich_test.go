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

// TestResolveToken_EnrichmentFailureNotCached covers issue #673: a transient
// DB failure while enriching a user (e.g. right after a deploy, when the
// per-token cache is cold for every session at once) must not be cached as
// "this user has no admin role / no app access" for the remainder of the TTL
// — it must fail this one resolution and let the next request retry cleanly.
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

	// A canceled context makes the enrichment DB queries (Upsert/GetByID)
	// fail, simulating a transient DB blip during resolution.
	canceledCtx, cancel := context.WithCancel(ctx)
	cancel()

	_, err := svc.ResolveToken(canceledCtx, accessToken.Value)
	require.Error(t, err)

	// The failed enrichment must not have been cached: a follow-up call with
	// a healthy context resolves the user's real (admin) role, not a stuck
	// "no access" result from the failed attempt above.
	user, err := svc.ResolveToken(ctx, accessToken.Value)
	require.NoError(t, err)
	assert.Equal(t, models.RoleAdmin, user.Role)
}

// TestTemplateAccess_RefreshEnrichmentFailure covers the refreshTokens side
// of the same issue #673 fix: when the access-token cookie is absent and the
// refresh-token fallback's enrichment DB call fails (a canceled context
// stands in for a transient DB blip), TemplateAccess must treat the request
// as unauthenticated rather than let a downgraded user through.
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

// TestResolveToken_NoAppUsersRepo covers enrichUser's "not configured" branch
// (appUsersRepo == nil): unlike a DB failure, this is a deliberate degrade —
// the raw GoTrue user is returned unchanged, with no error.
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

// TestResolveToken_GetByIDFailure covers enrichUser's GetByID-specific error
// branch: Upsert succeeds but the enrichment read fails. A shared canceled
// context can't isolate this from an Upsert failure (see the tests above),
// so this uses a fake store with independently configurable errors instead.
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

// TestFakeAppUsersStore_SuccessPaths exercises FakeAppUsersStore's success
// paths (GetByID returning the configured user, GetAll) that the failure
// tests above don't reach, so the fake itself isn't dragging down coverage.
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
