package vm

import (
	"bytes"
	"encoding/json"
	"strings"
	"time"
)

const (
	codexBin    = "codex"
	codexACPBin = "codex-acp"
)

// openAIPlaceholder stands in for OpenAI's key inside the sandbox: the host proxy
// puts the real one on the request (adr-0043), so the VM never holds it.
const openAIPlaceholder = "replaced-by-the-host-proxy"

// codexProxyArgs points Codex at the proxy's /openai route when the sandbox has one.
// The VM's own sandbox step is skipped: the VM is the sandbox.
const codexProxyArgs = `set --
if [ -n "${FACTORY_PROXY:-}" ]; then
  set -- -c "openai_base_url=\"$FACTORY_PROXY/openai/v1\""
fi
`

const (
	codexStart = codexProxyArgs + `exec codex "$@" exec --json ` +
		`--dangerously-bypass-approvals-and-sandbox --skip-git-repo-check -`
	// The session id is read before the arguments are replaced.
	codexAgain = `sid="$1"
` + codexProxyArgs + `exec codex "$@" exec --json ` +
		`--dangerously-bypass-approvals-and-sandbox --skip-git-repo-check resume "$sid" -`
	codexACP = codexProxyArgs + `exec codex-acp "$@" -c approval_policy=never ` +
		`-c sandbox_mode=danger-full-access`
	codexTerminal = `if [ -n "${FACTORY_PROXY:-}" ]; then
  exec codex -c "openai_base_url=\"$FACTORY_PROXY/openai/v1\"" ` +
		`--dangerously-bypass-approvals-and-sandbox "$@"
fi
exec codex --dangerously-bypass-approvals-and-sandbox "$@"`
)

// Codex runs the `codex` CLI headless on an OpenAI API key. The prompt comes in on
// stdin (`codex exec -`); the key stays on the host and the proxy adds it. The flags
// follow Codex's documented `exec` options; the event shape (`thread.started`,
// `turn.failed`, `error`) comes from its JSONL mode.
type Codex struct{}

// Command implements Harness.
func (Codex) Command() []string { return []string{"sh", "-c", codexStart, codexBin} }

// ACP implements Harness: Codex has no ACP server of its own, so the `codex-acp`
// adapter in the image drives it, on the same proxy route and with the VM as the
// only sandbox.
func (Codex) ACP() []string { return []string{"sh", "-c", codexACP, codexACPBin} }

// Resume implements Harness.
func (Codex) Resume(sessionID string) []string {
	return []string{"sh", "-c", codexAgain, codexBin, sessionID}
}

// Interactive is the argv for Codex's terminal UI, with args passed on.
func (Codex) Interactive(args []string) []string {
	return append([]string{"sh", "-c", codexTerminal, codexBin}, args...)
}

// SessionEnv implements Harness: Codex keeps its sessions under CODEX_HOME, and its
// key is a placeholder the proxy replaces.
func (Codex) SessionEnv(dir string) map[string]string {
	return map[string]string{
		"CODEX_HOME":     dir,
		"OPENAI_API_KEY": openAIPlaceholder,
		"CODEX_API_KEY":  openAIPlaceholder,
	}
}

// SessionID implements Harness: the first event is `thread.started` with a thread id.
func (Codex) SessionID(line []byte) string {
	if !bytes.Contains(line, []byte(`"thread_id"`)) {
		return ""
	}
	var event struct {
		Type     string `json:"type"`
		ThreadID string `json:"thread_id"`
	}
	if json.Unmarshal(line, &event) != nil || event.Type != "thread.started" {
		return ""
	}
	return event.ThreadID
}

// LimitFrom implements Harness. A refusal arrives as an `error` event or a
// `turn.failed` one whose message is the provider's; some carry `resets_at` (epoch
// seconds) or `resets_in_seconds`.
func (Codex) LimitFrom(line []byte) (bool, time.Time) {
	if !bytes.Contains(line, []byte("error")) && !bytes.Contains(line, []byte("failed")) {
		return false, time.Time{}
	}
	var event struct {
		Type    string `json:"type"`
		Message string `json:"message"`
		Error   struct {
			Message string `json:"message"`
		} `json:"error"`
	}
	if json.Unmarshal(line, &event) != nil ||
		(event.Type != "error" && event.Type != "turn.failed") {
		return false, time.Time{}
	}
	text := strings.ToLower(event.Message + " " + event.Error.Message)
	// The texts of Codex's usage-limit, exhausted-quota and rate-limit errors.
	marks := []string{
		"usage limit", "usage_limit_reached", "insufficient_quota",
		"exceeded your current quota", "rate limit", "rate_limit_exceeded",
	}
	for _, mark := range marks {
		if strings.Contains(text, mark) {
			return true, codexReset(text)
		}
	}
	return false, time.Time{}
}

// codexReset reads a reset time out of an error's text, zero if it has none.
func codexReset(text string) time.Time {
	var body struct {
		ResetsAt        float64 `json:"resets_at"`
		ResetsInSeconds float64 `json:"resets_in_seconds"`
	}
	if i := strings.Index(text, "{"); i >= 0 {
		_ = json.Unmarshal([]byte(text[i:]), &body)
	}
	if body.ResetsAt > 0 {
		return epochSeconds(body.ResetsAt)
	}
	if body.ResetsInSeconds > 0 {
		return epochSeconds(float64(time.Now().Unix()) + body.ResetsInSeconds)
	}
	return time.Time{}
}

// TransientError implements Harness.
func (Codex) TransientError(line []byte) bool { return transportErrorLine(line) }
