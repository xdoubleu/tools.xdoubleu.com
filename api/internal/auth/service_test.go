package auth_test

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"tools.xdoubleu.com/internal/auth"
	"tools.xdoubleu.com/internal/crypto"
	"tools.xdoubleu.com/internal/mailer"
	"tools.xdoubleu.com/internal/testhelper"
)

// capturingMailer records the last email sent, for tests to pull the
// plaintext reset token back out of the link (ForgotPassword never returns
// it directly, only emails it).
type capturingMailer struct {
	lastBody string
}

func (m *capturingMailer) Send(_ context.Context, _, body string) error {
	m.lastBody = body
	return nil
}

func (m *capturingMailer) SendTo(_ context.Context, _, _, body string) error {
	m.lastBody = body
	return nil
}

var _ mailer.Client = (*capturingMailer)(nil)

func newTestService(t *testing.T) (*auth.LocalService, *pgxpool.Pool) {
	t.Helper()
	cfg := testhelper.NewTestConfig()
	db := testhelper.ConnectTestDB(cfg.DBDsn)
	t.Cleanup(db.Close)

	sealer, err := crypto.New(cfg.EncryptionKey)
	require.NoError(t, err)

	return auth.NewService(
		cfg, auth.NewRepository(db), nil, sealer, mailer.New("", "", ""),
	), db
}

func TestSignInWithEmail_Success(t *testing.T) {
	service, db := newTestService(t)
	userID := seedUser(t, db)

	access, refresh, err := service.SignInWithEmail(
		context.Background(), userID+"@example.com", testPassword,
	)
	require.NoError(t, err)
	assert.NotEmpty(t, *access)
	assert.NotEmpty(t, *refresh)
}

func TestSignInWithEmail_WrongPassword(t *testing.T) {
	service, db := newTestService(t)
	userID := seedUser(t, db)

	_, _, err := service.SignInWithEmail(
		context.Background(), userID+"@example.com", "wrong-password",
	)
	require.Error(t, err)
}

func TestSignInWithEmail_NoUser(t *testing.T) {
	service, _ := newTestService(t)
	_, _, err := service.SignInWithEmail(
		context.Background(), "nobody@example.com", testPassword,
	)
	require.Error(t, err)
}

func TestRefreshToken_RotatesAndInvalidatesOldToken(t *testing.T) {
	service, db := newTestService(t)
	userID := seedUser(t, db)

	_, refresh, err := service.SignInWithEmail(
		context.Background(), userID+"@example.com", testPassword,
	)
	require.NoError(t, err)

	newAccess, newRefresh, err := service.SignInWithRefreshToken(
		context.Background(), *refresh,
	)
	require.NoError(t, err)
	assert.NotEmpty(t, *newAccess)
	assert.NotEqual(t, *refresh, *newRefresh)

	// The old (rotated-out) refresh token must no longer work.
	_, _, err = service.SignInWithRefreshToken(context.Background(), *refresh)
	require.Error(t, err)
}

func TestSignOut_RevokesRefreshTokens(t *testing.T) {
	service, db := newTestService(t)
	userID := seedUser(t, db)

	access, refresh, err := service.SignInWithEmail(
		context.Background(), userID+"@example.com", testPassword,
	)
	require.NoError(t, err)

	deleteAccess, deleteRefresh, err := service.SignOut(
		context.Background(), *access, false,
	)
	require.NoError(t, err)
	assert.Equal(t, -1, deleteAccess.MaxAge)
	assert.Equal(t, -1, deleteRefresh.MaxAge)

	_, _, err = service.SignInWithRefreshToken(context.Background(), *refresh)
	require.Error(t, err)
}

func TestUpdatePassword_RevokesAllRefreshTokens(t *testing.T) {
	service, db := newTestService(t)
	userID := seedUser(t, db)

	access, refresh, err := service.SignInWithEmail(
		context.Background(), userID+"@example.com", testPassword,
	)
	require.NoError(t, err)

	require.NoError(t, service.UpdatePassword(
		context.Background(), *access, "a-new-password",
	))

	_, _, err = service.SignInWithRefreshToken(context.Background(), *refresh)
	require.Error(t, err)

	// The new password now works; the old one doesn't.
	_, _, err = service.SignInWithEmail(
		context.Background(), userID+"@example.com", "a-new-password",
	)
	require.NoError(t, err)
	_, _, err = service.SignInWithEmail(
		context.Background(), userID+"@example.com", testPassword,
	)
	require.Error(t, err)
}

