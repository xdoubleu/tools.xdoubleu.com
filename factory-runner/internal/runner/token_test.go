package runner_test

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"tools.xdoubleu.com/factory-runner/internal/runner"
)

func TestKeychainGet(t *testing.T) {
	var gotArgs []string
	k := runner.NewKeychainWith(func(args ...string) ([]byte, int, error) {
		gotArgs = args
		return []byte("frt_abc\n"), 0, nil
	})
	token, err := k.Get()
	require.NoError(t, err)
	assert.Equal(t, "frt_abc", token)
	assert.Equal(t, "find-generic-password", gotArgs[0])
	assert.Contains(t, gotArgs, "-w")
}

func TestKeychainGetWithoutAnItemIsNotPaired(t *testing.T) {
	missing := runner.NewKeychainWith(func(...string) ([]byte, int, error) {
		return nil, 44, errors.New("exit 44")
	})
	_, err := missing.Get()
	require.ErrorIs(t, err, runner.ErrNoToken)

	empty := runner.NewKeychainWith(func(...string) ([]byte, int, error) {
		return []byte("\n"), 0, nil
	})
	_, err = empty.Get()
	require.ErrorIs(t, err, runner.ErrNoToken)
}

func TestKeychainGetFailure(t *testing.T) {
	k := runner.NewKeychainWith(func(...string) ([]byte, int, error) {
		return nil, 1, errors.New("locked")
	})
	_, err := k.Get()
	require.Error(t, err)
	require.NotErrorIs(t, err, runner.ErrNoToken)
}

func TestKeychainSet(t *testing.T) {
	var gotArgs []string
	k := runner.NewKeychainWith(func(args ...string) ([]byte, int, error) {
		gotArgs = args
		return nil, 0, nil
	})
	require.NoError(t, k.Set("frt_abc"))
	assert.Equal(t, "add-generic-password", gotArgs[0])
	assert.Contains(t, gotArgs, "-U")
	assert.Equal(t, "frt_abc", gotArgs[len(gotArgs)-1])

	failing := runner.NewKeychainWith(func(...string) ([]byte, int, error) {
		return nil, 1, errors.New("denied")
	})
	require.Error(t, failing.Set("x"))
}

func TestNewKeychainRunsSecurity(t *testing.T) {
	// `security` exists only on macOS; elsewhere the call must fail cleanly.
	_, err := runner.NewKeychain().Get()
	if err != nil {
		assert.Error(t, err)
	}
}
