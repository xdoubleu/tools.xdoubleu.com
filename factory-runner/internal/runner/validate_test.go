package runner_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"tools.xdoubleu.com/factory-runner/internal/runner"
)

const repoOK = "xdoubleu/tools.xdoubleu.com"

func allowedRepos() []string { return []string{repoOK} }

func job(repo, branch, base string) runner.Job {
	return runner.Job{Repo: repo, Branch: branch, BaseBranch: base}
}

func TestValidateAcceptsAnAllowedAgentJob(t *testing.T) {
	require.NoError(t, runner.Validate(
		job("XDoubleu/Tools.xdoubleu.com", "agent/task-12", "main"), allowedRepos()))
	require.NoError(t, runner.Validate(
		job(repoOK, "agent/a/b.c", "release/1.0"), allowedRepos()))
}

func TestValidateRefuses(t *testing.T) {
	for name, j := range map[string]runner.Job{
		"repo outside the allowlist": job("evil/tools", "agent/x", "main"),
		"empty repo":                 job("", "agent/x", "main"),
		"repo without owner":         job("tools.xdoubleu.com", "agent/x", "main"),
		"repo with a url":            job("https://github.com/a/b", "agent/x", "main"),
		"repo starting with dash":    job("-a/b", "agent/x", "main"),
		"branch outside agent":       job(repoOK, "main", "main"),
		"branch prefix only":         job(repoOK, "agent/", "main"),
		"branch starting with dash":  job(repoOK, "--upload-pack=x", "main"),
		"branch with dotdot":         job(repoOK, "agent/../main", "main"),
		"branch with empty part":     job(repoOK, "agent//x", "main"),
		"branch with hidden part":    job(repoOK, "agent/.x", "main"),
		"branch ending in .lock":     job(repoOK, "agent/x.lock", "main"),
		"branch ending in dot":       job(repoOK, "agent/x.", "main"),
		"branch with a space":        job(repoOK, "agent/a b", "main"),
		"branch with a newline":      job(repoOK, "agent/a\nb", "main"),
		"base starting with dash":    job(repoOK, "agent/x", "-main"),
		"empty base":                 job(repoOK, "agent/x", ""),
	} {
		err := runner.Validate(j, allowedRepos())
		require.ErrorIs(t, err, runner.ErrRefused, name)
	}
}

func TestAnEmptyAllowlistRefusesEverything(t *testing.T) {
	err := runner.Validate(job(repoOK, "agent/x", "main"), nil)
	require.ErrorIs(t, err, runner.ErrRefused)
}

func TestValidRepo(t *testing.T) {
	assert.True(t, runner.ValidRepo("a/b-c.d_e"))
	assert.False(t, runner.ValidRepo("a/b/c"))
	assert.False(t, runner.ValidRepo("a/"))
}
