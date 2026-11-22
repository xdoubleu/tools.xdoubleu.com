package vm

import (
	"bytes"
	"encoding/json"
	"regexp"
	"strconv"
	"strings"
	"time"
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
	// LimitFrom reports whether one line of output says the harness's provider
	// refused further use for a usage limit, and when it lifts (zero if unsaid).
	LimitFrom(line []byte) (bool, time.Time)
	// TransientError reports whether one line of output says the harness could not
	// reach its provider at all: a failure the next run may not hit.
	TransientError(line []byte) bool
	// ACP is the argv that serves the Agent Client Protocol over stdio, or nil when
	// the harness has no ACP server for a chat (adr-0060).
	ACP() []string
}

// errorEvent is the type of a harness stream's error event.
const errorEvent = "error"

// transportErrorLine matches an error event whose text is a provider transport
// failure. A usage limit is LimitFrom's, and any other error is the work's.
func transportErrorLine(line []byte) bool {
	lower := bytes.ToLower(line)
	if !bytes.Contains(lower, []byte(`"error"`)) &&
		!bytes.Contains(lower, []byte("is_error")) {
		return false
	}
	if bytes.Contains(lower, []byte("limit")) {
		return false
	}
	// The transport failures a harness reports; a run that hits one is worth
	// repeating in place.
	marks := []string{
		"no internet route", "ehostunreach", "econnreset", "econnrefused",
		"enotfound", "etimedout", "socket hang up", "connection error",
		"request timed out", "fetch failed",
	}
	for _, mark := range marks {
		if bytes.Contains(lower, []byte(mark)) {
			return true
		}
	}
	return false
}

// ClaudeCode runs the unmodified `claude` CLI headless on the subscription: never
// the Agent SDK. The flags follow Claude Code's documented print mode and are
// checked on the Mac by the sandbox probe's manual list.
type ClaudeCode struct{}

// ACP implements Harness: Anthropic's Agent SDK terms keep Claude Code out of a
// chat served to others, so it has none (adr-0060).
func (ClaudeCode) ACP() []string { return nil }

// Command implements Harness.
func (ClaudeCode) Command() []string {
	return []string{
		"sh", "-c", claudeStart, claudeBin, claudeBin, "-p", "--output-format", "stream-json",
		"--verbose", "--permission-mode", "bypassPermissions",
	}
}

// ClaudeConfigSeed is where the job image keeps the Claude Code config (plugins,
// marketplaces, settings). CLAUDE_CONFIG_DIR replaces the whole config directory,
// so a new session directory is filled from here once; a resumed one is left alone.
const ClaudeConfigSeed = "/opt/claude-config"

// Interactive is the argv for Claude Code on a terminal, with args passed on.
func (ClaudeCode) Interactive(args []string) []string {
	return append([]string{"sh", "-c", claudeStart, claudeBin, claudeBin}, args...)
}

const (
	claudeBin   = "claude"
	openCodeBin = "opencode"
)

// MCPEnv switches the board's MCP server off when set to MCPOff, for a sandbox whose
// proxy has no route to it. MCPServersEnv lists, space separated, the other MCP
// servers the proxy routes at /mcp-<name>; names are validated (MCPServerName), so
// they are safe in the shell and the JSON.
const (
	MCPEnv        = "FACTORY_MCP"
	MCPOff        = "off"
	MCPServersEnv = "FACTORY_MCP_SERVERS"
)

// mcpList builds $list, the JSON members of the servers to configure, each shaped by
// the printf format $mfmt (given the name and the proxy URL); empty if there are none.
const mcpList = `list=""; sep=""
  if [ "${FACTORY_MCP:-}" != off ]; then
    list=$(printf "$mfmt" board "$FACTORY_PROXY/mcp"); sep=,
  fi
  for n in ${FACTORY_MCP_SERVERS:-}; do
    list="$list$sep$(printf "$mfmt" "$n" "$FACTORY_PROXY/mcp-$n")"; sep=,
  done`

