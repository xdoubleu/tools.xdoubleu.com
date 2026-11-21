package ward_test

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"tools.xdoubleu.com/wardcase/internal/ward"
	"tools.xdoubleu.com/wardcase/vm"
)

const limitReply = `{"type":"assistant","message":{"content":[{"type":"text",` +
	`"text":"You’ve hit your session limit · resets 7am"}]}}`

// run is one harness run the fake VM saw.
type run struct {
	spec    vm.SandboxSpec
	command []string
	stdin   string
	tty     bool
}

// fakeBox records each run; act plays the harness, by writing its transcript or
// output.
type fakeBox struct {
	runs    []run
	act     func(i int, r run, out io.Writer) int
	openErr error
}

func (b *fakeBox) Run(ctx context.Context, spec vm.SandboxSpec) (int, error) {
	s, err := b.Open(ctx, spec)
	if err != nil {
		return -1, err
	}
	return s.Exec(ctx, spec.Command, spec.Stdin, spec.Log)
}

func (b *fakeBox) Open(
	_ context.Context, spec vm.SandboxSpec,
) (vm.SandboxSession, error) {
	if b.openErr != nil {
		return nil, b.openErr
	}
	return &fakeSession{box: b, spec: spec}, nil
}

type fakeSession struct {
	box  *fakeBox
	spec vm.SandboxSpec
}

func (s *fakeSession) Exec(
	_ context.Context, command []string, stdin string, log io.Writer,
) (int, error) {
	r := run{spec: s.spec, command: command, stdin: stdin, tty: false}
	return s.record(r, log), nil
}

func (s *fakeSession) Attach(
	_ context.Context, command []string, in io.Reader, out io.Writer, tty bool,
) (int, error) {
	typed, _ := io.ReadAll(in)
	r := run{spec: s.spec, command: command, stdin: string(typed), tty: tty}
	return s.record(r, out), nil
}

func (s *fakeSession) record(r run, out io.Writer) int {
	s.box.runs = append(s.box.runs, r)
	if s.box.act == nil {
		return 0
	}
	return s.box.act(len(s.box.runs)-1, r, out)
}

func (s *fakeSession) Close() {}

type memSecrets map[string]string

func (m memSecrets) Get(name string) (string, error) {
	if v, ok := m[name]; ok {
		return v, nil
	}
	return "", ward.ErrNoSecret
}

func (m memSecrets) Set(name, value string) error { m[name] = value; return nil }

// fakeGit answers like a worktree whose shared git directory is elsewhere.
func fakeGit(top, common string) ward.GitFunc {
	return func(_ context.Context, _ string, args ...string) (string, error) {
		switch strings.Join(args, " ") {
		case "rev-parse --show-toplevel":
			return top + "\n", nil
		case "rev-parse --path-format=absolute --git-common-dir":
			return common + "\n", nil
		case "rev-parse HEAD":
			return "abc123\n", nil
		case "rev-parse --abbrev-ref HEAD":
			return "feature\n", nil
		case "log --oneline abc123..HEAD":
			return "def456 add the thing\n", nil
		case "status --short":
			return " M main.go\n", nil
		}
		return "", errors.New("unexpected git " + strings.Join(args, " "))
	}
}

type fixture struct {
	box   *fakeBox
	r     *ward.Runner
	out   bytes.Buffer
	notes bytes.Buffer
	asked []string
}

