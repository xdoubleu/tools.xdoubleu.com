package runner_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"tools.xdoubleu.com/factory-runner/internal/runner"
)

func TestCommitsCarryTheBotAddressOnceConfiguredAndADefaultBefore(t *testing.T) {
	c := emptyConfig()
	assert.Equal(t, runner.DefaultBotEmail, c.AuthorEmail())

	const bot = "123456+factory[bot]@users.noreply.github.com"
	require.NoError(t, c.SetBotEmail(bot))
	assert.Equal(t, bot, c.AuthorEmail())
}

func TestAnAddressThatCouldBendACommitHeaderIsRefused(t *testing.T) {
	c := emptyConfig()
	for _, email := range []string{
		"", "plain", "a b@c.d", "a@b\nBcc: x@y.z", "<x@y.z>", "x@y.z>", "a@", "@b.c",
	} {
		require.Error(t, c.SetBotEmail(email), email)
	}
	assert.Empty(t, c.BotEmail)
}
