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
	code := run(context.Background(), []string{"--help"}, emptyFile(t), &out,
		io.Discard)
	assert.Equal(t, 0, code)
	assert.Contains(t, out.String(), "wardcase secret set")
}

func TestConfigShowsThePathAndDefaults(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	var out bytes.Buffer
	code := run(context.Background(), []string{"config"}, emptyFile(t), &out,
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
	assert.Equal(t, 1, run(context.Background(), []string{"config"}, emptyFile(t),
		io.Discard, &errOut))
	assert.Equal(t, 1, run(context.Background(), []string{"claude"}, emptyFile(t),
		io.Discard, &errOut))
	assert.Contains(t, errOut.String(), "overflow")
}

func TestSecretNeedsAKnownNameAndAValue(t *testing.T) {
	var errOut bytes.Buffer
	assert.Equal(t, exitUsage, secret([]string{"set", "github"}, strings.NewReader("v"),
		io.Discard, &errOut))
	assert.Equal(t, exitUsage, secret([]string{"get", "anthropic"}, strings.NewReader("v"),
		io.Discard, &errOut))
	assert.Equal(t, 1, secret([]string{"set", "anthropic"}, strings.NewReader(""),
		io.Discard, &errOut))
	assert.Contains(t, errOut.String(), "pipe the value in")
}

func TestTheNewestJobImageIsTheFirstListed(t *testing.T) {
	img, err := jobImageFrom("NAME         TAG     DIGEST\n" +
		"other        1       aa\nfactory-job  cf99    bb\nfactory-job  old     cc\n")
	require.NoError(t, err)
	assert.Equal(t, "factory-job:cf99", img)

	_, err = jobImageFrom("NAME TAG DIGEST\n")
	require.ErrorContains(t, err, "make image")
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

func TestTheImageListComesFromTheContainerCLI(t *testing.T) {
	img, err := newestJobImage(context.Background(),
		listExec{out: "factory-job  t1  d\n", code: 0, err: nil})
	require.NoError(t, err)
	assert.Equal(t, "factory-job:t1", img)
	_, err = newestJobImage(context.Background(), listExec{out: "", code: 1, err: nil})
	require.Error(t, err)
	_, err = newestJobImage(context.Background(),
		listExec{out: "", code: 0, err: errors.New("no container")})
	require.Error(t, err)
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

func TestHostGitRunsGitInTheDirectory(t *testing.T) {
	out, err := hostGit(context.Background(), t.TempDir(), "--version")
	require.NoError(t, err)
	assert.Contains(t, out, "git version")
	_, err = hostGit(context.Background(), t.TempDir(), "rev-parse", "--show-toplevel")
	require.Error(t, err, "not a repository")
}