func newFixture(t *testing.T, overflow string, answer bool) *fixture {
	t.Helper()
	f := &fixture{
		box: &fakeBox{runs: nil, act: nil, openErr: nil},
		r:   nil, out: bytes.Buffer{}, notes: bytes.Buffer{}, asked: nil,
	}
	f.r = &ward.Runner{
		Box: f.box,
		Secrets: memSecrets{
			ward.SecretAnthropic: "oat-token", ward.SecretOpenRouter: "or-key",
		},
		Config: ward.Config{
			Order: []string{"claude", "opencode"}, Overflow: overflow, Image: "",
			OpenCode: ward.OpenCodeConfig{Model: "vendor/model"},
		},
		StateDir: t.TempDir(), TempDir: t.TempDir(),
		Git: fakeGit("/Users/me/wt", "/Users/me/repo/.git"),
		NewProxy: func(cfg vm.EgressConfig) (vm.ProxyListener, error) {
			return vm.NewEgressProxy(cfg)
		},
		In: strings.NewReader(""), Out: &f.out, Notes: &f.notes, Terminal: true,
		Ask: func(q string) bool {
			f.asked = append(f.asked, q)
			return answer
		},
		Now: nil,
	}
	return f
}

// writeTranscript writes a Claude Code transcript into a run's session directory.
func writeTranscript(t *testing.T, r run, lines ...string) {
	t.Helper()
	dir := filepath.Join(r.spec.SessionDir, "projects", "-work")
	require.NoError(t, os.MkdirAll(dir, 0o700))
	body := strings.Join(lines, "\n") + "\n"
	require.NoError(t, os.WriteFile(filepath.Join(dir, "s1.jsonl"), []byte(body), 0o600))
}

func TestClaudeRunsInteractivelyInTheVMOnTheWorktree(t *testing.T) {
	f := newFixture(t, ward.OverflowAsk, true)
	code, err := f.r.Run(context.Background(), ward.Request{
		Dir: "/Users/me/wt/sub", Harness: "", Args: []string{"--resume"}, Print: false,
	})
	require.NoError(t, err)
	assert.Equal(t, 0, code)
	require.Len(t, f.box.runs, 1)

	r := f.box.runs[0]
	assert.True(t, r.tty)
	assert.Equal(t, "/Users/me/wt", r.spec.CloneDir, "the repository's top level")
	assert.Equal(t, []vm.Mount{{Source: "/Users/me/repo/.git",
		Target: "/Users/me/repo/.git", ReadOnly: false}}, r.spec.Mounts)
	assert.True(t, r.spec.SkipInstall)
	assert.Equal(t, "oat-token", r.spec.Env["CLAUDE_CODE_OAUTH_TOKEN"])
	assert.Equal(t, vm.MCPOff, r.spec.Env[vm.MCPEnv])
	assert.True(t, strings.HasPrefix(r.spec.Env["FACTORY_JOB"], "wardcase-"))
	assert.Equal(t, "/session", r.spec.Env["CLAUDE_CONFIG_DIR"])
	assert.NotNil(t, r.spec.Proxy)
	assert.Equal(t, "--resume", r.command[len(r.command)-1])
	assert.True(t, strings.HasPrefix(r.spec.SessionDir, f.r.StateDir))
	assert.Equal(t, "claude", filepath.Base(r.spec.SessionDir))
	assert.Empty(t, f.asked)
}

func TestTheSameCheckoutKeepsItsSessionsAcrossRuns(t *testing.T) {
	f := newFixture(t, ward.OverflowOff, false)
	req := ward.Request{Dir: "/Users/me/wt", Harness: "claude", Args: nil, Print: false}
	_, err := f.r.Run(context.Background(), req)
	require.NoError(t, err)
	_, err = f.r.Run(context.Background(), req)
	require.NoError(t, err)
	assert.Equal(t, f.box.runs[0].spec.SessionDir, f.box.runs[1].spec.SessionDir)
}

