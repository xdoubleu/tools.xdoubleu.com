package main

import (
	"bytes"
	"context"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"tools.xdoubleu.com/wardcase/acp"
	"tools.xdoubleu.com/wardcase/acp/acptest"
	"tools.xdoubleu.com/wardcase/internal/ward"
	"tools.xdoubleu.com/wardcase/vm"
)

type agentBox struct{ agent *acptest.Agent }

func (agentBox) Run(context.Context, vm.SandboxSpec) (int, error) { return 0, nil }

func (b agentBox) Open(context.Context, vm.SandboxSpec) (vm.SandboxSession, error) {
	return agentSession{agent: b.agent}, nil
}

type agentSession struct{ agent *acptest.Agent }

func (agentSession) Exec(context.Context, []string, string, io.Writer) (int, error) {
	return 0, nil
}

func (s agentSession) Attach(
	ctx context.Context, _ []string, in io.Reader, out io.Writer, _ bool,
) (int, error) {
	s.agent.Serve(ctx, in, out)
	return 0, nil
}

func (agentSession) Close() {}

func checkout(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	for _, args := range [][]string{
		{"init", "-q", "-b", "main"},
		{"-c", "user.name=t", "-c", "user.email=t@t", "commit", "-q", "--allow-empty", "-m", "i"},
	} {
		out, err := exec.Command("git", append([]string{"-C", dir}, args...)...).CombinedOutput()
		require.NoError(t, err, string(out))
	}
	return dir
}

func acpRunner(t *testing.T, agent *acptest.Agent, notes io.Writer) *ward.Runner {
	t.Helper()
	return &ward.Runner{
		Box: agentBox{agent: agent}, Secrets: memSecrets{ward.SecretOpenRouter: "or"},
		Config: ward.Config{
			Order: []string{ward.OpenCode}, Overflow: ward.OverflowOff,
			OpenCode: ward.OpenCodeConfig{Model: "vendor/model"}, MCP: nil,
		},
		StateDir: t.TempDir(), TempDir: t.TempDir(), Git: ward.HostGit, GitEnv: nil, GH: nil,
		NewProxy: newProxy, In: nil, Out: io.Discard, Notes: notes, Terminal: false,
		Ask: nil, Now: nil,
	}
}

func TestWardcaseACPServesASandboxedSessionToAnADE(t *testing.T) {
	agent := acptest.New()
	var notes bytes.Buffer
	r := acpRunner(t, agent, &notes)

	adeToWardR, adeToWardW := io.Pipe()
	wardToAdeR, wardToAdeW := io.Pipe()
	served := make(chan error, 1)
	go func() { served <- serveACP(context.Background(), r, ward.OpenCode, adeToWardR, wardToAdeW) }()

	var mu sync.Mutex
	var updates []string
	ade := acp.NewClient(wardToAdeR, adeToWardW, func(e acp.Event) {
		mu.Lock()
		defer mu.Unlock()
		updates = append(updates, e.Kind+":"+e.Text())
	})
	go func() { _ = ade.Serve(context.Background()) }()

	ctx := context.Background()
	require.NoError(t, ade.Initialize(ctx))
	id, err := ade.NewSession(ctx, checkout(t))
	require.NoError(t, err)
	reason, err := ade.Prompt(ctx, id, "hi")
	require.NoError(t, err)
	assert.Equal(t, "end_turn", reason)

	mu.Lock()
	assert.Equal(t, []string{"update:working on hi", "update:done"}, updates,
		"permissions are answered in the VM, not forwarded")
	mu.Unlock()
	require.Len(t, agent.Outcomes(), 1)
	assert.Contains(t, string(agent.Outcomes()[0]), `"selected"`)

	require.NoError(t, adeToWardW.Close())
	require.NoError(t, <-served)
	assert.Contains(t, notes.String(), "allowed a tool call")
	assert.Contains(t, notes.String(), "nothing left")
}

func TestWardcaseACPFallsBackToItsOwnDirectoryWhenTheADENamesNone(t *testing.T) {
	agent := acptest.New()
	r := acpRunner(t, agent, io.Discard)
	t.Chdir(checkout(t))

	adeToWardR, adeToWardW := io.Pipe()
	wardToAdeR, wardToAdeW := io.Pipe()
	go func() { _ = serveACP(context.Background(), r, ward.OpenCode, adeToWardR, wardToAdeW) }()
	ade := acp.NewClient(wardToAdeR, adeToWardW, nil)
	go func() { _ = ade.Serve(context.Background()) }()
	defer adeToWardW.Close()

	require.NoError(t, ade.Initialize(context.Background()))
	_, err := ade.NewSession(context.Background(), "")
	require.NoError(t, err)
}

func TestWardcaseACPReportsASessionThatCannotStart(t *testing.T) {
	r := acpRunner(t, acptest.New(), io.Discard)
	r.Secrets = memSecrets{}
	adeToWardR, adeToWardW := io.Pipe()
	wardToAdeR, wardToAdeW := io.Pipe()
	go func() { _ = serveACP(context.Background(), r, ward.OpenCode, adeToWardR, wardToAdeW) }()
	ade := acp.NewClient(wardToAdeR, adeToWardW, nil)
	go func() { _ = ade.Serve(context.Background()) }()
	defer adeToWardW.Close()

	ctx := context.Background()
	require.NoError(t, ade.Initialize(ctx))
	_, err := ade.NewSession(ctx, checkout(t))
	assert.ErrorContains(t, err, "openrouter")
	_, err = ade.Prompt(ctx, "x", "hi")
	assert.Error(t, err)
}

func TestWardcaseACPStopsWhenTheContextEnds(t *testing.T) {
	r := acpRunner(t, acptest.New(), io.Discard)
	in, _ := io.Pipe()
	ctx, cancel := context.WithCancel(context.Background())
	served := make(chan error, 1)
	go func() { served <- serveACP(ctx, r, ward.OpenCode, in, io.Discard) }()
	cancel()
	_ = in.Close()
	<-served
}

func TestACPTakesOnlyAHarnessWithAnACPServer(t *testing.T) {
	var errOut bytes.Buffer
	for _, args := range [][]string{{"claude"}, {"opencode", "x"}, {"vim"}} {
		code := run(context.Background(), testHost(), append([]string{"acp"}, args...),
			emptyFile(t), io.Discard, &errOut)
		assert.Equal(t, exitUsage, code, args)
	}
	assert.Contains(t, errOut.String(), "usage: wardcase acp")
}

func TestACPWithoutTheContainerCLIFailsBeforeServing(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	var errOut bytes.Buffer
	code := run(context.Background(), testHost(), []string{"acp"}, emptyFile(t),
		io.Discard, &errOut)
	assert.Equal(t, 1, code)
	assert.Contains(t, errOut.String(), "container CLI")
}

func TestACPWithABadConfigFails(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	dir := filepath.Join(home, ".config", "wardcase")
	require.NoError(t, os.MkdirAll(dir, 0o700))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "config.toml"),
		[]byte(`overflow = "x"`), 0o600))
	var errOut bytes.Buffer
	assert.Equal(t, 1, run(context.Background(), testHost(), []string{"acp"},
		emptyFile(t), io.Discard, &errOut))
	assert.True(t, strings.Contains(errOut.String(), "overflow"))
}
