package auth_test

import (
	"crypto/ecdsa"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"tools.xdoubleu.com/internal/platform/auth"
)

// assertionResponse is what a software authenticator returns for a
// navigator.credentials.get() call, user-verified, reporting signCount.
func (k *softKey) assertionResponse(
	t *testing.T,
	optionsJSON []byte,
	origin, userHandle string,
	signCount uint32,
) []byte {
	t.Helper()
	var options struct {
		Challenge string `json:"challenge"`
	}
	require.NoError(t, json.Unmarshal(optionsJSON, &options))

	rpHash := sha256.Sum256([]byte(testRPID))
	authData := append([]byte{}, rpHash[:]...)
	authData = append(authData, 0x05) // UP|UV
	authData = binary.BigEndian.AppendUint32(authData, signCount)

	clientData, err := json.Marshal(map[string]any{
		"type":      "webauthn.get",
		"challenge": options.Challenge,
		"origin":    origin,
	})
	require.NoError(t, err)
	clientHash := sha256.Sum256(clientData)
	digest := sha256.Sum256(append(append([]byte{}, authData...), clientHash[:]...))
	sig, err := ecdsa.SignASN1(rand.Reader, k.key, digest[:])
	require.NoError(t, err)

	enc := base64.RawURLEncoding.EncodeToString
	out, err := json.Marshal(map[string]any{
		"id":    enc(k.credID),
		"rawId": enc(k.credID),
		"type":  "public-key",
		"response": map[string]any{
			"clientDataJSON":    enc(clientData),
			"authenticatorData": enc(authData),
			"signature":         enc(sig),
			"userHandle":        enc([]byte(userHandle)),
		},
	})
	require.NoError(t, err)
	return out
}

// passkeyAccount is a user with one registered passkey.
func passkeyAccount(t *testing.T) (*auth.LocalService, string, *softKey) {
	t.Helper()
	service, userID, token := signedInToken(t)
	options, err := service.BeginPasskeyRegistration(t.Context(), token)
	require.NoError(t, err)
	response, key := registrationResponseWithKey(t, options, testOrigin)
	_, err = service.FinishPasskeyRegistration(t.Context(), token, "Laptop", response)
	require.NoError(t, err)
	return service, userID, key
}

func freshIP() string {
	b := make([]byte, 6)
	_, _ = rand.Read(b)
	return "ip-" + base64.RawURLEncoding.EncodeToString(b)
}

func signInOnce(
	t *testing.T,
	service *auth.LocalService,
	ip string,
	key *softKey,
	origin, userID string,
	signCount uint32,
) (*string, *string, error) {
	t.Helper()
	options, err := service.BeginPasskeySignIn(t.Context())
	require.NoError(t, err)
	return service.FinishPasskeySignIn(
		t.Context(), ip, key.assertionResponse(t, options, origin, userID, signCount),
	)
}

func TestPasskeySignIn_IssuesMFASession(t *testing.T) {
	service, userID, key := passkeyAccount(t)

	access, refresh, err := signInOnce(t, service, freshIP(), key, testOrigin, userID, 0)
	require.NoError(t, err)
	require.NotNil(t, refresh)

	// aal2: no TOTP step is demanded of the session.
	user, err := service.GetUser(t.Context(), *access)
	require.NoError(t, err)
	assert.Equal(t, userID, user.ID)

	list, err := service.ListPasskeys(t.Context(), *access)
	require.NoError(t, err)
	require.Len(t, list, 1)
	assert.NotNil(t, list[0].LastUsedAt)
}

func TestPasskeySignIn_OptionsAreDiscoverable(t *testing.T) {
	service, _ := newTestService(t)
	options, err := service.BeginPasskeySignIn(t.Context())
	require.NoError(t, err)
	var parsed struct {
		RPID             string            `json:"rpId"`
		AllowCredentials []json.RawMessage `json:"allowCredentials"`
	}
	require.NoError(t, json.Unmarshal(options, &parsed))
	assert.Equal(t, testRPID, parsed.RPID)
	assert.Empty(t, parsed.AllowCredentials)
}

