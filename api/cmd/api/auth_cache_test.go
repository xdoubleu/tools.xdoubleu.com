package main

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"tools.xdoubleu.com/internal/auth"
	"tools.xdoubleu.com/internal/constants"
	"tools.xdoubleu.com/internal/mocks"
	"tools.xdoubleu.com/internal/models"
	"tools.xdoubleu.com/internal/testhelper"
)

// TestAuthCacheServesAndInvalidates exercises the per-token user cache end to
// end: a role change is invisible while the entry is cached and visible again
// after InvalidateUserCache.
func TestAuthCacheServesAndInvalidates(t *testing.T) {
	ctx := context.Background()

	cfg := testhelper.NewTestConfig()
	cfg.AuthCacheTTL = 60

	svc := auth.NewService(cfg, mocks.NewMockedGoTrueClient(), testApp.appUsersRepo)

	require.NoError(
		t,
		testApp.appUsersRepo.Upsert(ctx, testUserID, "user@example.com"),
	)
	original, err := testApp.appUsersRepo.GetByID(ctx, testUserID)
	require.NoError(t, err)
	defer func() {
		require.NoError(
			t,
			testApp.appUsersRepo.SetRole(ctx, testUserID, original.Role),
		)
	}()

	var seenRole models.Role
	handler := svc.Access(func(_ http.ResponseWriter, r *http.Request) {
		user, ok := r.Context().Value(constants.UserContextKey).(models.User)
		require.True(t, ok)
		seenRole = user.Role
	})

	doRequest := func() {
		r := httptest.NewRequest(http.MethodGet, "/", nil)
		r.AddCookie(&accessToken)
		w := httptest.NewRecorder()
		handler(w, r)
		require.Equal(t, http.StatusOK, w.Code)
	}

	require.NoError(
		t,
		testApp.appUsersRepo.SetRole(ctx, testUserID, models.RoleUser),
	)
	doRequest()
	assert.Equal(t, models.RoleUser, seenRole)

	// Role changes in the DB, but the cached entry keeps serving the old one.
	require.NoError(
		t,
		testApp.appUsersRepo.SetRole(ctx, testUserID, models.RoleAdmin),
	)
	doRequest()
	assert.Equal(t, models.RoleUser, seenRole)

	// Clearing the cache makes the new role visible immediately.
	svc.InvalidateUserCache()
	doRequest()
	assert.Equal(t, models.RoleAdmin, seenRole)
}
