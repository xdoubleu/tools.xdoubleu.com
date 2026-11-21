package vm_test

import (
	"bytes"
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"tools.xdoubleu.com/wardcase/vm"
)

// runOnly is a Sandbox without sessions; it records each run's spec.
type runOnly struct{ runs []vm.SandboxSpec }

func (r *runOnly) Run(_ context.Context, spec vm.SandboxSpec) (int, error) {
	r.runs = append(r.runs, spec)
	return len(r.runs), nil
}

func TestASandboxWithoutSessionsGetsAFreshRunPerExec(t *testing.T) {
	box := &runOnly{runs: nil}
	base := vm.SandboxSpec{
		CloneDir: "/c", OutcomeDir: "", SessionDir: "", Env: nil, Command: nil,
		Proxy: nil, Stdin: "", Log: nil, Mounts: nil, SkipInstall: true,
	}
	s, err := vm.OpenSession(context.Background(), box, base)
	require.NoError(t, err)
	defer s.Close()

	var log bytes.Buffer
	code, err := s.Exec(context.Background(), []string{"a"}, "in1", &log)
	require.NoError(t, err)
	assert.Equal(t, 1, code)
	code, err = s.Exec(context.Background(), []string{"b"}, "in2", nil)
	require.NoError(t, err)
	assert.Equal(t, 2, code)

	require.Len(t, box.runs, 2)
	assert.Equal(t, []string{"a"}, box.runs[0].Command)
	assert.Equal(t, "in1", box.runs[0].Stdin)
	assert.Same(t, &log, box.runs[0].Log)
	assert.Equal(t, "/c", box.runs[1].CloneDir)
	assert.True(t, box.runs[1].SkipInstall)
}
