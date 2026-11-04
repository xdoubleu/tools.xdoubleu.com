package runner

import (
	"bytes"
	"encoding/json"
)

// HarnessClaudeCode is the server's name for Claude Code (the proto enum).
const HarnessClaudeCode = "HARNESS_CLAUDE_CODE"

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
