package runner_test

import (
	"context"
	"os/exec"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"tools.xdoubleu.com/factory-runner/internal/runner"
)

const (
	containerfile = "factory-runner/Containerfile"
	flakeNix      = "flake.nix"
)

func TestOnlyTheFlakeFilesMayChange(t *testing.T) {
	assert.Error(t, runner.FlakeProblemForTest(nil))
	assert.NoError(t, runner.FlakeProblemForTest([]string{"flake.nix"}))
	assert.NoError(t, runner.FlakeProblemForTest([]string{"flake.lock", "flake.nix"}))
	assert.Error(t, runner.FlakeProblemForTest([]string{"flake.nix", "api/main.go"}))
	assert.Error(t, runner.FlakeProblemForTest([]string{containerfile}))
	assert.Error(t, runner.FlakeProblemForTest([]string{"sub/flake.nix"}))
}

// sandboxJob has the agent commit its work, then a second branch off main.
func sandboxJob(t *testing.T, files map[string]string, tail string) *publishing {
	t.Helper()
	return newPublishing(t, agentDoes(
		"Fix it\n\nok"+allPass+"\n"+tail,
		func(clone string) {
			commitFile(clone, "a.txt", "work")
			run := func(args ...string) {
				cmd := exec.Command("git", args...)
				cmd.Dir = clone
				_ = cmd.Run()
			}
			run("checkout", "-b", "agent/sandbox-ruby", "main")
			for name, content := range files {
				commitFile(clone, name, content)
			}
			run("checkout", branch)
		}))
}

func TestAFlakeFixGetsItsOwnPullRequest(t *testing.T) {
	p := sandboxJob(t, map[string]string{
		flakeNix: "{ outputs = _: { }; }\n",
	}, "Sandbox-fix: agent/sandbox-ruby\nBlocked: no ruby")

	_, err := p.runner.Next(context.Background())
	require.NoError(t, err)

	got := p.api.done()[0]
	assert.True(t, got.succeeded)
	require.Len(t, p.gh.opened, 2)
	fix := p.gh.opened[1]
	assert.Equal(t, "agent/sandbox-ruby", fix.Head)
	assert.Equal(t, "main", fix.Base)
	assert.True(t, strings.HasPrefix(fix.Title, "Sandbox: "), fix.Title)
	assert.Contains(t, fix.Body, taskID)
	assert.Contains(t, strings.Join(p.api.comments, "\n"), "Sandbox fix opened")
}

func TestASandboxFixTouchingAnythingElseIsRefusedButTheJobStillPublishes(t *testing.T) {
	p := sandboxJob(t, map[string]string{
		flakeNix:      "{ }\n",
		"api/main.go": "package main",
	}, "Sandbox-fix: agent/sandbox-ruby")

	_, err := p.runner.Next(context.Background())
	require.NoError(t, err)

	got := p.api.done()[0]
	assert.True(t, got.succeeded)
	assert.Len(t, p.gh.opened, 1, "only the job's own pull request")
	assert.Contains(t, strings.Join(p.api.comments, "\n"), "Sandbox fix not published")
}

func TestAContainerfileSandboxFixIsRefused(t *testing.T) {
	p := sandboxJob(t, map[string]string{
		containerfile: "RUN apt-get install -y ruby=1:3.3.0\n",
	}, "Sandbox-fix: agent/sandbox-ruby")

	_, err := p.runner.Next(context.Background())
	require.NoError(t, err)
	assert.Len(t, p.gh.opened, 1)
	assert.Contains(t, strings.Join(p.api.comments, "\n"), "Sandbox fix not published")
}

func TestASandboxFixBranchWithAnotherNameIsRefused(t *testing.T) {
	p := newPublishing(t, agentDoes("Fix it\n\nok"+allPass+"\nSandbox-fix: main",
		func(clone string) { commitFile(clone, "a.txt", "work") }))
	_, err := p.runner.Next(context.Background())
	require.NoError(t, err)
	assert.Len(t, p.gh.opened, 1)
	assert.Contains(t, strings.Join(p.api.comments, "\n"), "Sandbox fix not published")
}

// fixOnly has the agent commit only on the sandbox branch, nothing on the job's.
func fixOnly(t *testing.T, file, content string) *publishing {
	t.Helper()
	return newPublishing(t, agentDoes(
		"Sandbox: ruby\n\nok"+allPass+"\nSandbox-fix: agent/sandbox-ruby\nBlocked: no ruby",
		func(clone string) {
			if exec.Command("git", "-C", clone, "rev-parse", "--verify", "-q",
				"agent/sandbox-ruby").Run() == nil {
				return // a retry: the agent leaves its refused branch as it was
			}
			run := func(args ...string) {
				cmd := exec.Command("git", args...)
				cmd.Dir = clone
				_ = cmd.Run()
			}
			run("checkout", "-b", "agent/sandbox-ruby", "main")
			commitFile(clone, file, content)
			run("checkout", branch)
		}))
}

func TestAValidFixWithNoJobBranchCommitsPublishesTheFix(t *testing.T) {
	p := fixOnly(t, flakeNix, "{ }\n")
	_, err := p.runner.Next(context.Background())
	require.NoError(t, err)

	got := p.api.done()[0]
	assert.True(t, got.succeeded)
	require.Len(t, p.gh.opened, 1)
	assert.Equal(t, "agent/sandbox-ruby", p.gh.opened[0].Head)
}

func TestARefusedFixAloneIsReportedAndNotRetriedUnchanged(t *testing.T) {
	p := fixOnly(t, containerfile, "FROM ubuntu:24.04\n")
	_, err := p.runner.Next(context.Background())
	require.NoError(t, err)

	got := p.api.done()[0]
	assert.False(t, got.succeeded)
	assert.Empty(t, p.gh.opened)
	assert.Contains(t, got.outcome, "it must change only flake.nix and flake.lock")
}

func TestAFixWhosePullRequestCannotBeOpenedIsReportedAsSuch(t *testing.T) {
	p := fixOnly(t, flakeNix, "{ }\n")
	p.gh.err, p.gh.failFirst = assert.AnError, 100
	_, err := p.runner.Next(context.Background())
	require.NoError(t, err)
	assert.Contains(t, strings.Join(p.api.comments, "\n"), "could not be opened")
}

func TestAFixWithoutAPushTokenIsNotPublished(t *testing.T) {
	p := fixOnly(t, flakeNix, "{ }\n")
	p.api.pushToken = ""
	_, err := p.runner.Next(context.Background())
	require.NoError(t, err)
	assert.Empty(t, p.gh.opened)
}
