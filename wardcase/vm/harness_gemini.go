package vm

import (
	"bytes"
	"encoding/json"
	"strings"
	"time"
)

const geminiBin = "gemini"

// geminiPlaceholder stands in for Gemini's key inside the sandbox: the host proxy
// puts the real one on the request (adr-0043), so the VM never holds it.
const geminiPlaceholder = "replaced-by-the-host-proxy"

// geminiPrelude points Gemini at the proxy's /gemini route when the sandbox has one.
const geminiPrelude = `if [ -n "${FACTORY_PROXY:-}" ]; then
  export GOOGLE_GEMINI_BASE_URL="$FACTORY_PROXY/gemini"
fi
`

const (
	geminiStart = geminiPrelude + `exec gemini --yolo --output-format stream-json`
	// The session id is a parameter, never spliced into the script.
	geminiAgain = geminiPrelude + `exec gemini --yolo --output-format stream-json ` +
		`--resume "$1"`
	geminiTerminal = geminiPrelude + `exec gemini --yolo "$@"`
	geminiACP      = geminiPrelude + `exec gemini --acp`
)

// Gemini runs the `gemini` CLI headless on a Gemini API key. The prompt comes in on
// stdin; the key stays on the host and the proxy adds it. The flags follow Gemini
// CLI's documented options; the event shape (`init`, `error`, `result`) comes from
// its stream-json mode.
type Gemini struct{}

// Command implements Harness.
func (Gemini) Command() []string { return []string{"sh", "-c", geminiStart, geminiBin} }

// ACP implements Harness: Gemini CLI serves ACP on stdio from `gemini --acp`.
func (Gemini) ACP() []string { return []string{"sh", "-c", geminiACP, geminiBin} }

// Resume implements Harness.
func (Gemini) Resume(sessionID string) []string {
	return []string{"sh", "-c", geminiAgain, geminiBin, sessionID}
}

// Interactive is the argv for Gemini's terminal UI, with args passed on.
func (Gemini) Interactive(args []string) []string {
	return append([]string{"sh", "-c", geminiTerminal, geminiBin}, args...)
}

// SessionEnv implements Harness: Gemini keeps its state under GEMINI_CLI_HOME, and
// its key is a placeholder the proxy replaces.
func (Gemini) SessionEnv(dir string) map[string]string {
	return map[string]string{
		"GEMINI_CLI_HOME": dir,
		"GEMINI_API_KEY":  geminiPlaceholder,
	}
}

// SessionID implements Harness: the `init` event carries the session id.
func (Gemini) SessionID(line []byte) string {
	if !bytes.Contains(line, []byte(`"session_id"`)) {
		return ""
	}
	var event struct {
		Type      string `json:"type"`
		SessionID string `json:"session_id"`
	}
	if json.Unmarshal(line, &event) != nil || event.Type != "init" {
		return ""
	}
	return event.SessionID
}

// LimitFrom implements Harness: a quota or rate-limit refusal arrives as an error
// event or an error result whose text is the API's. Gemini says nothing machine
// readable about when it lifts, so the reset is zero.
func (Gemini) LimitFrom(line []byte) (bool, time.Time) {
	if !bytes.Contains(line, []byte("error")) {
		return false, time.Time{}
	}
	var event struct {
		Type    string `json:"type"`
		Status  string `json:"status"`
		Message string `json:"message"`
		Error   struct {
			Message string `json:"message"`
		} `json:"error"`
	}
	if json.Unmarshal(line, &event) != nil {
		return false, time.Time{}
	}
	if event.Type != errorEvent &&
		(event.Type != "result" || event.Status != errorEvent) {
		return false, time.Time{}
	}
	text := strings.ToLower(event.Message + " " + event.Error.Message)
	// The texts of the API's quota and rate-limit errors.
	marks := []string{
		"resource_exhausted", "quota exceeded", "exceeded your current quota",
		"rate limit", "too many requests", "usage limit",
	}
	for _, mark := range marks {
		if strings.Contains(text, mark) {
			return true, time.Time{}
		}
	}
	return false, time.Time{}
}

// TransientError implements Harness.
func (Gemini) TransientError(line []byte) bool { return transportErrorLine(line) }
