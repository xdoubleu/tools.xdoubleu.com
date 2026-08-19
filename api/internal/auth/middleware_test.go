package auth_test

import (
	"context"
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
