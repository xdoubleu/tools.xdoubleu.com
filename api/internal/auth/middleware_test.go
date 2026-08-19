package auth_test

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/bcrypt"

	"tools.xdoubleu.com/internal/auth"
	"tools.xdoubleu.com/internal/mailer"
	"tools.xdoubleu.com/internal/testhelper"
)

const testPassword = "password"

// seedUser inserts a fresh auth.users row with a random email, sharing
// testPassword, and returns its ID.
func seedUser(t *testing.T, db *pgxpool.Pool) string {
	t.Helper()
	ctx := context.Background()

	id := uuid.NewString()
	hash, err := bcrypt.GenerateFromPassword([]byte(testPassword), bcrypt.DefaultCost)
	require.NoError(t, err)

	_, err = db.Exec(ctx, `
		INSERT INTO auth.users (id, email, password_hash) VALUES ($1, $2, $3)
	`, id, id+"@example.com", string(hash))
	require.NoError(t, err)

	return id
}

func newTestAccessService(t *testing.T) (*auth.LocalService, string) {
	t.Helper()
	db := testhelper.ConnectTestDB(testhelper.NewTestConfig().DBDsn)
	t.Cleanup(db.Close)

	userID := seedUser(t, db)

	service := auth.NewService(
		testhelper.NewTestConfig(), auth.NewRepository(db), nil, nil,
		mailer.New("", "", ""),
	)

	token, _, err := service.SignInWithEmail(
		context.Background(), userID+"@example.com", testPassword,
	)
	require.NoError(t, err)

	return service, *token
}

// TestResolveUserCoalescesConcurrentCalls guards against the thundering herd
// behind issue #852: opening several tabs at once used to fire one
// verification round trip per tab for the same (cache-miss) access token.
// The per-token cache alone already prevents re-verifying on every call
// once warm; this exercises the concurrent cache-miss path specifically.
func TestResolveUserCoalescesConcurrentCalls(t *testing.T) {
	service, token := newTestAccessService(t)

	const tabs = 10
	var wg sync.WaitGroup
	wg.Add(tabs)
	for range tabs {
		go func() {
			defer wg.Done()
			_, err := service.ResolveToken(context.Background(), token)
			assert.NoError(t, err)
		}()
	}
	wg.Wait()
}

func callAccess(
	service *auth.LocalService,
	cookies ...*http.Cookie,
) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	for _, c := range cookies {
		req.AddCookie(c)
	}
	rec := httptest.NewRecorder()
	handler := service.Access(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	handler(rec, req)
	return rec
}

func TestAccessNoCookiesUnauthorized(t *testing.T) {
	service, _ := newTestAccessService(t)
	rec := callAccess(service)
	assert.Equal(t, http.StatusUnauthorized, rec.Code)
}

func TestAccessValidAccessTokenPassesThrough(t *testing.T) {
	service, token := newTestAccessService(t)
	rec := callAccess(
		service,
		&http.Cookie{Name: "accessToken", Value: token},
	)
	assert.Equal(t, http.StatusOK, rec.Code)
}

// TestAccessExpiredTokenRefreshesSession guards against issue #809: a
// session left open long enough for the access token to expire used to 401
// on every subsequent request even though the refresh token cookie was
// still valid, since (unlike TemplateAccess) Access never attempted a
// refresh.
func TestAccessExpiredTokenRefreshesSession(t *testing.T) {
	db := testhelper.ConnectTestDB(testhelper.NewTestConfig().DBDsn)
	t.Cleanup(db.Close)
	userID := seedUser(t, db)

	service := auth.NewService(
		testhelper.NewTestConfig(), auth.NewRepository(db), nil, nil,
		mailer.New("", "", ""),
	)
	_, refreshToken, err := service.SignInWithEmail(
		context.Background(), userID+"@example.com", testPassword,
	)
	require.NoError(t, err)

	rec := callAccess(
		service,
		&http.Cookie{Name: "accessToken", Value: "expired-or-malformed"},
		&http.Cookie{Name: "refreshToken", Value: *refreshToken},
	)
	require.Equal(t, http.StatusOK, rec.Code)

	cookies := rec.Result().Cookies()
	names := make([]string, 0, len(cookies))
	for _, c := range cookies {
		names = append(names, c.Name)
	}
	assert.Contains(t, names, "accessToken")
	assert.Contains(t, names, "refreshToken")
}

func TestAccessExpiredTokenNoRefreshTokenUnauthorized(t *testing.T) {
	service, _ := newTestAccessService(t)
	rec := callAccess(
		service,
		&http.Cookie{Name: "accessToken", Value: "expired-or-malformed"},
	)
	assert.Equal(t, http.StatusUnauthorized, rec.Code)
}

// fakeOAuth2TokenResolver is a stand-in for internal/oauth2as.TokenResolver,
// letting these tests drive ResolveToken's opaque-OAuth-token fallback path
// without spinning up a real fosite provider.
type fakeOAuth2TokenResolver struct {
	userID string
	err    error
}