func TestPasskeySignIn_WrongOriginRefused(t *testing.T) {
	service, userID, key := passkeyAccount(t)

	_, _, err := signInOnce(
		t, service, freshIP(), key, "https://evil.example", userID, 0,
	)
	require.Error(t, err)
}

func TestPasskeySignIn_ZeroCounterAlwaysAccepted(t *testing.T) {
	service, userID, key := passkeyAccount(t)
	ip := freshIP()

	for range 2 {
		_, _, err := signInOnce(t, service, ip, key, testOrigin, userID, 0)
		require.NoError(t, err)
	}
}

func TestPasskeySignIn_ClonedCounterRefused(t *testing.T) {
	service, userID, key := passkeyAccount(t)
	ip := freshIP()

	_, _, err := signInOnce(t, service, ip, key, testOrigin, userID, 5)
	require.NoError(t, err)
	_, _, err = signInOnce(t, service, ip, key, testOrigin, userID, 5)
	require.Error(t, err)
	_, _, err = signInOnce(t, service, ip, key, testOrigin, userID, 3)
	require.Error(t, err)
	_, _, err = signInOnce(t, service, ip, key, testOrigin, userID, 6)
	require.NoError(t, err)
}

func TestPasskeySignIn_ChallengeIsSingleUse(t *testing.T) {
	service, userID, key := passkeyAccount(t)
	options, err := service.BeginPasskeySignIn(t.Context())
	require.NoError(t, err)
	response := key.assertionResponse(t, options, testOrigin, userID, 0)

	_, _, err = service.FinishPasskeySignIn(t.Context(), freshIP(), response)
	require.NoError(t, err)
	_, _, err = service.FinishPasskeySignIn(t.Context(), freshIP(), response)
	require.Error(t, err)
}

func TestPasskeySignIn_UnknownCredentialRefused(t *testing.T) {
	service, userID, _ := passkeyAccount(t)
	_, _, other := passkeyAccount(t)

	_, _, err := signInOnce(t, service, freshIP(), other, testOrigin, userID, 0)
	require.Error(t, err)
}

func TestPasskeySignIn_GarbageRefused(t *testing.T) {
	service, _ := newTestService(t)
	_, _, err := service.FinishPasskeySignIn(t.Context(), freshIP(), []byte("{}"))
	require.Error(t, err)
}

func TestPasskeySignIn_LockoutPerIP(t *testing.T) {
	service, userID, key := passkeyAccount(t)
	ip := freshIP()

	// Garbage never resolves an account, so only the IP key counts.
	// Five failures are free; the sixth starts the lock.
	for range 6 {
		_, _, err := service.FinishPasskeySignIn(t.Context(), ip, []byte("{}"))
		require.Error(t, err)
		require.NotErrorIs(t, err, auth.ErrTooManyAttempts)
	}
	_, _, err := signInOnce(t, service, ip, key, testOrigin, userID, 0)
	require.ErrorIs(t, err, auth.ErrTooManyAttempts)

	// Another address is unaffected: the account itself was never charged.
	_, _, err = signInOnce(t, service, freshIP(), key, testOrigin, userID, 0)
	require.NoError(t, err)
}

func TestPasskeySignIn_LockoutPerAccount(t *testing.T) {
	service, userID, key := passkeyAccount(t)

	// Five failures are free; the sixth starts the lock.
	for range 6 {
		_, _, err := signInOnce(
			t, service, freshIP(), key, "https://evil.example", userID, 0,
		)
		require.Error(t, err)
		require.NotErrorIs(t, err, auth.ErrTooManyAttempts)
	}
	// A valid assertion from a fresh address still hits the account's lock.
	_, _, err := signInOnce(t, service, freshIP(), key, testOrigin, userID, 0)
	require.ErrorIs(t, err, auth.ErrTooManyAttempts)
}

func TestPasskeySignIn_DeactivatedAccountRefused(t *testing.T) {
	service, userID, key := passkeyAccount(t)
	require.NoError(t, service.DeactivateUser(t.Context(), userID))

	_, _, err := signInOnce(t, service, freshIP(), key, testOrigin, userID, 0)
	require.Error(t, err)
}
