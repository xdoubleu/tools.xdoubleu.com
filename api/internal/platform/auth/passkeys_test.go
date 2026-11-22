package auth_test

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"tools.xdoubleu.com/internal/kit/testhelper"
	"tools.xdoubleu.com/internal/platform/auth"
)

const (
	testOrigin = "http://localhost:3000"
	testRPID   = "localhost"
)

// registrationResponse is what a software authenticator returns for a
// navigator.credentials.create() call: an ES256 credential, "none" attestation.
func registrationResponse(
	t *testing.T,
	optionsJSON []byte,
	origin string,
) []byte {
	t.Helper()
	response, _ := registrationResponseWithKey(t, optionsJSON, origin)
	return response
}

// softKey is a software authenticator's credential.
type softKey struct {
	key    *ecdsa.PrivateKey
	credID []byte
}

func registrationResponseWithKey(
	t *testing.T,
	optionsJSON []byte,
	origin string,
) ([]byte, *softKey) {
	t.Helper()
	var options struct {
		Challenge string `json:"challenge"`
		RP        struct {
			ID string `json:"id"`
		} `json:"rp"`
	}
	require.NoError(t, json.Unmarshal(optionsJSON, &options))
	require.Equal(t, testRPID, options.RP.ID)

	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)
	credID := make([]byte, 16)
	_, err = rand.Read(credID)
	require.NoError(t, err)

	x := key.X.FillBytes(make([]byte, 32))
	y := key.Y.FillBytes(make([]byte, 32))
	// COSE_Key: kty EC2, alg ES256, crv P-256, x, y.
	cose := append(
		[]byte{0xa5, 0x01, 0x02, 0x03, 0x26, 0x20, 0x01, 0x21, 0x58, 0x20},
		x...)
	cose = append(cose, 0x22, 0x58, 0x20)
	cose = append(cose, y...)

	rpHash := sha256.Sum256([]byte(testRPID))
	authData := append([]byte{}, rpHash[:]...)
	authData = append(authData, 0x45, 0, 0, 0, 0) // UP|UV|AT, sign count 0
	authData = append(authData, make([]byte, 16)...)
	authData = binary.BigEndian.AppendUint16(
		authData,
		uint16(len(credID)),
	)
	authData = append(authData, credID...)
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
	attObj = binary.BigEndian.AppendUint16(
		attObj,
		uint16(len(authData)),
	)
	attObj = append(attObj, authData...)

	clientData, err := json.Marshal(map[string]any{
		"type":      "webauthn.create",
		"challenge": options.Challenge,
		"origin":    origin,
	})
	require.NoError(t, err)

	out, err := json.Marshal(map[string]any{
		"id":    base64.RawURLEncoding.EncodeToString(credID),
		"rawId": base64.RawURLEncoding.EncodeToString(credID),
		"type":  "public-key",
		"response": map[string]any{
			"clientDataJSON":    base64.RawURLEncoding.EncodeToString(clientData),
			"attestationObject": base64.RawURLEncoding.EncodeToString(attObj),
		},
	})
	require.NoError(t, err)
	return out, &softKey{key: key, credID: credID}
}

// signedInToken signs in for real, so its auth_time is now.
func signedInToken(t *testing.T) (*auth.LocalService, string, string) {
	t.Helper()
	service, db := newTestService(t)
	userID := seedUser(t, db)
	access, _, err := service.SignInWithEmail(
		t.Context(), userID+"@example.com", testPassword,
	)
	require.NoError(t, err)
	return service, userID, *access
}

// staleToken is a valid session whose sign-in happened ago.
func staleToken(t *testing.T, userID string, ago time.Duration) string {
	t.Helper()
	now := time.Now()
	tok := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"sub":       userID,
		"iat":       now.Unix(),
		"exp":       now.Add(time.Hour).Unix(),
		"aal":       "aal1",
		"auth_time": now.Add(-ago).Unix(),
	})
	signed, err := tok.SignedString([]byte(testhelper.NewTestConfig().JWTSecret))
	require.NoError(t, err)
	return signed
}