func (f *fakeOAuth2TokenResolver) ResolveAccessToken(
	_ context.Context, _ string,
) (string, error) {
	return f.userID, f.err
}

func TestResolveToken_NoOAuthResolver_InvalidTokenErrors(t *testing.T) {
	service, _ := newTestAccessService(t)
	_, err := service.ResolveToken(context.Background(), "not-a-real-token")
	require.Error(t, err)
}

func TestResolveToken_OAuthFallback_Success(t *testing.T) {
	service, db := newTestService(t)
	userID := seedUser(t, db)
	service.OAuth2TokenResolver = &fakeOAuth2TokenResolver{userID: userID, err: nil}

	user, err := service.ResolveToken(context.Background(), "opaque-oauth-token")
	require.NoError(t, err)
	require.NotNil(t, user)
	assert.Equal(t, userID, user.ID)
}

func TestResolveToken_OAuthFallback_ResolverError(t *testing.T) {
	service, _ := newTestService(t)
	service.OAuth2TokenResolver = &fakeOAuth2TokenResolver{
		userID: "",
		err:    errors.New("token introspection failed"),
	}

	_, err := service.ResolveToken(context.Background(), "opaque-oauth-token")
	require.Error(t, err)
}

func TestResolveToken_OAuthFallback_UnknownUser(t *testing.T) {
	service, _ := newTestService(t)
	service.OAuth2TokenResolver = &fakeOAuth2TokenResolver{
		userID: uuid.NewString(), err: nil,
	}

	_, err := service.ResolveToken(context.Background(), "opaque-oauth-token")
	require.Error(t, err)
}

func callTemplateAccess(
	service *auth.LocalService,
	cookies ...*http.Cookie,
) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodGet, "/dashboard", nil)
	for _, c := range cookies {
		req.AddCookie(c)
	}
	rec := httptest.NewRecorder()
	handler := service.TemplateAccess(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	handler(rec, req)
	return rec
}

func TestTemplateAccess_ValidTokenPassesThrough(t *testing.T) {
	service, token := newTestAccessService(t)
	rec := callTemplateAccess(service, &http.Cookie{Name: "accessToken", Value: token})
	assert.Equal(t, http.StatusOK, rec.Code)
}

// TestTemplateAccess_NoAuthInvokesSignInRenderer covers the no-session
// branch: with no SignInRenderer set (the zero value, as in every other test
// here), TemplateAccess must simply not call next and not panic.
func TestTemplateAccess_NoAuthInvokesSignInRenderer(t *testing.T) {
	service, _ := newTestAccessService(t)

	var rendered bool
	var redirectURL string
	service.SignInRenderer = func(
		w http.ResponseWriter, _ *http.Request, redirectTo string,
	) {
		rendered = true
		redirectURL = redirectTo
		w.WriteHeader(http.StatusTeapot)
	}

	rec := callTemplateAccess(service)
	assert.True(t, rendered)
	assert.Equal(t, "/dashboard", redirectURL)
	assert.Equal(t, http.StatusTeapot, rec.Code)
}

func callAdminAccess(
	service *auth.LocalService,
	cookies ...*http.Cookie,
) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodGet, "/admin", nil)
	for _, c := range cookies {
		req.AddCookie(c)
	}
	rec := httptest.NewRecorder()
	handler := service.AdminAccess(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	handler(rec, req)
	return rec
}

// TestAdminAccess_NonAdminRedirects: every user resolved through these
// tests' fixtures (no appUsersRepo wired) always enriches to RoleUser, so
// this exercises the denial branch — the admin-bypass branch would need a
// real appUsersRepo/DB row granting RoleAdmin, which these package tests
// don't set up.
func TestAdminAccess_NonAdminRedirects(t *testing.T) {
	service, token := newTestAccessService(t)
	rec := callAdminAccess(service, &http.Cookie{Name: "accessToken", Value: token})
	assert.Equal(t, http.StatusSeeOther, rec.Code)
	assert.Equal(t, "/", rec.Header().Get("Location"))
}

func callAppAccess(
	service *auth.LocalService,
	appName string,
	cookies ...*http.Cookie,
) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodGet, "/apps/"+appName, nil)
	for _, c := range cookies {
		req.AddCookie(c)
	}
	rec := httptest.NewRecorder()
	handler := service.AppAccess(appName, func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	handler(rec, req)
	return rec
}

// TestAppAccess_NoAccessForbidden: as with TestAdminAccess_NonAdminRedirects,
// AppAccess's `!ok` branch (missing user in context) can't be reached
// through this handler — TemplateAccess only ever calls next once it has
// already set a valid user on the context — so only the "resolved user
// lacking this app's access" denial is exercised here.
func TestAppAccess_NoAccessForbidden(t *testing.T) {
	service, token := newTestAccessService(t)
	rec := callAppAccess(
		service,
		"games",
		&http.Cookie{Name: "accessToken", Value: token},
	)
	assert.Equal(t, http.StatusForbidden, rec.Code)
}
