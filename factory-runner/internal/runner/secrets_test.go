package runner_test

import (
	"context"
	"errors"
	"io"
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"tools.xdoubleu.com/factory-runner/internal/runner"
)

func TestSecretsAreStoredUnderTheirOwnKeychainAccounts(t *testing.T) {
	var calls [][]string
	k := runner.NewKeychainWith(func(args ...string) ([]byte, int, error) {
		calls = append(calls, args)
		if args[0] == "find-generic-password" {
			return []byte("  value-1\n"), 0, nil
		}
		return nil, 0, nil
	})

	require.NoError(t, k.SetSecret(runner.SecretClaude, "tok"))
	require.NoError(t, k.SetSecret(runner.SecretOpenRouter, "key"))
	got, err := k.GetSecret(runner.SecretClaude)
	require.NoError(t, err)

	assert.Equal(t, "value-1", got, "whitespace around the stored value is dropped")
	assert.Contains(t, strings.Join(calls[0], " "), "-a secret-claude")
	assert.Contains(t, strings.Join(calls[1], " "), "-a secret-openrouter")
	assert.NotContains(t, strings.Join(calls[0], " "), "device-token")
}

func TestAMissingSecretNamesTheCommandThatStoresIt(t *testing.T) {
	k := runner.NewKeychainWith(func(...string) ([]byte, int, error) {
		return nil, 44, errors.New("not found")
	})
	_, err := k.GetSecret(runner.SecretOpenRouter)
	require.ErrorIs(t, err, runner.ErrNoSecret)
	assert.Contains(t, err.Error(), "factory-runner config secret openrouter")
}

func TestEmptyAndBrokenKeychainAnswersAreErrors(t *testing.T) {
	empty := runner.NewKeychainWith(func(...string) ([]byte, int, error) {
		return []byte("\n"), 0, nil
	})
	_, err := empty.GetSecret(runner.SecretClaude)
	require.ErrorIs(t, err, runner.ErrNoSecret)

	broken := runner.NewKeychainWith(func(...string) ([]byte, int, error) {
		return nil, 1, errors.New("keychain locked")
	})
	_, err = broken.GetSecret(runner.SecretClaude)
	require.ErrorContains(t, err, "read keychain")
	require.ErrorContains(t, broken.SetSecret(runner.SecretClaude, "x"), "write keychain")
}

func TestOnlyTheTwoKnownSecretsAndOneLineValuesAreAccepted(t *testing.T) {
	var ran bool
	k := runner.NewKeychainWith(func(...string) ([]byte, int, error) {
		ran = true
		return nil, 0, nil
	})
	_, err := k.GetSecret("device-token")
	require.ErrorContains(t, err, "unknown secret")
	require.ErrorContains(t, k.SetSecret("github", "x"), "unknown secret")
	require.Error(t, k.SetSecret(runner.SecretClaude, ""))
	require.Error(t, k.SetSecret(runner.SecretClaude, "a\nb"))
	assert.False(t, ran, "nothing reached the keychain")
}

func TestAJobProxyReplacesTheSandboxsOwnAndTheEnvFileIsInAStableOrder(t *testing.T) {
	var envBody string
	exec := newExecScript(routesAnswer)
	exec.onExec = func(argv []string, _ string) {
		for i, a := range argv {
			if a == "--env-file" {
				body, err := os.ReadFile(argv[i+1])
				require.NoError(t, err)
				envBody = string(body)
			}
		}
	}
	var sandboxGateway, jobGateway string
	box := sandboxWith(t, exec)
	box.ProxyPort = 8118
	box.Proxy = &fakeProxy{
		gateway: &sandboxGateway, err: nil, port: nil, onStop: func() {},
	}
	s := spec(t, io.Discard)
	s.Proxy = &fakeProxy{gateway: &jobGateway, err: nil, port: nil, onStop: func() {}}

	_, err := box.Run(context.Background(), s)
	require.NoError(t, err)

	assert.Equal(t, "10.231.7.1", jobGateway)
	assert.Empty(t, sandboxGateway, "the job's proxy is used, not the sandbox's")
	assert.Equal(t, strings.Join([]string{
		"GOMODCACHE=/tmp/gomodcache",
		"GOPROXY=file:///opt/gomodcache/cache/download,http://10.231.7.1:8118/goproxy/",
		"HOME=/tmp/home", "npm_config_cache=/tmp/npmcache",
		"npm_config_prefer_offline=true",
		"npm_config_registry=http://10.231.7.1:8118/npm/",
		"FACTORY_GATEWAY=10.231.7.1", "FACTORY_PROXY=http://10.231.7.1:8118",
		"FACTORY_JOB=0a1b2c3d-4e5f", "SECRET=s3cret", "",
	}, "\n"), envBody)
}

func TestTheSandboxOwnsTheVariablesThatSayWhereItsProxyIs(t *testing.T) {
	for _, key := range []string{"FACTORY_GATEWAY", "FACTORY_PROXY"} {
		exec := newExecScript(routesAnswer)
		s := spec(t, io.Discard)
		s.Env[key] = "http://evil.example"

		_, err := sandboxWith(t, exec).Run(context.Background(), s)

		require.ErrorContains(t, err, "set by the sandbox", key)
		assert.Empty(t, exec.calls, key)
	}
}
