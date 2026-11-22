package main

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"net/http"
	"net/url"
	"strings"
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

func TestPasskeySignInRPCs_UnauthenticatedBeginAndBadFinish(t *testing.T) {
	client := mfaClient(t)
	ctx := context.Background()

	begin, err := client.PasskeySignInBegin(ctx, &authv1.PasskeySignInBeginRequest{})
	require.NoError(t, err)
	assert.Contains(t, begin.OptionsJson, `"challenge"`)

	_, err = client.PasskeySignInFinish(ctx, &authv1.PasskeySignInFinishRequest{
		CredentialJson: "{}",
	})
	requireCode(t, err, connect.CodeUnauthenticated)
}

// softAuthenticator answers WebAuthn ceremonies for one ES256 credential.
type softAuthenticator struct {
	key    *ecdsa.PrivateKey
	credID []byte
	origin string
	rpID   string
}

func newSoftAuthenticator(t *testing.T) *softAuthenticator {
	t.Helper()
	webURL, err := url.Parse(testhelper.NewTestConfig().WebURL)
	require.NoError(t, err)
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)
	return &softAuthenticator{
		key:    key,
		credID: []byte(uuid.NewString()),
		origin: webURL.Scheme + "://" + webURL.Host,
		rpID:   webURL.Hostname(),
	}
}

func challengeOf(t *testing.T, optionsJSON string) string {
	t.Helper()
	var options struct {
		Challenge string `json:"challenge"`
	}
	require.NoError(t, json.Unmarshal([]byte(optionsJSON), &options))
	return options.Challenge
}

func (a *softAuthenticator) clientData(t *testing.T, typ, challenge string) []byte {
	t.Helper()
	data, err := json.Marshal(map[string]any{
		"type": typ, "challenge": challenge, "origin": a.origin,
	})
	require.NoError(t, err)
	return data
}

func (a *softAuthenticator) register(t *testing.T, optionsJSON string) string {
	t.Helper()
	x := a.key.X.FillBytes(make([]byte, 32))
	y := a.key.Y.FillBytes(make([]byte, 32))
	cose := append(
		[]byte{0xa5, 0x01, 0x02, 0x03, 0x26, 0x20, 0x01, 0x21, 0x58, 0x20}, x...,
	)
	cose = append(cose, 0x22, 0x58, 0x20)
	cose = append(cose, y...)

	rpHash := sha256.Sum256([]byte(a.rpID))
	authData := append([]byte{}, rpHash[:]...)
	authData = append(authData, 0x45, 0, 0, 0, 0) // UP|UV|AT, sign count 0
	authData = append(authData, make([]byte, 16)...)
	authData = binary.BigEndian.AppendUint16(authData, uint16(len(a.credID)))
	authData = append(authData, a.credID...)
	authData = append(authData, cose...)

	attObj := []byte{0xa3, 0x63}
	attObj = append(attObj, "fmt"...)
	attObj = append(attObj, 0x64)
	attObj = append(attObj, "none"...)
	attObj = append(attObj, 0x67)
	attObj = append(attObj, "attStmt"...)
	attObj = append(attObj, 0xa0, 0x68)
	attObj = append(attObj, "authData"...)
	attObj = append(attObj, 0x59)
	attObj = binary.BigEndian.AppendUint16(attObj, uint16(len(authData)))
	attObj = append(attObj, authData...)

	enc := base64.RawURLEncoding.EncodeToString
	out, err := json.Marshal(map[string]any{
		"id": enc(a.credID), "rawId": enc(a.credID), "type": "public-key",
		"response": map[string]any{
			"clientDataJSON": enc(a.clientData(
				t, "webauthn.create", challengeOf(t, optionsJSON),
			)),
			"attestationObject": enc(attObj),
		},
	})
	require.NoError(t, err)
	return string(out)
}

func (a *softAuthenticator) assert(
	t *testing.T,
	optionsJSON, userHandle string,
) string {
	t.Helper()
	rpHash := sha256.Sum256([]byte(a.rpID))
	authData := append([]byte{}, rpHash[:]...)
	authData = append(authData, 0x05, 0, 0, 0, 0) // UP|UV, sign count 0

	clientData := a.clientData(t, "webauthn.get", challengeOf(t, optionsJSON))
	clientHash := sha256.Sum256(clientData)
	digest := sha256.Sum256(append(append([]byte{}, authData...), clientHash[:]...))
	sig, err := ecdsa.SignASN1(rand.Reader, a.key, digest[:])
	require.NoError(t, err)

	enc := base64.RawURLEncoding.EncodeToString
	out, err := json.Marshal(map[string]any{
		"id": enc(a.credID), "rawId": enc(a.credID), "type": "public-key",
		"response": map[string]any{
			"clientDataJSON":    enc(clientData),
			"authenticatorData": enc(authData),
			"signature":         enc(sig),
			"userHandle":        enc([]byte(userHandle)),
		},
	})
	require.NoError(t, err)
	return string(out)
}

func TestPasskeySignInRPCs_SignInSetsSessionCookie(t *testing.T) {
	client := mfaClient(t)
	token := freshTestUser(t)
	authenticator := newSoftAuthenticator(t)

	ctx := withCookies(
		context.Background(),
		http.Cookie{Name: "accessToken", Value: token},
	)
	regBegin, err := client.PasskeyRegisterBegin(
		ctx, &authv1.PasskeyRegisterBeginRequest{},
	)
	require.NoError(t, err)
	_, err = client.PasskeyRegisterFinish(ctx, &authv1.PasskeyRegisterFinishRequest{
		Name: "Laptop", CredentialJson: authenticator.register(t, regBegin.OptionsJson),
	})
	require.NoError(t, err)

	anon := context.Background()
	begin, err := client.PasskeySignInBegin(anon, &authv1.PasskeySignInBeginRequest{})
	require.NoError(t, err)
	anon, info := connect.NewClientContext(anon)
	_, err = client.PasskeySignInFinish(anon, &authv1.PasskeySignInFinishRequest{
		CredentialJson: authenticator.assert(t, begin.OptionsJson, subjectOf(t, token)),
		RememberMe:     true,
	})
	require.NoError(t, err)

	cookies := strings.Join(info.ResponseHeader().Values("Set-Cookie"), "\n")
	assert.Contains(t, cookies, "accessToken=")
	assert.Contains(t, cookies, "refreshToken=")
}
