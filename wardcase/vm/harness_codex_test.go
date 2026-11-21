package vm_test

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"tools.xdoubleu.com/wardcase/vm"
)

func TestCodexRunsHeadlessWithThePromptOnStdin(t *testing.T) {
	start := vm.Codex{}.Command()
	assert.Equal(t, []string{"sh", "-c"}, start[:2])
	assert.Equal(t, "codex", start[3])
	assert.Contains(t, start[2], "exec codex \"$@\" exec --json")
	assert.Contains(t, start[2], "--dangerously-bypass-approvals-and-sandbox")
	assert.True(t, strings.HasSuffix(start[2], " -"), "the prompt is read from stdin")
	assert.Contains(t, start[2], `$FACTORY_PROXY/openai/v1`)

	again := vm.Codex{}.Resume("019a-thread")
	assert.Equal(t, []string{"codex", "019a-thread"}, again[3:])
	assert.Contains(t, again[2], `resume "$sid" -`)
	assert.NotContains(t, again[2], "019a-thread", "the session id is a parameter")
}

func TestCodexTerminalPassesArgsOnAndPointsAtTheProxy(t *testing.T) {
	argv := vm.Codex{}.Interactive([]string{"--version"})
	assert.Equal(t, []string{"codex", "--version"}, argv[3:])
	assert.Contains(t, argv[2], `$FACTORY_PROXY/openai/v1`)
}

func TestCodexHoldsOnlyAPlaceholderKey(t *testing.T) {
	env := vm.Codex{}.SessionEnv("/session")
	assert.Equal(t, "/session", env["CODEX_HOME"])
	assert.Equal(t, "replaced-by-the-host-proxy", env["OPENAI_API_KEY"])
	assert.Equal(t, "replaced-by-the-host-proxy", env["CODEX_API_KEY"])
}

func TestCodexReadsTheSessionFromTheThreadStartedEvent(t *testing.T) {
	h := vm.Codex{}
	started := []byte(`{"type":"thread.started","thread_id":"th_1"}`)
	assert.Equal(t, "th_1", h.SessionID(started))
	assert.Empty(t, h.SessionID([]byte(`{"type":"turn.started","thread_id":"th_2"}`)))
	assert.Empty(t, h.SessionID([]byte(`{"type":"item.completed"}`)))
	assert.Empty(t, h.SessionID([]byte(`{"thread_id":`)))
}

// Modelled on Codex's JSONL `error` and `turn.failed` events; UNVERIFIED against a
// real limit hit.
func TestCodexUsageLimitAndQuotaErrorsAreALimit(t *testing.T) {
	for name, line := range map[string]string{
		"error event": `{"type":"error","message":"You've hit your usage limit."}`,
		"turn failed": `{"type":"turn.failed","error":{"message":"You exceeded your ` +
			`current quota, please check your plan and billing details."}}`,
		"code": `{"type":"error","message":"insufficient_quota"}`,
		"rate": `{"type":"error","message":"Rate limit reached for gpt-5"}`,
	} {
		limited, _ := vm.Codex{}.LimitFrom([]byte(line))
		assert.True(t, limited, name)
	}
}

func TestCodexLimitCarriesItsResetTimeWhenSaid(t *testing.T) {
	reset := time.Now().Add(2 * time.Hour).Unix()
	line := fmt.Sprintf(`{"type":"error","message":%q}`,
		fmt.Sprintf(`usage limit: {"resets_at":%d}`, reset))
	limited, at := vm.Codex{}.LimitFrom([]byte(line))
	assert.True(t, limited)
	assert.Equal(t, time.Unix(reset, 0).UTC(), at)

	_, at = vm.Codex{}.LimitFrom([]byte(`{"type":"error","message":"usage limit: ` +
		`{\"resets_in_seconds\":3600}"}`))
	assert.WithinDuration(t, time.Now().Add(time.Hour), at, time.Minute)
}

func TestOnlyCodexErrorEventsAreALimit(t *testing.T) {
	for name, line := range map[string]string{
		"agent text":  `{"type":"item.completed","item":{"text":"error: usage limit"}}`,
		"other error": `{"type":"error","message":"file not found"}`,
		"not json":    `error: usage limit`,
	} {
		limited, _ := vm.Codex{}.LimitFrom([]byte(line))
		assert.False(t, limited, name)
	}
}

func TestCodexTransportFailuresAreTransient(t *testing.T) {
	reset := []byte(`{"type":"error","message":"ECONNRESET"}`)
	assert.True(t, vm.Codex{}.TransientError(reset))
	assert.False(t, vm.Codex{}.TransientError([]byte(`{"type":"turn.started"}`)))
}

func TestTheShellStepHandsCodexItsFlagsAndTheProxyBase(t *testing.T) {
	dir := t.TempDir()
	out := filepath.Join(dir, "args.txt")
	script := "#!/bin/sh\nfor a in \"$@\"; do printf '[%s]\\n' \"$a\" >> '" + out +
		"'; done\ncat >> '" + out + ".in'\n"
	require.NoError(t, os.WriteFile(filepath.Join(dir, "codex"), []byte(script), 0o755))
	run := func(argv []string, proxy string) string {
		_ = os.Remove(out)
		cmd := exec.Command(argv[0], argv[1:]...)
		cmd.Env = append(os.Environ(), "PATH="+dir+":"+os.Getenv("PATH"),
			"FACTORY_PROXY="+proxy)
		cmd.Stdin = strings.NewReader("do it $(id)")
		b, err := cmd.CombinedOutput()
		require.NoError(t, err, string(b))
		got, err := os.ReadFile(out)
		require.NoError(t, err)
		return string(got)
	}

	got := run(vm.Codex{}.Command(), "http://gw:8118")
	assert.Equal(t, "[-c]\n[openai_base_url=\"http://gw:8118/openai/v1\"]\n[exec]\n"+
		"[--json]\n[--dangerously-bypass-approvals-and-sandbox]\n"+
		"[--skip-git-repo-check]\n[-]\n", got)

	got = run(vm.Codex{}.Command(), "")
	assert.True(t, strings.HasPrefix(got, "[exec]\n"), got)

	got = run(vm.Codex{}.Resume("th_9"), "")
	assert.Contains(t, got, "[resume]\n[th_9]\n[-]\n")

	got = run(vm.Codex{}.Interactive([]string{"--version"}), "http://gw:8118")
	assert.Equal(t, "[-c]\n[openai_base_url=\"http://gw:8118/openai/v1\"]\n"+
		"[--dangerously-bypass-approvals-and-sandbox]\n[--version]\n", got)
}
