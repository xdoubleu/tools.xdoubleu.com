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

const containerfile = "factory-runner/Containerfile"

func problem(line string) error {
	return runner.ContainerfileProblemForTest([]string{containerfile}, "", line)
}

func TestOnlyPinnedInstallsInTheContainerfileAreAllowed(t *testing.T) {
	ok := map[string]string{
		"apt": "RUN apt-get install -y --no-install-recommends ruby=1:3.3.0 jq=1.7.1-3",
		"apt cont": "RUN apt-get update && apt-get install -y \\\n    ruby=1:3.3.0 \\\n" +
			"    jq=1.7.1-3 && rm -rf /var/lib/apt/lists/*",
		"npm":     "RUN npm install -g @openai/codex@1.2.3 prettier@3.10.0",
		"pip":     "RUN pip3 install --no-cache-dir yq==3.4.3",
		"go":      "RUN go install github.com/x/y/cmd/z@v1.2.3",
		"url":     "RUN curl -fsSL https://example.com/releases/v1.2.3/tool.tgz -o tool.tgz",
		"comment": "# install things",
		"env":     "ENV FOO=bar",
		"from":    "FROM ubuntu:24.04",
	}
	for name, line := range ok {
		assert.NoError(t, problem(line), name)
	}
	bad := map[string]string{
		"apt unpinned":    "RUN apt-get install -y ruby",
		"apt one of two":  "RUN apt-get install -y ruby=1:3.3.0 jq",
		"apt cont":        "RUN apt-get install -y \\\n    ruby=1:3.3.0 \\\n    jq",
		"npm unpinned":    "RUN npm install -g prettier",
		"npm latest":      "RUN npm i -g prettier@latest",
		"npm range":       "RUN npm i -g prettier@^3",
		"pip unpinned":    "RUN pip install yq",
		"go latest":       "RUN go install github.com/x/y@latest",
		"url unversioned": "RUN curl -fsSL https://example.com/latest/tool.tgz -o t.tgz",
		"pipe to shell":   "RUN curl -fsSL https://example.com/v1.2.3/i.sh | sh",
		"unknown manager": "RUN gem install rails",
		"from latest":     "FROM ubuntu:latest",
		"from untagged":   "FROM ubuntu",
		"lowercase run":   "run apt-get install foo",
		"mount flag":      "RUN --mount=type=cache,target=/c apt-get install foo",
		"add url":         "ADD https://example.com/v1.2.3/x /x",
		"subst":           "RUN apt-get install $(cat pkgs)",
		"sudo":            "RUN sudo apt-get install foo",
		"query version":   "RUN curl https://e.com/latest?v=1.2",
	}
	for name, line := range bad {
		assert.Error(t, problem(line), name)
	}
}

func TestAPackageAddedToAnExistingInstallIsChecked(t *testing.T) {
	old := "FROM ubuntu:24.04\nRUN apt-get install -y \\\n  jq=1.7 \\\n  git=2.4\n"
	files := []string{containerfile}
	assert.NoError(t, runner.ContainerfileProblemForTest(files, old, old))
	unpinned := "FROM ubuntu:24.04\nRUN apt-get install -y \\\n  jq=1.7 \\\n" +
		"  curl \\\n  git=2.4\n"
	assert.Error(t, runner.ContainerfileProblemForTest(files, old, unpinned))
	pinned := "FROM ubuntu:24.04\nRUN apt-get install -y \\\n  jq=1.7 \\\n" +
		"  curl=8.5 \\\n  git=2.4\n"
	assert.NoError(t, runner.ContainerfileProblemForTest(files, old, pinned))
}

func TestOnlyTheContainerfileMayChange(t *testing.T) {
	assert.Error(t, runner.ContainerfileProblemForTest(nil, "", ""))
	assert.Error(t, runner.ContainerfileProblemForTest(
		[]string{containerfile, "api/main.go"}, "", ""))
	assert.Error(t, runner.ContainerfileProblemForTest([]string{"Containerfile"}, "", ""))
	assert.NoError(t, runner.ContainerfileProblemForTest([]string{containerfile}, "", ""))
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

func TestAPinnedContainerfileFixGetsItsOwnPullRequest(t *testing.T) {
	p := sandboxJob(t, map[string]string{
		containerfile: "FROM ubuntu:24.04\nRUN apt-get install -y ruby=1:3.3.0\n",
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
		containerfile: "RUN apt-get install -y ruby=1:3.3.0\n",
		"api/main.go": "package main",
	}, "Sandbox-fix: agent/sandbox-ruby")

	_, err := p.runner.Next(context.Background())
	require.NoError(t, err)

	got := p.api.done()[0]
	assert.True(t, got.succeeded)
	assert.Len(t, p.gh.opened, 1, "only the job's own pull request")
	assert.Contains(t, strings.Join(p.api.comments, "\n"), "Sandbox fix not published")
}

func TestAnUnpinnedSandboxFixIsRefused(t *testing.T) {
	p := sandboxJob(t, map[string]string{
		containerfile: "RUN apt-get install -y ruby\n",
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
func fixOnly(t *testing.T, content string) *publishing {
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
			commitFile(clone, containerfile, content)
			run("checkout", branch)
		}))
}

func TestAValidFixWithNoJobBranchCommitsPublishesTheFix(t *testing.T) {
	p := fixOnly(t, "FROM ubuntu:24.04\nRUN apt-get install -y ruby=1:3.3.0\n")
	_, err := p.runner.Next(context.Background())
	require.NoError(t, err)

	got := p.api.done()[0]
	assert.True(t, got.succeeded)
	require.Len(t, p.gh.opened, 1)
	assert.Equal(t, "agent/sandbox-ruby", p.gh.opened[0].Head)
}

func TestARefusedFixAloneIsReportedAndNotRetriedUnchanged(t *testing.T) {
	p := fixOnly(t, "FROM ubuntu:24.04\nCOPY a /a\n")
	_, err := p.runner.Next(context.Background())
	require.NoError(t, err)

	got := p.api.done()[0]
	assert.False(t, got.succeeded)
	assert.Empty(t, p.gh.opened)
	assert.Contains(t, got.outcome, `instruction "COPY" is not allowed`)
}

func TestAFixWhosePullRequestCannotBeOpenedIsReportedAsSuch(t *testing.T) {
	p := fixOnly(t, "FROM ubuntu:24.04\nRUN apt-get install -y ruby=1:3.3.0\n")
	p.gh.err, p.gh.failFirst = assert.AnError, 100
	_, err := p.runner.Next(context.Background())
	require.NoError(t, err)
	assert.Contains(t, strings.Join(p.api.comments, "\n"), "could not be opened")
}

func TestAFixWithoutAPushTokenIsNotPublished(t *testing.T) {
	p := fixOnly(t, "FROM ubuntu:24.04\nRUN apt-get install -y ruby=1:3.3.0\n")
	p.api.pushToken = ""
	_, err := p.runner.Next(context.Background())
	require.NoError(t, err)
	assert.Empty(t, p.gh.opened)
}