func TestALimitedClaudeAsksAndHandsTheWorkToOpenCode(t *testing.T) {
	f := newFixture(t, ward.OverflowAsk, true)
	f.box.act = func(i int, r run, _ io.Writer) int {
		if i == 0 {
			writeTranscript(t, r,
				`{"type":"user","message":{"content":"build the parser"}}`,
				`{"type":"assistant","message":{"content":[{"type":"tool_use","name":"Bash"},`+
					`{"type":"text","text":"Parser half done."}]}}`,
				`{"type":"user","message":{"content":[{"type":"tool_result","content":"ok"}]}}`,
				limitReply)
			return 1
		}
		return 0
	}
	code, err := f.r.Run(context.Background(),
		ward.Request{Dir: "/Users/me/wt", Harness: "claude", Args: nil, Print: false})
	require.NoError(t, err)
	assert.Equal(t, 0, code, "the last harness's exit")
	require.Len(t, f.box.runs, 2)
	require.Len(t, f.asked, 1)
	assert.Contains(t, f.asked[0], "Continue in opencode?")

	next := f.box.runs[1]
	assert.Equal(t, "opencode", filepath.Base(next.spec.SessionDir))
	assert.Equal(t, "--prompt", next.command[len(next.command)-2])
	assert.Contains(t, next.command[len(next.command)-1], "/session/wardcase-handoff.md")
	assert.Empty(t, next.spec.Env["CLAUDE_CODE_OAUTH_TOKEN"], "no Claude token")
	assert.Equal(t, "/session", next.spec.Env["XDG_DATA_HOME"])

	handoff, err := os.ReadFile(filepath.Join(next.spec.SessionDir, ward.HandoffFile))
	require.NoError(t, err)
	text := string(handoff)
	assert.Contains(t, text, "# Handoff from claude to opencode")
	assert.Contains(t, text, "feature")
	assert.Contains(t, text, "def456 add the thing")
	assert.Contains(t, text, "M main.go")
	assert.Contains(t, text, "**User:** build the parser")
	assert.Contains(t, text, "**Agent:** Parser half done.")
	assert.NotContains(t, text, "tool_result")
	assert.Contains(t, f.notes.String(), "continuing in opencode")
}

func TestDecliningTheOverflowStopsAtTheLimit(t *testing.T) {
	f := newFixture(t, ward.OverflowAsk, false)
	f.box.act = func(_ int, r run, _ io.Writer) int {
		writeTranscript(t, r, limitReply)
		return 1
	}
	code, err := f.r.Run(context.Background(),
		ward.Request{Dir: "/w", Harness: "claude", Args: nil, Print: false})
	require.NoError(t, err)
	assert.Equal(t, 1, code)
	assert.Len(t, f.box.runs, 1)
}

func TestOverflowOffOnlySaysTheLimitWasHit(t *testing.T) {
	f := newFixture(t, ward.OverflowOff, true)
	f.box.act = func(_ int, r run, _ io.Writer) int {
		writeTranscript(t, r, limitReply)
		return 1
	}
	_, err := f.r.Run(context.Background(),
		ward.Request{Dir: "/w", Harness: "claude", Args: nil, Print: false})
	require.NoError(t, err)
	assert.Len(t, f.box.runs, 1)
	assert.Empty(t, f.asked)
	assert.Contains(t, f.notes.String(), "claude hit its usage limit")
}

func TestAnOldLimitInAResumedTranscriptIsNotANewOne(t *testing.T) {
	f := newFixture(t, ward.OverflowAuto, true)
	f.r.Now = func() time.Time { return time.Now().Add(time.Hour) }
	f.box.act = func(_ int, r run, _ io.Writer) int {
		writeTranscript(t, r, limitReply)
		return 0
	}
	_, err := f.r.Run(context.Background(),
		ward.Request{Dir: "/w", Harness: "claude", Args: nil, Print: false})
	require.NoError(t, err)
	assert.Len(t, f.box.runs, 1)
}

