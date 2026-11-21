package main

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"tools.xdoubleu.com/wardcase/internal/ward"
)

// emptyFile is standard input that is a file, not a terminal.
func emptyFile(t *testing.T) *os.File {
	t.Helper()
	path := filepath.Join(t.TempDir(), "in")
	require.NoError(t, os.WriteFile(path, nil, 0o600))
	f, err := os.Open(path)
	require.NoError(t, err)
	t.Cleanup(func() { _ = f.Close() })
	return f
}

func TestHelpPrintsUsage(t *testing.T) {
	var out bytes.Buffer
	code := run(context.Background(), testHost(), []string{"--help"}, emptyFile(t), &out,
		io.Discard)
	assert.Equal(t, 0, code)
	assert.Contains(t, out.String(), "wardcase secret set")
}

func TestConfigShowsThePathAndDefaults(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	var out bytes.Buffer
	code := run(context.Background(), testHost(), []string{"config"}, emptyFile(t), &out,
		io.Discard)
	assert.Equal(t, 0, code)
	assert.Contains(t, out.String(), ".config/wardcase/config.toml")
	assert.Contains(t, out.String(), `overflow = "ask"`)
}

func TestABadConfigFailsEveryCommandThatReadsIt(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	dir := filepath.Join(home, ".config", "wardcase")
	require.NoError(t, os.MkdirAll(dir, 0o700))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "config.toml"),
		[]byte(`overflow = "x"`), 0o600))
	var errOut bytes.Buffer
	assert.Equal(t, 1, run(context.Background(), testHost(), []string{"config"},
		emptyFile(t), io.Discard, &errOut))
	assert.Equal(t, 1, run(context.Background(), testHost(), []string{"claude"},
		emptyFile(t), io.Discard, &errOut))
	assert.Contains(t, errOut.String(), "overflow")
}

func TestSecretNeedsAKnownNameAndAValue(t *testing.T) {
	var errOut bytes.Buffer
	set := func(args []string, in string) int {
		return secret(memSecrets{}, args, strings.NewReader(in), io.Discard, &errOut)
	}
	assert.Equal(t, exitUsage, set([]string{"set", "github"}, "v"))
	assert.Equal(t, exitUsage, set([]string{"get", "anthropic"}, "v"))
	assert.Equal(t, 1, set([]string{"set", "anthropic"}, ""))
	assert.Contains(t, errOut.String(), "pipe the value in")
}

type listExec struct {
	out  string
	code int
	err  error
}

func (l listExec) Exec(
	_ context.Context, _ []string, _ io.Reader, out io.Writer,
) (int, error) {
	_, _ = io.WriteString(out, l.out)
	return l.code, l.err
}

func TestTheImageIsTheReleasesOwn(t *testing.T) {
	assert.Equal(t, "ghcr.io/xdoubleu/wardcase:1.2.3", ImageRef("1.2.3"))
}

// recordExec answers the container CLI's image calls and records them.
type recordExec struct {
	have  bool
	pull  int
	calls []string
}

func (r *recordExec) Exec(
	_ context.Context, argv []string, _ io.Reader, _ io.Writer,
) (int, error) {
	r.calls = append(r.calls, strings.Join(argv[1:], " "))
	switch argv[2] {
	case "inspect":
		if r.have {
			return 0, nil
		}
		return 1, nil
	case "pull":
		return r.pull, nil
	}
	return 1, errors.New("unexpected")
}

func TestTheImageIsPulledOnlyWhenMissing(t *testing.T) {
	have := &recordExec{have: true, pull: 0, calls: nil}
	require.NoError(t, ensureImage(context.Background(), have, "img:1", io.Discard))
	assert.Equal(t, []string{"image inspect img:1"}, have.calls)

	var notes bytes.Buffer
	missing := &recordExec{have: false, pull: 0, calls: nil}
	require.NoError(t, ensureImage(context.Background(), missing, "img:1", &notes))
	assert.Equal(t, []string{"image inspect img:1", "image pull img:1"}, missing.calls)
	assert.Contains(t, notes.String(), "pulling img:1")

	failing := &recordExec{have: false, pull: 1, calls: nil}
	require.ErrorContains(t,
		ensureImage(context.Background(), failing, "img:1", io.Discard), "pull img:1")
}

func TestAnAnswerDefaultsToYes(t *testing.T) {
	answers := map[string]bool{"\n": true, "y\n": true, "YES\n": true, "n\n": false}
	for in, want := range answers {
		var errOut bytes.Buffer
		assert.Equal(t, want, ask(strings.NewReader(in), &errOut)("Go on?"), in)
		assert.Equal(t, "Go on? [Y/n] ", errOut.String())
	}
}

func TestAFileIsNotATerminal(t *testing.T) {
	assert.False(t, isTerminal(emptyFile(t)))
}

// memSecrets is a keychain in memory.
type memSecrets map[string]string

func (m memSecrets) Get(name string) (string, error) {
	if v, ok := m[name]; ok {
		return v, nil
	}
	return "", ward.ErrNoSecret
}

func (m memSecrets) Set(name, value string) error {
	m[name] = value
	return nil
}

func (m memSecrets) Delete(name string) error { delete(m, name); return nil }

// hostCLI answers the container CLI's version and image list.
type hostCLI struct{ version string }

func (h hostCLI) Exec(
	_ context.Context, argv []string, _ io.Reader, out io.Writer,
) (int, error) {
	switch strings.Join(argv[1:], " ") {
	case "--version":
		_, _ = io.WriteString(out, h.version)
	case "image inspect " + ImageRef(Version):
	default:
		return 1, errors.New("unexpected: " + strings.Join(argv, " "))
	}
	return 0, nil
}

