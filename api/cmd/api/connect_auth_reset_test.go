package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"testing"
	"time"

	"connectrpc.com/connect"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/bcrypt"

	authv1 "tools.xdoubleu.com/gen/auth/v1"
)

// seedResetUserWithToken inserts a brand-new auth.users row plus a matching
// password_reset_tokens row (bypassing ForgotPassword, which only ever
// emails the plaintext token rather than returning it), and returns the raw
// token and the user's email.
func seedResetUserWithToken(
	t *testing.T, expiresAt time.Time, usedAt *time.Time,
) (string, string) {
	t.Helper()
	ctx := context.Background()

	id := uuid.New().String()
	email := id + "@example.com"

	hash, err := bcrypt.GenerateFromPassword(
		[]byte(testUserPassword), bcrypt.DefaultCost,
	)
	require.NoError(t, err)

	_, err = testApp.db.Exec(ctx, `
		INSERT INTO auth.users (id, email, password_hash) VALUES ($1, $2, $3)
	`, id, email, string(hash))
	require.NoError(t, err)

	rawToken := "reset-token-" + uuid.NewString()
	sum := sha256.Sum256([]byte(rawToken))
	tokenHash := hex.EncodeToString(sum[:])

	_, err = testApp.db.Exec(ctx, `
		INSERT INTO auth.password_reset_tokens (user_id, token_hash, expires_at, used_at)
		VALUES ($1, $2, $3, $4)
	`, id, tokenHash, expiresAt, usedAt)
	require.NoError(t, err)

	return rawToken, email
}

func TestResetPassword_Success(t *testing.T) {
	client := authClient(t)
	token, email := seedResetUserWithToken(t, time.Now().Add(time.Hour), nil)

	_, err := client.ResetPassword(context.Background(), connect.NewRequest(
		&authv1.ResetPasswordRequest{
			Token:       token,
			NewPassword: "a-brand-new-password",
		},
	))
	require.NoError(t, err)

	// The new password now works.
	signInResp, err := client.SignIn(context.Background(), connect.NewRequest(
		&authv1.SignInRequest{
			Email:    email,
			Password: "a-brand-new-password",
			Redirect: "/",
		},
	))
	require.NoError(t, err)
	assert.False(t, signInResp.Msg.NeedsMfa)
}

func TestResetPassword_EmptyToken(t *testing.T) {
	client := authClient(t)
	_, err := client.ResetPassword(context.Background(), connect.NewRequest(
		&authv1.ResetPasswordRequest{
			Token:       "",
			NewPassword: "a-brand-new-password",
		},
	))
	require.Error(t, err)
	var connectErr *connect.Error
	require.ErrorAs(t, err, &connectErr)
	assert.Equal(t, connect.CodeInvalidArgument, connectErr.Code())
}

func TestResetPassword_EmptyNewPassword(t *testing.T) {
	client := authClient(t)
	_, err := client.ResetPassword(context.Background(), connect.NewRequest(
		&authv1.ResetPasswordRequest{
			Token:       "some-token",
			NewPassword: "",
		},
	))
	require.Error(t, err)
	var connectErr *connect.Error
	require.ErrorAs(t, err, &connectErr)
	assert.Equal(t, connect.CodeInvalidArgument, connectErr.Code())
}

func TestResetPassword_InvalidToken(t *testing.T) {
	client := authClient(t)
	_, err := client.ResetPassword(context.Background(), connect.NewRequest(
		&authv1.ResetPasswordRequest{
			Token:       "not-a-real-token",
			NewPassword: "a-brand-new-password",
		},
	))
	require.Error(t, err)
	var connectErr *connect.Error
	require.ErrorAs(t, err, &connectErr)
	assert.Equal(t, connect.CodeUnauthenticated, connectErr.Code())
}

func TestResetPassword_ExpiredToken(t *testing.T) {
	client := authClient(t)
	token, _ := seedResetUserWithToken(t, time.Now().Add(-time.Hour), nil)

	_, err := client.ResetPassword(context.Background(), connect.NewRequest(
		&authv1.ResetPasswordRequest{
			Token:       token,
			NewPassword: "a-brand-new-password",
		},
	))
	require.Error(t, err)
	var connectErr *connect.Error
	require.ErrorAs(t, err, &connectErr)
	assert.Equal(t, connect.CodeUnauthenticated, connectErr.Code())
}

func TestResetPassword_AlreadyUsedToken(t *testing.T) {
	client := authClient(t)
	usedAt := time.Now().Add(-time.Minute)
	token, _ := seedResetUserWithToken(t, time.Now().Add(time.Hour), &usedAt)

	_, err := client.ResetPassword(context.Background(), connect.NewRequest(
		&authv1.ResetPasswordRequest{
			Token:       token,
			NewPassword: "a-brand-new-password",
		},
	))
	require.Error(t, err)
	var connectErr *connect.Error
	require.ErrorAs(t, err, &connectErr)
	assert.Equal(t, connect.CodeUnauthenticated, connectErr.Code())
}