func addPasskey(
	t *testing.T,
	service *auth.LocalService,
	token, name string,
) *auth.Passkey {
	t.Helper()
	options, err := service.BeginPasskeyRegistration(t.Context(), token)
	require.NoError(t, err)
	passkey, err := service.FinishPasskeyRegistration(
		t.Context(), token, name, registrationResponse(t, options, testOrigin),
	)
	require.NoError(t, err)
	return passkey
}

func TestPasskeys_RegisterListRenameRemove(t *testing.T) {
	service, _, token := signedInToken(t)

	passkey := addPasskey(t, service, token, "  Laptop  ")
	assert.Equal(t, "Laptop", passkey.Name)
	assert.Nil(t, passkey.LastUsedAt)

	list, err := service.ListPasskeys(t.Context(), token)
	require.NoError(t, err)
	require.Len(t, list, 1)
	assert.Equal(t, passkey.ID, list[0].ID)

	require.NoError(t, service.RenamePasskey(t.Context(), token, passkey.ID, "Phone"))
	list, err = service.ListPasskeys(t.Context(), token)
	require.NoError(t, err)
	assert.Equal(t, "Phone", list[0].Name)

	require.NoError(t, service.RemovePasskey(t.Context(), token, passkey.ID))
	list, err = service.ListPasskeys(t.Context(), token)
	require.NoError(t, err)
	assert.Empty(t, list)
}

func TestPasskeys_BeginExcludesExistingCredentials(t *testing.T) {
	service, _, token := signedInToken(t)
	addPasskey(t, service, token, "One")

	options, err := service.BeginPasskeyRegistration(t.Context(), token)
	require.NoError(t, err)
	var parsed struct {
		ExcludeCredentials []json.RawMessage `json:"excludeCredentials"`
	}
	require.NoError(t, json.Unmarshal(options, &parsed))
	assert.Len(t, parsed.ExcludeCredentials, 1)
}

func TestPasskeys_StaleSignInRefusedForAddAndRemove(t *testing.T) {
	service, userID, token := signedInToken(t)
	passkey := addPasskey(t, service, token, "Laptop")

	stale := staleToken(t, userID, passkeyRecent+time.Minute)

	_, err := service.BeginPasskeyRegistration(t.Context(), stale)
	require.ErrorIs(t, err, auth.ErrRecentSignInRequired)

	err = service.RemovePasskey(t.Context(), stale, passkey.ID)
	require.ErrorIs(t, err, auth.ErrRecentSignInRequired)

	// Listing and renaming need only a session.
	list, err := service.ListPasskeys(t.Context(), stale)
	require.NoError(t, err)
	assert.Len(t, list, 1)
	require.NoError(t, service.RenamePasskey(t.Context(), stale, passkey.ID, "Renamed"))
}

func TestPasskeys_TokenWithoutAuthTimeIsStale(t *testing.T) {
	service, userID, _ := signedInToken(t)
	now := time.Now()
	tok := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"sub": userID, "iat": now.Unix(), "exp": now.Add(time.Hour).Unix(),
		"aal": "aal1",
	})
	signed, err := tok.SignedString([]byte(testhelper.NewTestConfig().JWTSecret))
	require.NoError(t, err)

	_, err = service.BeginPasskeyRegistration(t.Context(), signed)
	require.ErrorIs(t, err, auth.ErrRecentSignInRequired)
}

func TestPasskeys_RefreshKeepsSignInTime(t *testing.T) {
	service, db := newTestService(t)
	userID := seedUser(t, db)
	_, refresh, err := service.SignInWithEmail(
		t.Context(), userID+"@example.com", testPassword,
	)
	require.NoError(t, err)

	// Age the chain's sign-in, then rotate: the new access token must not look
	// freshly signed in.
	_, err = db.Exec(t.Context(), `
		UPDATE auth.refresh_tokens SET auth_time = now() - interval '1 hour'
		WHERE user_id = $1
	`, userID)
	require.NoError(t, err)
	refreshed, _, err := service.SignInWithRefreshToken(t.Context(), *refresh)
	require.NoError(t, err)

	_, err = service.BeginPasskeyRegistration(t.Context(), *refreshed)
	require.ErrorIs(t, err, auth.ErrRecentSignInRequired)
}

