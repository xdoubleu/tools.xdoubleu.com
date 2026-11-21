package vm_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"tools.xdoubleu.com/wardcase/vm"
)

func TestOpenCodeRunsHeadlessOnOpenRouterWithTheModelAsAParameter(t *testing.T) {
	h := vm.OpenCode{Model: "vendor/model:free"}

	start := h.Command()
	assert.Equal(t, []string{"sh", "-c"}, start[:2])
	assert.Equal(t, []string{"opencode", "vendor/model:free"}, start[3:])
	assert.Contains(t, start[2], "opencode run --standalone --format json --auto")
	assert.Contains(t, start[2], `-m "openrouter/$1"`)
	assert.NotContains(t, start[2], "vendor/model", "the model is a parameter, not script")
	assert.NotContains(t, start[2], "--session")

	again := h.Resume("ses_abc123")
	assert.Equal(t, []string{"opencode", "vendor/model:free", "ses_abc123"}, again[3:])
	assert.Contains(t, again[2], `--session "$2"`)
	assert.NotContains(t, again[2], "ses_abc123", "the session id is a parameter too")
}

func TestOpenCodeKeepsItsSessionsInThePerTaskDirectoryWithoutTheRealKey(t *testing.T) {
	env := vm.OpenCode{Model: "a/b"}.SessionEnv("/session")
	assert.Equal(t, "/session", env["XDG_DATA_HOME"])
	assert.Equal(t, "replaced-by-the-host-proxy", env["OPENROUTER_API_KEY"])
}

func TestOpenCodeReadsTheSessionIDFromAnyEvent(t *testing.T) {
	h := vm.OpenCode{Model: "a/b"}
	for name, tc := range map[string]struct{ line, want string }{
		"camel case":    {`{"type":"step_start","sessionID":"ses_1","part":{}}`, "ses_1"},
		"snake case":    {`{"type":"text","session_id":"ses_2"}`, "ses_2"},
		"camel wins":    {`{"sessionID":"ses_3","session_id":"ses_4"}`, "ses_3"},
		"no session":    {`{"type":"text","part":{"text":"hi"}}`, ""},
		"not json":      {`Compacting context…`, ""},
		"broken json":   {`{"sessionID":"ses_5"`, ""},
		"empty":         {``, ""},
		"other id name": {`{"sessionIDs":["x"]}`, ""},
	} {
		assert.Equal(t, tc.want, h.SessionID([]byte(tc.line)), name)
	}
}

// fakeOpenCode puts an `opencode` on PATH that records each argument in brackets,
// one per line, and returns that directory and the file it writes to.
func fakeOpenCode(t *testing.T) (string, string) {
	t.Helper()
	dir := t.TempDir()
	out := filepath.Join(dir, "args.txt")
	script := "#!/bin/sh\nfor a in \"$@\"; do printf '[%s]\\n' \"$a\" >> '" + out +
		"'; done\n"
	require.NoError(t, os.WriteFile(filepath.Join(dir, "opencode"), []byte(script), 0o755))
	return dir, out
}

func runScript(t *testing.T, argv []string, stdin string) string {
	t.Helper()
	dir, out := fakeOpenCode(t)
	cmd := exec.Command(argv[0], argv[1:]...)
	cmd.Env = append(os.Environ(), "PATH="+dir+":"+os.Getenv("PATH"))
	cmd.Stdin = strings.NewReader(stdin)
	output, err := cmd.CombinedOutput()
	require.NoError(t, err, string(output))
	got, err := os.ReadFile(out)
	require.NoError(t, err)
	return string(got)
}

func TestTheShellStepHandsOpenCodeTheModelAndPromptAsSingleArguments(t *testing.T) {
	prompt := "You are running unattended.\n\n" +
		"# Task: it's \"quoted\" $(touch pwned) `id` ; rm -rf /\n"
	got := runScript(t, vm.OpenCode{Model: "vendor/model"}.Command(), prompt)

	assert.Equal(t, strings.Join([]string{
		"[run]", "[--standalone]", "[--format]", "[json]", "[--auto]", "[-m]",
		"[openrouter/vendor/model]",
		"[" + strings.TrimRight(prompt, "\n") + "]",
	}, "\n")+"\n", got)
	assert.NoFileExists(t, "pwned")
}

func TestTheShellStepResumesTheSessionWithoutSplicingItIntoTheScript(t *testing.T) {
	got := runScript(t, vm.OpenCode{Model: "a/b"}.Resume("ses_x"), "more\n")

	assert.Contains(t, got, "[--session]\n[ses_x]\n")
	assert.True(t, strings.HasSuffix(got, "[more]\n"), got)
}

// The lines are modelled on OpenCode's `error` event carrying OpenRouter's message
// (402 out of credits, 429 rate limit); UNVERIFIED against a real capture.
func TestOpenRouterCreditAndRateLimitErrorsAreALimit(t *testing.T) {
	h := vm.OpenCode{Model: "a/b"}
	for name, line := range map[string]string{
		"credits": `{"type":"error","sessionID":"s","error":{"name":"APIError",` +
			`"data":{"message":"Insufficient credits. Add more at openrouter.ai",` +
			`"statusCode":402}}}`,
		"afford": `{"type":"error","error":{"data":{"message":"This request requires ` +
			`more credits, or fewer max_tokens. You requested up to 8000 tokens, but ` +
			`can only afford 100"}}}`,
		"rate": `{"type":"error","error":{"data":{"message":"Rate limit exceeded: ` +
			`free-models-per-day","statusCode":429}}}`,
	} {
		limited, at := h.LimitFrom([]byte(line))
		assert.True(t, limited, name)
		assert.True(t, at.IsZero(), name)
	}
}

func TestOnlyAnErrorEventIsAnOpenCodeLimit(t *testing.T) {
	h := vm.OpenCode{Model: "a/b"}
	for name, line := range map[string]string{
		"agent text":  `{"type":"text","part":{"text":"rate limit exceeded: error"}}`,
		"other error": `{"type":"error","error":{"data":{"message":"file not found"}}}`,
		"not json":    `error: Insufficient credits`,
		"no error":    `{"type":"step_start"}`,
	} {
		limited, _ := h.LimitFrom([]byte(line))
		assert.False(t, limited, name)
	}
}
