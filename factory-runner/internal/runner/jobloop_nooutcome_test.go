package runner_test

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"tools.xdoubleu.com/factory-runner/internal/runner"
)

func TestAnAgentThatForgotTheOutcomeFileIsAskedOnceInTheSameSession(t *testing.T) {
	var stdins []string
	p := newPublishing(t, func(_ context.Context, spec runner.SandboxSpec) (int, error) {
		stdins = append(stdins, spec.Stdin)
		_, _ = fmt.Fprintln(spec.Log, `{"type":"system","session_id":"sess-9"}`)
		if len(stdins) == 1 {
			commitFile(spec.CloneDir, "agent.txt", "work")
			return 0, nil
		}
		_ = os.WriteFile(filepath.Join(spec.OutcomeDir, "outcome.md"),
			[]byte("Done"+passAll), 0o644)
		return 0, nil
	})

	_, err := p.runner.Next(context.Background())
	require.NoError(t, err)

	require.Len(t, stdins, 2)
	assert.Contains(t, stdins[1], "outcome file")
	assert.NotContains(t, p.api.done()[0].outcome, "wrote no outcome file")
}