func TestHeadlessRunsPassTheirOutputThroughAndOverflowOnlyWhenAuto(t *testing.T) {
	limited := `{"type":"rate_limit_event","rate_limit_info":{"status":"rejected"}}`
	for overflow, wantRuns := range map[string]int{
		ward.OverflowAuto: 2, ward.OverflowAsk: 1, ward.OverflowOff: 1,
	} {
		f := newFixture(t, overflow, true)
		f.box.act = func(i int, _ run, out io.Writer) int {
			if i == 0 {
				_, _ = io.WriteString(out, `{"type":"system"}`+"\n"+limited)
				return 1
			}
			_, _ = io.WriteString(out, "done\n")
			return 0
		}
		_, err := f.r.Run(context.Background(), ward.Request{
			Dir: "/w", Harness: "", Args: []string{"fix", "the", "bug"}, Print: true,
		})
		require.NoError(t, err)
		require.Len(t, f.box.runs, wantRuns, overflow)
		first := f.box.runs[0]
		assert.False(t, first.tty)
		assert.Equal(t, "fix the bug", first.stdin)
		assert.Contains(t, f.out.String(), `{"type":"system"}`)
		assert.Empty(t, f.asked, "a headless run never asks")
		if wantRuns == 2 {
			assert.Contains(t, f.box.runs[1].stdin, "wardcase-handoff.md")
			assert.Contains(t, f.box.runs[1].stdin, "fix the bug")
		}
	}
}

func TestAHeadlessPromptCanComeFromStdin(t *testing.T) {
	f := newFixture(t, ward.OverflowOff, false)
	f.r.In = strings.NewReader("from stdin\n")
	_, err := f.r.Run(context.Background(),
		ward.Request{Dir: "/w", Harness: "claude", Args: nil, Print: true})
	require.NoError(t, err)
	assert.Equal(t, "from stdin\n", f.box.runs[0].stdin)

	f.r.In = strings.NewReader("  ")
	_, err = f.r.Run(context.Background(),
		ward.Request{Dir: "/w", Harness: "claude", Args: nil, Print: true})
	require.Error(t, err)
}

func TestAMissingCredentialOrModelStopsBeforeTheVMStarts(t *testing.T) {
	f := newFixture(t, ward.OverflowOff, false)
	f.r.Secrets = memSecrets{}
	_, err := f.r.Run(context.Background(),
		ward.Request{Dir: "/w", Harness: "claude", Args: nil, Print: false})
	require.ErrorIs(t, err, ward.ErrNoSecret)
	_, err = f.r.Run(context.Background(),
		ward.Request{Dir: "/w", Harness: "opencode", Args: nil, Print: false})
	require.ErrorIs(t, err, ward.ErrNoSecret)

	f.r.Secrets = memSecrets{ward.SecretOpenRouter: "k"}
	f.r.Config.OpenCode.Model = ""
	_, err = f.r.Run(context.Background(),
		ward.Request{Dir: "/w", Harness: "opencode", Args: nil, Print: false})
	require.Error(t, err)
	assert.Empty(t, f.box.runs)
}

func TestOpenCodesKeyStaysOnTheHost(t *testing.T) {
	f := newFixture(t, ward.OverflowOff, false)
	var gotKey string
	f.r.NewProxy = func(cfg vm.EgressConfig) (vm.ProxyListener, error) {
		require.NotNil(t, cfg.OpenRouterKey)
		gotKey, _ = cfg.OpenRouterKey()
		assert.Nil(t, cfg.MCPToken, "no board route")
		assert.Nil(t, cfg.PostHogKey)
		return vm.NewEgressProxy(cfg)
	}
	_, err := f.r.Run(context.Background(),
		ward.Request{Dir: "/w", Harness: "opencode", Args: nil, Print: false})
	require.NoError(t, err)
	assert.Equal(t, "or-key", gotKey)
	for k, v := range f.box.runs[0].spec.Env {
		assert.NotContains(t, v, "or-key", k)
	}
}

func TestOutsideAGitRepositoryTheDirectoryItselfIsMounted(t *testing.T) {
	f := newFixture(t, ward.OverflowOff, false)
	f.r.Git = func(context.Context, string, ...string) (string, error) {
		return "", errors.New("not a git repository")
	}
	_, err := f.r.Run(context.Background(),
		ward.Request{Dir: "/plain", Harness: "claude", Args: nil, Print: false})
	require.NoError(t, err)
	assert.Equal(t, "/plain", f.box.runs[0].spec.CloneDir)
	assert.Empty(t, f.box.runs[0].spec.Mounts)
}