func TestForgotPasswordResetPassword_RoundTrip(t *testing.T) {
	db := testhelper.ConnectTestDB(testhelper.NewTestConfig().DBDsn)
	t.Cleanup(db.Close)
	//nolint:exhaustruct //lastBody is populated by SendTo
	mail := &capturingMailer{}
	service := auth.NewService(
		testhelper.NewTestConfig(), auth.NewRepository(db), nil, nil, mail,
	)
	userID := seedUser(t, db)
	email := userID + "@example.com"

	require.NoError(t, service.ForgotPassword(context.Background(), email, "/reset"))
	require.Contains(t, mail.lastBody, "token=")

	idx := strings.Index(mail.lastBody, "token=")
	token := strings.TrimSpace(mail.lastBody[idx+len("token="):])
	require.NotEmpty(t, token)

	require.NoError(t, service.ResetPasswordWithToken(
		context.Background(), token, "brand-new-password",
	))

	_, _, err := service.SignInWithEmail(
		context.Background(), email, "brand-new-password",
	)
	require.NoError(t, err)

	// The reset token is single-use.
	err = service.ResetPasswordWithToken(
		context.Background(), token, "another-password",
	)
	require.Error(t, err)
}

func TestGetAllUsers_NilRepoReturnsEmpty(t *testing.T) {
	service, _ := newTestService(t)
	users, err := service.GetAllUsers(context.Background())
	require.NoError(t, err)
	assert.Empty(t, users)
}

func TestSignInWithRefreshToken_ExpiredToken(t *testing.T) {
	service, db := newTestService(t)
	userID := seedUser(t, db)

	// Insert an already-expired refresh token directly, bypassing the
	// service's own expiry TTL so this test doesn't need to sleep.
	rawToken := "expired-refresh-token-" + uuid.NewString()
	sum := sha256.Sum256([]byte(rawToken))
	tokenHash := hex.EncodeToString(sum[:])
	_, err := db.Exec(context.Background(), `
		INSERT INTO auth.refresh_tokens (user_id, token_hash, aal, expires_at)
		VALUES ($1, $2, 'aal1', now() - interval '1 hour')
	`, userID, tokenHash)
	require.NoError(t, err)

	_, _, err = service.SignInWithRefreshToken(context.Background(), rawToken)
	require.Error(t, err)

	// The expired token is deleted as a side effect, so a second attempt
	// fails with "not found" rather than "expired" but is still an error.
	_, _, err = service.SignInWithRefreshToken(context.Background(), rawToken)
	require.Error(t, err)
}

func TestRefreshSession_Success(t *testing.T) {
	service, db := newTestService(t)
	userID := seedUser(t, db)

	_, refresh, err := service.SignInWithEmail(
		context.Background(), userID+"@example.com", testPassword,
	)
	require.NoError(t, err)

	user, accessCookie, refreshCookie, err := service.RefreshSession(
		context.Background(), *refresh,
	)
	require.NoError(t, err)
	require.NotNil(t, user)
	assert.Equal(t, userID, user.ID)
	assert.NotEmpty(t, accessCookie.Value)
	assert.NotEmpty(t, refreshCookie.Value)
	assert.NotEqual(t, *refresh, refreshCookie.Value)
}

func TestRefreshSession_InvalidToken(t *testing.T) {
	service, _ := newTestService(t)

	user, accessCookie, refreshCookie, err := service.RefreshSession(
		context.Background(), "not-a-real-refresh-token",
	)
	require.Error(t, err)
	assert.Nil(t, user)
	assert.Nil(t, accessCookie)
	assert.Nil(t, refreshCookie)
}

func TestUpdatePassword_InvalidAccessToken(t *testing.T) {
	service, _ := newTestService(t)

	err := service.UpdatePassword(
		context.Background(), "not-a-real-access-token", "a-new-password",
	)
	require.Error(t, err)
}

func TestResetPasswordWithToken_ExpiredToken(t *testing.T) {
	db := testhelper.ConnectTestDB(testhelper.NewTestConfig().DBDsn)
	t.Cleanup(db.Close)
	service := auth.NewService(
		testhelper.NewTestConfig(), auth.NewRepository(db), nil, nil,
		mailer.New("", "", ""),
	)
	userID := seedUser(t, db)

	// Insert an already-expired reset token directly, bypassing
	// ForgotPassword's TTL so this test doesn't need to sleep.
	rawToken := "expired-reset-token-" + uuid.NewString()
	sum := sha256.Sum256([]byte(rawToken))
	tokenHash := hex.EncodeToString(sum[:])
	_, err := db.Exec(context.Background(), `
		INSERT INTO auth.password_reset_tokens (user_id, token_hash, expires_at)
		VALUES ($1, $2, now() - interval '1 hour')
	`, userID, tokenHash)
	require.NoError(t, err)

	err = service.ResetPasswordWithToken(
		context.Background(), rawToken, "brand-new-password",
	)
	require.Error(t, err)
}

func TestForgotPassword_UnknownEmailSwallowsError(t *testing.T) {
	service, _ := newTestService(t)
	require.NoError(t, service.ForgotPassword(
		context.Background(), "nobody@example.com", "/reset",
	))
}

func TestResetPasswordWithToken_InvalidToken(t *testing.T) {
	service, _ := newTestService(t)
	err := service.ResetPasswordWithToken(
		context.Background(), "not-a-real-token", "new-password",
	)
	require.Error(t, err)
}
