package ward_test

import (
	"errors"
	"os"
	"path/filepath"
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
	// A stand-in `security` on PATH, so the test runs off macOS too; it reports the
	// item missing the way the real one does.
	dir := t.TempDir()
	stub := "#!/bin/sh\nexit 44\n"
	require.NoError(t, os.WriteFile(filepath.Join(dir, "security"), []byte(stub), 0o700))
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
	k := ward.NewKeychain()
	_, code, err := k.Run("find-generic-password", "-s", "wardcase-test-none", "-a", "x")
	require.NoError(t, err)
	assert.NotEqual(t, 0, code)
}

func TestASecretIsDeletedAndAMissingOneIsNotAnError(t *testing.T) {
	var calls [][]string
	require.NoError(t, keychain("", 0, nil, &calls).Delete("mcp-x"))
	assert.Equal(t, []string{"delete-generic-password", "-s", "com.xdoubleu.wardcase",
		"-a", "mcp-x"}, calls[0])
	require.NoError(t, keychain("", 44, nil, &calls).Delete("mcp-x"))
	require.Error(t, keychain("", 1, nil, &calls).Delete("mcp-x"))
	require.Error(t, keychain("", 0, errors.New("x"), &calls).Delete("mcp-x"))
}