// testHost fails any container call; a test that needs the CLI builds its own.
func testHost() host {
	return host{
		exec: listExec{out: "", code: 1, err: nil}, secrets: memSecrets{}, open: nil,
	}
}

func TestSecretSetStoresTheTrimmedValue(t *testing.T) {
	m := memSecrets{}
	var out bytes.Buffer
	code := secret(m, []string{"set", "openrouter"}, strings.NewReader(" k1 \n"), &out,
		io.Discard)
	assert.Equal(t, 0, code)
	assert.Equal(t, "k1", m["openrouter"])
	assert.Contains(t, out.String(), "Stored the openrouter secret")
}

func TestARunWithoutTheClaudeTokenStopsBeforeTheVM(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	h := host{
		exec:    hostCLI{version: "container CLI version 1.6.0 (build: release)"},
		secrets: memSecrets{}, open: nil,
	}
	var errOut bytes.Buffer
	code := run(context.Background(), h, []string{"--print", "claude", "hi"},
		emptyFile(t), io.Discard, &errOut)
	assert.Equal(t, 1, code)
	assert.Contains(t, errOut.String(), "anthropic")
}

func TestAnOldContainerCLIIsRefused(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	h := host{
		exec:    hostCLI{version: "container CLI version 0.9.0"},
		secrets: memSecrets{}, open: nil,
	}
	var errOut bytes.Buffer
	code := run(context.Background(), h, []string{"opencode"}, emptyFile(t),
		io.Discard, &errOut)
	assert.Equal(t, 1, code)
	assert.Contains(t, errOut.String(), "1.6.x is required")
}

func TestInPlaceIsAFlagBeforeTheHarness(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	h := host{
		exec:    hostCLI{version: "container CLI version 1.6.0 (build: release)"},
		secrets: memSecrets{}, open: nil,
	}
	var errOut bytes.Buffer
	code := run(context.Background(), h, []string{"--in-place", "--print", "claude", "hi"},
		emptyFile(t), io.Discard, &errOut)
	assert.Equal(t, 1, code)
	assert.Contains(t, errOut.String(), "anthropic")
}

func store(t *testing.T, h host) memSecrets {
	t.Helper()
	m, ok := h.secrets.(memSecrets)
	require.True(t, ok)
	return m
}

func mcpHome(t *testing.T, config string) {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)
	dir := filepath.Join(home, ".config", "wardcase")
	require.NoError(t, os.MkdirAll(dir, 0o700))
	path := filepath.Join(dir, "config.toml")
	require.NoError(t, os.WriteFile(path, []byte(config), 0o600))
}

const oneMCP = "[[mcp]]\nname = \"posthog\"\nurl = \"https://mcp.posthog.com/mcp\"\n"

func TestMCPListShowsEachServerAndWhetherItIsSignedIn(t *testing.T) {
	mcpHome(t, oneMCP)
	h := testHost()
	var out bytes.Buffer
	assert.Equal(t, 0, run(context.Background(), h, []string{"mcp", "list"}, emptyFile(t),
		&out, io.Discard))
	assert.Equal(t, "posthog\thttps://mcp.posthog.com/mcp\tnot signed in\tjobs=false\n",
		out.String())

	store(t, h)["mcp-posthog"] = `{"refresh_token":"r"}`
	out.Reset()
	run(context.Background(), h, []string{"mcp", "list"}, emptyFile(t), &out, io.Discard)
	assert.Contains(t, out.String(), "\tsigned in\t")
}

func TestMCPLogoutForgetsTheStoredSignIn(t *testing.T) {
	mcpHome(t, oneMCP)
	h := testHost()
	store(t, h)["mcp-posthog"] = `{"refresh_token":"r"}`
	assert.Equal(t, 0, run(context.Background(), h, []string{"mcp", "logout", "posthog"},
		emptyFile(t), io.Discard, io.Discard))
	assert.Empty(t, store(t, h))
}

func TestMCPNeedsAConfiguredNameAndAKnownCommand(t *testing.T) {
	mcpHome(t, oneMCP)
	var errOut bytes.Buffer
	for _, args := range [][]string{{"mcp", "login", "nope"}, {"mcp", "logout", "nope"}} {
		assert.Equal(t, 1, run(context.Background(), testHost(), args, emptyFile(t),
			io.Discard, &errOut), args)
	}
	assert.Contains(t, errOut.String(), `no [[mcp]] named "nope"`)
	assert.Equal(t, exitUsage, run(context.Background(), testHost(), []string{"mcp"},
		emptyFile(t), io.Discard, &errOut))
}

func TestMCPLoginReportsAServerThatCannotBeSignedIn(t *testing.T) {
	mcpHome(t, "[[mcp]]\nname = \"dead\"\nurl = \"http://127.0.0.1:1/mcp\"\n")
	var errOut bytes.Buffer
	args := []string{"mcp", "login", "dead"}
	assert.Equal(t, 1, run(context.Background(), testHost(), args, emptyFile(t),
		io.Discard, &errOut))
	assert.Contains(t, errOut.String(), "RFC 9728")
}

func TestMCPFailsOnABadConfig(t *testing.T) {
	mcpHome(t, "overflow = \"x\"")
	assert.Equal(t, 1, run(context.Background(), testHost(), []string{"mcp", "list"},
		emptyFile(t), io.Discard, io.Discard))
}
