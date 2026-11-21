package main

import (
	"context"
	"net/http"
	"testing"
	"time"

	"connectrpc.com/connect/v2"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	authv1 "tools.xdoubleu.com/gen/auth/v1"
	"tools.xdoubleu.com/internal/kit/testhelper"
)

func requireCode(t *testing.T, err error, want connect.Code) {
	t.Helper()
	require.Error(t, err)
	var connectErr *connect.Error
	require.ErrorAs(t, err, &connectErr)
	assert.Equal(t, want, connectErr.Code())
}

// signedAccessToken is a valid session token for userID signed in ago.
func signedAccessToken(t *testing.T, userID string, ago time.Duration) string {
	t.Helper()
	now := time.Now()
	tok := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"sub": userID, "iat": now.Unix(), "exp": now.Add(time.Hour).Unix(),
		"aal": "aal1", "auth_time": now.Add(-ago).Unix(),
	})
	signed, err := tok.SignedString([]byte(testhelper.NewTestConfig().JWTSecret))
	require.NoError(t, err)
	return signed
}

func subjectOf(t *testing.T, token string) string {
	t.Helper()
	var claims jwt.RegisteredClaims
	_, _, err := jwt.NewParser().ParseUnverified(token, &claims)
	require.NoError(t, err)
	return claims.Subject
}

func TestPasskeyRPCs_RequireSignIn(t *testing.T) {
	client := mfaClient(t)
	ctx := context.Background()

	_, err := client.PasskeyRegisterBegin(ctx, &authv1.PasskeyRegisterBeginRequest{})
	requireCode(t, err, connect.CodeUnauthenticated)
	_, err = client.PasskeyRegisterFinish(ctx, &authv1.PasskeyRegisterFinishRequest{})
	requireCode(t, err, connect.CodeUnauthenticated)
	_, err = client.ListPasskeys(ctx, &authv1.ListPasskeysRequest{})
	requireCode(t, err, connect.CodeUnauthenticated)
	_, err = client.RenamePasskey(ctx, &authv1.RenamePasskeyRequest{})
	requireCode(t, err, connect.CodeUnauthenticated)
	_, err = client.RemovePasskey(ctx, &authv1.RemovePasskeyRequest{})
	requireCode(t, err, connect.CodeUnauthenticated)
}

func TestPasskeyRPCs_BeginAndBadFinish(t *testing.T) {
	client := mfaClient(t)
	ctx := withCookies(
		context.Background(),
		http.Cookie{Name: "accessToken", Value: freshTestUser(t)},
	)

	begin, err := client.PasskeyRegisterBegin(ctx, &authv1.PasskeyRegisterBeginRequest{})
	require.NoError(t, err)
	assert.Contains(t, begin.OptionsJson, `"challenge"`)

	_, err = client.PasskeyRegisterFinish(ctx, &authv1.PasskeyRegisterFinishRequest{
		Name: "Laptop", CredentialJson: "{}",
	})
	requireCode(t, err, connect.CodeInvalidArgument)

	_, err = client.PasskeyRegisterFinish(ctx, &authv1.PasskeyRegisterFinishRequest{
		Name: " ", CredentialJson: "{}",
	})
	requireCode(t, err, connect.CodeInvalidArgument)
}

func TestPasskeyRPCs_ListRenameRemove(t *testing.T) {
	client := mfaClient(t)
	token := freshTestUser(t)
	userID := subjectOf(t, token)
	ctx := withCookies(
		context.Background(),
		http.Cookie{Name: "accessToken", Value: token},
	)

	id := uuid.NewString()
	_, err := testApp.db.Exec(context.Background(), `
		INSERT INTO auth.passkeys (id, user_id, credential_id, public_key,
			attestation_type, aaguid, flags, name, last_used_at)
		VALUES ($1, $2, $3, '\x00', 'none', '\x00', 69, 'Old', now())
	`, id, userID, []byte(id))
	require.NoError(t, err)

	list, err := client.ListPasskeys(ctx, &authv1.ListPasskeysRequest{})
	require.NoError(t, err)
	require.Len(t, list.Passkeys, 1)
	assert.Equal(t, id, list.Passkeys[0].Id)
	assert.NotEmpty(t, list.Passkeys[0].LastUsedAt)

	_, err = client.RenamePasskey(ctx, &authv1.RenamePasskeyRequest{Id: id, Name: "New"})
	require.NoError(t, err)
	list, err = client.ListPasskeys(ctx, &authv1.ListPasskeysRequest{})
	require.NoError(t, err)
	assert.Equal(t, "New", list.Passkeys[0].Name)

	_, err = client.RenamePasskey(ctx, &authv1.RenamePasskeyRequest{
		Id: uuid.NewString(), Name: "x",
	})
	requireCode(t, err, connect.CodeNotFound)
	_, err = client.RenamePasskey(ctx, &authv1.RenamePasskeyRequest{Id: "nope", Name: "x"})
	requireCode(t, err, connect.CodeInvalidArgument)

	_, err = client.RemovePasskey(ctx, &authv1.RemovePasskeyRequest{Id: "nope"})
	requireCode(t, err, connect.CodeInvalidArgument)
	_, err = client.RemovePasskey(ctx, &authv1.RemovePasskeyRequest{Id: id})
	require.NoError(t, err)
	list, err = client.ListPasskeys(ctx, &authv1.ListPasskeysRequest{})
	require.NoError(t, err)
	assert.Empty(t, list.Passkeys)
}

func TestPasskeyRPCs_StaleSignInNeedsReauthentication(t *testing.T) {
	client := mfaClient(t)
	stale := signedAccessToken(t, subjectOf(t, freshTestUser(t)), time.Hour)
	ctx := withCookies(
		context.Background(),
		http.Cookie{Name: "accessToken", Value: stale},
	)

	_, err := client.PasskeyRegisterBegin(ctx, &authv1.PasskeyRegisterBeginRequest{})
	requireCode(t, err, connect.CodeFailedPrecondition)
	_, err = client.RemovePasskey(ctx, &authv1.RemovePasskeyRequest{Id: uuid.NewString()})
	requireCode(t, err, connect.CodeFailedPrecondition)
}
