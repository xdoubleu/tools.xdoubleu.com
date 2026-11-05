package runner

import (
	"bytes"
	"encoding/json"
	"regexp"
)

// HarnessClaudeCode is the server's name for Claude Code (the proto enum).
const HarnessClaudeCode = "HARNESS_CLAUDE_CODE"

// HarnessOpenCode is the server's name for OpenCode.
const HarnessOpenCode = "HARNESS_OPENCODE"

// Harness is the agent CLI a job runs inside the sandbox.
type Harness interface {
	// Command is the argv to run; the prompt goes in on stdin, never in argv.
	Command() []string
	// Resume is the argv that continues an earlier session instead of starting one.
	Resume(sessionID string) []string
	// SessionEnv points the harness's session state at dir (inside the sandbox),
	// so a later job can resume what this one started.
	SessionEnv(dir string) map[string]string
	// SessionID reads the harness's session id from one line of its output, or "".
	SessionID(line []byte) string
}

// ClaudeCode runs the unmodified `claude` CLI headless on the subscription: never
// the Agent SDK. The flags follow Claude Code's documented print mode and are
// checked on the Mac by the sandbox probe's manual list.
type ClaudeCode struct{}

// Command implements Harness.
func (ClaudeCode) Command() []string {
	return []string{
		"claude", "-p", "--output-format", "stream-json", "--verbose",
		"--permission-mode", "bypassPermissions",
	}
}

// Resume implements Harness.
func (c ClaudeCode) Resume(sessionID string) []string {
	return append(c.Command(), "--resume", sessionID)
}

// SessionEnv implements Harness: Claude Code keeps its sessions under its config
// directory, keyed by the working directory, which is the same in every sandbox.
func (ClaudeCode) SessionEnv(dir string) map[string]string {
	return map[string]string{"CLAUDE_CONFIG_DIR": dir}
}

// SessionID implements Harness: the init event of the stream carries it.
func (ClaudeCode) SessionID(line []byte) string {
	if !bytes.Contains(line, []byte(`"session_id"`)) {
		return ""
	}
	var event struct {
		SessionID string `json:"session_id"`
	}
	if json.Unmarshal(line, &event) != nil {
		return ""
	}
	return event.SessionID
}

// modelPattern is an OpenRouter model id, "vendor/model" with an optional tag. It is
// matched again here because the server is untrusted and the id goes into argv.
var modelPattern = regexp.MustCompile(
	`^[A-Za-z0-9][A-Za-z0-9._-]*/[A-Za-z0-9][A-Za-z0-9._:-]*$`,
)

// maxModelLen bounds a model id.
const maxModelLen = 100

// validModel reports whether model is a usable OpenRouter model id.
func validModel(model string) bool {
	return len(model) <= maxModelLen && modelPattern.MatchString(model)
}

// openRouterPlaceholder stands in for OpenRouter's key inside the sandbox: the host
// proxy puts the real one on the request (adr-0043), so the VM never holds it.
const openRouterPlaceholder = "replaced-by-the-host-proxy"

// OpenCode runs the `opencode` CLI headless on OpenRouter. OpenCode's docs show the
// prompt as arguments and don't mention stdin, so a small shell step reads the
// prompt from stdin into the one argument; the model and session id come in as
// positional parameters, never spliced into the script. The flags follow OpenCode's
// documented `run` options; the event shape comes from third-party notes, and both
// are checked on the Mac by the sandbox probe's manual list.
type OpenCode struct {
	// Model is an OpenRouter model id, "vendor/model".
	Model string
}

const (
	openCodeStart = `exec opencode run --format json --auto -m "openrouter/$1" "$(cat)"`
	openCodeAgain = `exec opencode run --format json --auto -m "openrouter/$1" ` +
		`--session "$2" "$(cat)"`
)

// Command implements Harness.
func (o OpenCode) Command() []string {
	return []string{"sh", "-c", openCodeStart, "opencode", o.Model}
}

// Resume implements Harness.
func (o OpenCode) Resume(sessionID string) []string {
	return []string{"sh", "-c", openCodeAgain, "opencode", o.Model, sessionID}
}

// SessionEnv implements Harness: OpenCode keeps its sessions under the XDG data
// directory, and its OpenRouter key is a placeholder the proxy replaces.
func (OpenCode) SessionEnv(dir string) map[string]string {
	return map[string]string{
		"XDG_DATA_HOME":      dir,
		"OPENROUTER_API_KEY": openRouterPlaceholder,
	}
}

// SessionID implements Harness: every event of the stream carries the session id.
func (OpenCode) SessionID(line []byte) string {
	if !bytes.Contains(line, []byte(`"sessionI`)) &&
		!bytes.Contains(line, []byte(`"session_id"`)) {
		return ""
	}
	var event struct {
		SessionID string `json:"sessionID"`
		Snake     string `json:"session_id"`
	}
	if json.Unmarshal(line, &event) != nil {
		return ""
	}
	if event.SessionID != "" {
		return event.SessionID
	}
	return event.Snake
}