// claudeStart seeds the config once, then, when the sandbox has a proxy, points Claude
// at it: the API through /anthropic and the board's and the configured MCP servers
// through /mcp and /mcp-<name> with no credential (the host adds it).
const claudeStart = `[ -e "$CLAUDE_CONFIG_DIR/.seeded" ] || ` +
	`{ cp -R "$CLAUDE_CONFIG_SEED/." "$CLAUDE_CONFIG_DIR/" && ` +
	`touch "$CLAUDE_CONFIG_DIR/.seeded"; } || exit 1
if [ -n "${FACTORY_PROXY:-}" ]; then
  export ANTHROPIC_BASE_URL="$FACTORY_PROXY/anthropic"
  mfmt='"%s":{"type":"http","url":"%s"}'
  ` + mcpList + `
  [ -n "$list" ] || exec "$@"
  mcp="$HOME/factory-mcp.json"
  printf '{"mcpServers":{%s}}\n' "$list" > "$mcp" || exit 1
  exec "$@" --mcp-config "$mcp"
fi
exec "$@"`

// Resume implements Harness.
func (c ClaudeCode) Resume(sessionID string) []string {
	return append(c.Command(), "--resume", sessionID)
}

// SessionEnv implements Harness: Claude Code keeps its sessions under its config
// directory, keyed by the working directory, which is the same in every sandbox.
func (ClaudeCode) SessionEnv(dir string) map[string]string {
	return map[string]string{
		"CLAUDE_CONFIG_DIR":  dir,
		"CLAUDE_CONFIG_SEED": ClaudeConfigSeed,
	}
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

// LimitFrom implements Harness. Claude Code's stream carries a `rate_limit_event`
// whose `rate_limit_info` has a status ("allowed", "allowed_warning" or "rejected")
// and `resetsAt` in epoch seconds; only "rejected" is a stop. A limit can also show
// as an error result reading "usage limit reached" or "You've hit your ... limit".
// Both forms come from the CLI's own strings, not from a limit hit on the wire.
func (ClaudeCode) LimitFrom(line []byte) (bool, time.Time) {
	if !bytes.Contains(line, []byte("rate_limit")) &&
		!bytes.Contains(line, []byte("is_error")) {
		return false, time.Time{}
	}
	var event struct {
		Type    string `json:"type"`
		IsError bool   `json:"is_error"`
		Result  string `json:"result"`
		Info    struct {
			Status   string  `json:"status"`
			ResetsAt float64 `json:"resetsAt"`
		} `json:"rate_limit_info"`
	}
	if json.Unmarshal(line, &event) != nil {
		return false, time.Time{}
	}
	switch {
	case event.Type == "rate_limit_event" && event.Info.Status == "rejected":
		return true, epochSeconds(event.Info.ResetsAt)
	case event.Type == "result" && event.IsError:
		return LimitInText(event.Result)
	}
	return false, time.Time{}
}

// TransientError implements Harness. Claude Code reports a request it could not
// make as an assistant event with an `error` field, or an error result, whose text
// is the transport's own ("API Error: No internet route ... (EHOSTUNREACH)").
func (ClaudeCode) TransientError(line []byte) bool { return transportErrorLine(line) }

// LimitInText reads Claude Code's error text for a usage limit. The older form is
// "Claude AI usage limit reached|<epoch seconds>".
func LimitInText(text string) (bool, time.Time) {
	lower := strings.ToLower(strings.ReplaceAll(text, "\u2019", "'"))
	hitYourLimit := strings.Contains(lower, "you've hit your") &&
		strings.Contains(lower, "limit")
	if !strings.Contains(lower, "usage limit reached") && !hitYourLimit {
		return false, time.Time{}
	}
	if _, after, found := strings.Cut(lower, "usage limit reached|"); found {
		if secs, err := strconv.ParseFloat(strings.TrimSpace(after), 64); err == nil {
			return true, epochSeconds(secs)
		}
	}
	return true, time.Time{}
}

// maxLimitAhead bounds a reset time worth believing: weekly caps are the longest. The
// server keeps the same window.
const maxLimitAhead = 15 * 24 * time.Hour

// epochSeconds is a reset time from epoch seconds, or zero if it is not a plausible
// time in the next fifteen days.
func epochSeconds(secs float64) time.Time {
	if secs <= 0 {
		return time.Time{}
	}
	at := time.Unix(int64(secs), 0).UTC()
	if at.Before(time.Now()) || at.After(time.Now().Add(maxLimitAhead)) {
		return time.Time{}
	}
	return at
}

// LimitFrom implements Harness: OpenRouter's out-of-credits (402) and rate-limit
// (429) refusals reach the stream as error events carrying the provider's message.
// OpenRouter says nothing about when either lifts, so the reset is zero.
func (OpenCode) LimitFrom(line []byte) (bool, time.Time) {
	if !bytes.Contains(line, []byte(`"error"`)) {
		return false, time.Time{}
	}
	var event struct {
		Type string `json:"type"`
	}
	// Only an error event counts: the agent's own text may talk about limits.
	if json.Unmarshal(line, &event) != nil || event.Type != errorEvent {
		return false, time.Time{}
	}
	// The texts of OpenRouter's credit and rate-limit errors.
	marks := []string{
		"insufficient credits", "requires more credits", "can only afford",
		"rate limit exceeded", "rate-limited", "rate limited", "free-models-per-day",
		`"code":402`, `"code":429`, `"statuscode":402`, `"statuscode":429`,
	}
	lower := bytes.ToLower(line)
	for _, mark := range marks {
		if bytes.Contains(lower, []byte(mark)) {
			return true, time.Time{}
		}
	}
	return false, time.Time{}
}

// TransientError implements Harness: OpenCode reports a failed request as an error
// event carrying the transport's own message.
func (OpenCode) TransientError(line []byte) bool { return transportErrorLine(line) }

// modelPattern is an OpenRouter model id, "vendor/model" with an optional tag. It is
// matched again here because the server is untrusted and the id goes into argv.
var modelPattern = regexp.MustCompile(
	`^[A-Za-z0-9][A-Za-z0-9._-]*/[A-Za-z0-9][A-Za-z0-9._:-]*$`,
)

// maxModelLen bounds a model id.
const maxModelLen = 100

// ValidModel reports whether model is a usable OpenRouter model id.
func ValidModel(model string) bool {
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

// openCodePrelude writes the job's OpenCode config when the sandbox has a proxy:
// OpenRouter through /openrouter as the default model (declared, as the catalogue
// needs the network), and the board's (unless switched off) and the configured MCP
// servers through /mcp and /mcp-<name>. The model id is validated, so it is safe inside
// the JSON string.
const openCodePrelude = `if [ -n "${FACTORY_PROXY:-}" ]; then
  cfg="$HOME/opencode-job.json"
  p="$FACTORY_PROXY"
  mfmt='"%s":{"type":"remote","url":"%s","oauth":false}'
  ` + mcpList + `
  mcp=""
  [ -z "$list" ] || mcp=",\"mcp\":{$list}"
  printf '{"model":"openrouter/%s",` +
	`"provider":{"openrouter":{"options":{"baseURL":"%s/openrouter/v1"},` +
	`"models":{"%s":{}}}}%s}\n' "$1" "$p" "$1" "$mcp" > "$cfg" || exit 1
  export OPENCODE_CONFIG="$cfg"
fi
`

const (
	openCodeStart = openCodePrelude + `exec opencode run --standalone --format json ` +
		`--auto -m "openrouter/$1" "$(cat)"`
	openCodeAgain = openCodePrelude + `exec opencode run --standalone --format json ` +
		`--auto -m "openrouter/$1" --session "$2" "$(cat)"`
	openCodeTerminal = openCodePrelude + `shift
exec opencode --standalone "$@"`
	openCodeACP = openCodePrelude + `exec opencode acp`
)

// ACP implements Harness: OpenCode serves ACP on stdio from `opencode acp`, the
// working directory being the checkout.
func (o OpenCode) ACP() []string {
	return []string{"sh", "-c", openCodeACP, openCodeBin, o.Model}
}

// Interactive is the argv for OpenCode's terminal UI, with args passed on.
func (o OpenCode) Interactive(args []string) []string {
	return append([]string{"sh", "-c", openCodeTerminal, openCodeBin, o.Model}, args...)
}

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