func TestPasskeys_WrongOriginRefused(t *testing.T) {
	service, _, token := signedInToken(t)

	options, err := service.BeginPasskeyRegistration(t.Context(), token)
	require.NoError(t, err)
	_, err = service.FinishPasskeyRegistration(
		t.Context(), token, "Evil",
		registrationResponse(t, options, "https://evil.example"),
	)
	require.ErrorIs(t, err, auth.ErrPasskeyInvalid)

	list, err := service.ListPasskeys(t.Context(), token)
	require.NoError(t, err)
	assert.Empty(t, list)
}

func TestPasskeys_ChallengeIsSingleUse(t *testing.T) {
	service, _, token := signedInToken(t)

	options, err := service.BeginPasskeyRegistration(t.Context(), token)
	require.NoError(t, err)
	response := registrationResponse(t, options, testOrigin)

	_, err = service.FinishPasskeyRegistration(t.Context(), token, "One", response)
	require.NoError(t, err)
	_, err = service.FinishPasskeyRegistration(t.Context(), token, "Again", response)
	require.ErrorIs(t, err, auth.ErrPasskeyInvalid)
}

func TestPasskeys_ExpiredChallengeRefused(t *testing.T) {
	service, db := newTestService(t)
	userID := seedUser(t, db)
	access, _, err := service.SignInWithEmail(
		t.Context(), userID+"@example.com", testPassword,
	)
	require.NoError(t, err)

	options, err := service.BeginPasskeyRegistration(t.Context(), *access)
	require.NoError(t, err)
	_, err = db.Exec(t.Context(), `
		UPDATE auth.passkey_challenges SET expires_at = now() - interval '1 second'
		WHERE user_id = $1
	`, userID)
	require.NoError(t, err)

	_, err = service.FinishPasskeyRegistration(
		t.Context(), *access, "Late", registrationResponse(t, options, testOrigin),
	)
	require.ErrorIs(t, err, auth.ErrPasskeyInvalid)
}

func TestPasskeys_BadNameRefused(t *testing.T) {
	service, _, token := signedInToken(t)

	_, err := service.FinishPasskeyRegistration(t.Context(), token, "  ", nil)
	require.ErrorIs(t, err, auth.ErrPasskeyInvalid)
	err = service.RenamePasskey(t.Context(), token, uuid.New(), "")
	require.ErrorIs(t, err, auth.ErrPasskeyInvalid)
}

func TestPasskeys_OtherUsersPasskeyNotFound(t *testing.T) {
	service, _, token := signedInToken(t)
	passkey := addPasskey(t, service, token, "Mine")
	_, _, otherToken := signedInToken(t)

	err := service.RemovePasskey(t.Context(), otherToken, passkey.ID)
	require.ErrorIs(t, err, auth.ErrPasskeyNotFound)
	err = service.RenamePasskey(t.Context(), otherToken, passkey.ID, "Stolen")
	require.ErrorIs(t, err, auth.ErrPasskeyNotFound)
}

func TestPasskeys_RequireSession(t *testing.T) {
	service, _, _ := signedInToken(t)

	_, err := service.ListPasskeys(t.Context(), "not-a-token")
	require.Error(t, err)
}

func TestPasskeys_FinishWithoutBegin(t *testing.T) {
	service, _, token := signedInToken(t)

	_, err := service.FinishPasskeyRegistration(t.Context(), token, "X", []byte("{}"))
	require.ErrorIs(t, err, auth.ErrPasskeyInvalid)
}

const passkeyRecent = 5 * time.Minute