func TestAGitDirectoryInsideTheCheckoutNeedsNoExtraMount(t *testing.T) {
	f := newFixture(t, ward.OverflowOff, false)
	f.r.Git = fakeGit("/Users/me/repo", "/Users/me/repo/.git")
	_, err := f.r.Run(context.Background(),
		ward.Request{Dir: "/Users/me/repo", Harness: "claude", Args: nil, Print: false})
	require.NoError(t, err)
	assert.Empty(t, f.box.runs[0].spec.Mounts)
}

func TestAVMThatWillNotStartIsAnError(t *testing.T) {
	f := newFixture(t, ward.OverflowOff, false)
	f.box.openErr = errors.New("no container")
	_, err := f.r.Run(context.Background(),
		ward.Request{Dir: "/w", Harness: "claude", Args: nil, Print: false})
	require.ErrorContains(t, err, "start the VM")
}

func TestTheLastHarnessHasNowhereToOverflow(t *testing.T) {
	f := newFixture(t, ward.OverflowAuto, true)
	f.r.Config.Order = []string{"claude"}
	f.box.act = func(_ int, r run, _ io.Writer) int {
		writeTranscript(t, r, limitReply)
		return 1
	}
	code, err := f.r.Run(context.Background(),
		ward.Request{Dir: "/w", Harness: "claude", Args: nil, Print: false})
	require.NoError(t, err)
	assert.Equal(t, 1, code)
	assert.Len(t, f.box.runs, 1)
}

func TestCodexsKeyStaysOnTheHost(t *testing.T) {
	f := newFixture(t, ward.OverflowOff, false)
	f.r.Secrets = memSecrets{ward.SecretOpenAI: "oa-key"}
	var gotKey string
	f.r.NewProxy = func(cfg vm.EgressConfig) (vm.ProxyListener, error) {
		require.NotNil(t, cfg.OpenAIKey)
		gotKey, _ = cfg.OpenAIKey()
		assert.Nil(t, cfg.OpenRouterKey)
		assert.Nil(t, cfg.MCPToken, "no board route")
		return vm.NewEgressProxy(cfg)
	}
	_, err := f.r.Run(context.Background(),
		ward.Request{Dir: "/w", Harness: "codex", Args: []string{"--version"}, Print: false})
	require.NoError(t, err)
	assert.Equal(t, "oa-key", gotKey)
	spec := f.box.runs[0].spec
	assert.Equal(t, "codex", filepath.Base(spec.SessionDir))
	for k, v := range spec.Env {
		assert.NotContains(t, v, "oa-key", k)
	}
	assert.Equal(t, "--version", f.box.runs[0].command[len(f.box.runs[0].command)-1])
}

func TestCodexNeedsItsSecret(t *testing.T) {
	f := newFixture(t, ward.OverflowOff, false)
	_, err := f.r.Run(context.Background(),
		ward.Request{Dir: "/w", Harness: "codex", Args: nil, Print: false})
	require.ErrorIs(t, err, ward.ErrNoSecret)
}

func TestAnOpenCodeCreditLimitOverflowsToCodex(t *testing.T) {
	f := newFixture(t, ward.OverflowAuto, true)
	f.r.Secrets = memSecrets{ward.SecretOpenRouter: "or-key", ward.SecretOpenAI: "oa-key"}
	f.r.Config.Order = []string{"opencode", "codex"}
	f.box.act = func(i int, _ run, out io.Writer) int {
		if i == 0 {
			_, _ = io.WriteString(out, `{"type":"error","error":{"data":`+
				`{"message":"Insufficient credits","statusCode":402}}}`+"\n")
			return 1
		}
		return 0
	}
	code, err := f.r.Run(context.Background(),
		ward.Request{Dir: "/w", Harness: "", Args: []string{"go"}, Print: true})
	require.NoError(t, err)
	assert.Equal(t, 0, code)
	require.Len(t, f.box.runs, 2)
	assert.Equal(t, "codex", filepath.Base(f.box.runs[1].spec.SessionDir))
}
