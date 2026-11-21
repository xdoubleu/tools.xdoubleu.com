package ward_test

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"tools.xdoubleu.com/wardcase/internal/ward"
)

func keychain(out string, code int, err error, calls *[][]string) *ward.Keychain {
	return &ward.Keychain{Run: func(args ...string) ([]byte, int, error) {
		*calls = append(*calls, args)
		return []byte(out), code, err
	}}
}

func TestASecretIsReadFromWardcasesOwnKeychainItem(t *testing.T) {
	var calls [][]string
	v, err := keychain("tok\n", 0, nil, &calls).Get("anthropic")
	require.NoError(t, err)
	assert.Equal(t, "tok", v)
	assert.Equal(t, []string{"find-generic-password", "-s", "com.xdoubleu.wardcase",
		"-a", "anthropic", "-w"}, calls[0])
}

func TestAMissingSecretSaysHowToSetIt(t *testing.T) {
	var calls [][]string
	_, err := keychain("", 44, nil, &calls).Get("openrouter")
	require.ErrorIs(t, err, ward.ErrNoSecret)
	assert.Contains(t, err.Error(), "wardcase secret set openrouter")
}

func TestKeychainFailuresAreErrors(t *testing.T) {
	var calls [][]string
	_, err := keychain("", 1, nil, &calls).Get("anthropic")
	require.Error(t, err)
	_, err = keychain("", 0, errors.New("no security"), &calls).Get("anthropic")
	require.Error(t, err)
	require.Error(t, keychain("", 1, nil, &calls).Set("anthropic", "v"))
	require.Error(t, keychain("", 0, errors.New("x"), &calls).Set("anthropic", "v"))
}

func TestASecretIsStoredReplacingAnOldOne(t *testing.T) {
	var calls [][]string
	require.NoError(t, keychain("", 0, nil, &calls).Set("openrouter", "k"))
	assert.Equal(t, []string{"add-generic-password", "-U", "-s", "com.xdoubleu.wardcase",
		"-a", "openrouter", "-w", "k"}, calls[0])
}

func TestOnlyTheTwoModelSecretsAreKnown(t *testing.T) {
	assert.True(t, ward.ValidSecret("anthropic"))
	assert.True(t, ward.ValidSecret("openrouter"))
	assert.True(t, ward.ValidSecret("openai"))
	assert.False(t, ward.ValidSecret("github"))
}

func TestTheRealKeychainRunsSecurity(t *testing.T) {
	k := ward.NewKeychain()
	// A lookup of an item that never exists: `security` runs and reports it missing.
	_, code, err := k.Run("find-generic-password", "-s", "wardcase-test-none", "-a", "x")
	require.NoError(t, err)
	assert.NotEqual(t, 0, code)
}
