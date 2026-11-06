package main

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"tools.xdoubleu.com/factory-runner/internal/runner"
)

// countingSecrets counts reads.
type countingSecrets struct {
	value string
	err   error
	reads int
}

func (c *countingSecrets) GetSecret(string) (string, error) {
	c.reads++
	return c.value, c.err
}

func (c *countingSecrets) SetSecret(string, string) error { return nil }

func TestTheOpenRouterKeyIsReadOnceNotOnEveryModelCall(t *testing.T) {
	s := &countingSecrets{value: "key-1", err: nil, reads: 0}
	get := cachedKey(s, runner.SecretOpenRouter)

	for range 5 {
		got, err := get()
		require.NoError(t, err)
		assert.Equal(t, "key-1", got)
	}
	assert.Equal(t, 1, s.reads)
}

func TestAKeyThatCouldNotBeReadIsTriedAgainOnTheNextRequest(t *testing.T) {
	s := &countingSecrets{value: "", err: errors.New("keychain locked"), reads: 0}
	get := cachedKey(s, runner.SecretOpenRouter)

	_, err := get()
	require.Error(t, err)

	s.value, s.err = "key-2", nil
	got, err := get()
	require.NoError(t, err)
	assert.Equal(t, "key-2", got, "stored mid-job, picked up")
	assert.Equal(t, 2, s.reads)
}

func TestSecretOnATerminalWarnsThatItIsEchoedAndSuggestsAPipe(t *testing.T) {
	e, secrets := withSecretInput(newEnv(t), "pasted-value\n")
	e.app.interactive = true

	require.Equal(t, 0, e.do("config", "secret", "claude"))

	assert.Contains(t, e.errs.String(), "Ctrl-D")
	assert.Contains(t, e.errs.String(), "pbpaste | factory-runner config secret claude")
	assert.NotContains(t, e.errs.String(), "pasted-value")
	assert.Equal(t, "pasted-value", secrets.values["claude"])
}

func TestSecretFromAPipeSaysNothingExtra(t *testing.T) {
	e, _ := withSecretInput(newEnv(t), "v\n")
	require.Equal(t, 0, e.do("config", "secret", "claude"))
	assert.Empty(t, e.errs.String())
}
