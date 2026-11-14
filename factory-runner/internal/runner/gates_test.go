package runner_test

import (
	"context"
	"fmt"
	"os/exec"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"tools.xdoubleu.com/factory-runner/internal/runner"
)

const gatesText = "# c\napi/ | make api\nweb/ | make web\n\n* | make docs\nbad line\n"

func TestGatesAreScopedByChangedPaths(t *testing.T) {
	assert.Len(t, runner.ParseGatesForTest(gatesText), 3)
	assert.Equal(t, []string{"make api", "make docs"},
		runner.GatesForTest(gatesText, []string{"api/a.go"}))
	assert.Equal(t, []string{"make docs"}, runner.GatesForTest(gatesText, nil))
}

func TestAFailingHostGateGoesBackToTheAgentBeforeAnyPush(t *testing.T) {
	agentN, gateN := 0, 0
	p := newPublishing(t, func(_ context.Context, spec runner.SandboxSpec) (int, error) {
		if len(spec.Command) > 1 && spec.Command[0] == "bash" {
			if spec.Command[1] == "-c" {
				_, _ = fmt.Fprintln(spec.Log, "api/a.go")
				return 0, nil
			}
			gateN++
			if gateN == 1 {
				_, _ = fmt.Fprintln(spec.Log, "lint boom")
				return 1, nil
			}
			return 0, nil
		}
		agentN++
		return agentRuns("Try"+passAll, true)(agentN, spec), nil
	})

	p.runner.Git.Run = func(
		ctx context.Context, dir string, args ...string,
	) ([]byte, error) {
		if len(args) > 0 && args[0] == "cat-file" {
			return []byte("api/ | make lint\n"), nil
		}
		cmd := exec.CommandContext(ctx, "git", args...)
		cmd.Dir = dir
		return cmd.CombinedOutput()
	}

	_, err := p.runner.Next(context.Background())
	require.NoError(t, err)

	require.Len(t, p.sandbox.specs, 6)
	assert.Contains(t, p.sandbox.specs[3].Stdin, "lint boom")
	assert.Contains(t, p.sandbox.specs[3].Stdin, "make lint")
}
