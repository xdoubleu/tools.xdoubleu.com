package vm_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"tools.xdoubleu.com/wardcase/vm"
)

func TestGeminiHoldsOnlyAPlaceholderKey(t *testing.T) {
	env := vm.Gemini{}.SessionEnv("/session")
	assert.Equal(t, "/session", env["GEMINI_CLI_HOME"])
	assert.Equal(t, "replaced-by-the-host-proxy", env["GEMINI_API_KEY"])
}

func TestGeminiReadsTheSessionFromTheInitEvent(t *testing.T) {
	h := vm.Gemini{}
	assert.Equal(t, "g1",
		h.SessionID([]byte(`{"type":"init","session_id":"g1","model":"m"}`)))
	assert.Empty(t, h.SessionID([]byte(`{"type":"message","session_id":"g2"}`)))
	assert.Empty(t, h.SessionID([]byte(`{"type":"init"}`)))
	assert.Empty(t, h.SessionID([]byte(`{"session_id":`)))
}

func TestGeminiQuotaErrorsAreALimit(t *testing.T) {
	h := vm.Gemini{}
	for name, line := range map[string]string{
		"error event": `{"type":"error","message":"RESOURCE_EXHAUSTED: Quota exceeded"}`,
		"error result": `{"type":"result","status":"error",` +
			`"error":{"message":"Too many requests"}}`,
	} {
		limited, at := h.LimitFrom([]byte(line))
		assert.True(t, limited, name)
		assert.True(t, at.IsZero(), name)
	}
	for name, line := range map[string]string{
		"agent text":  `{"type":"message","content":"the rate limit error is handled"}`,
		"other error": `{"type":"error","message":"file not found"}`,
		"no error":    `{"type":"result","status":"success"}`,
		"not json":    `error {`,
	} {
		limited, _ := h.LimitFrom([]byte(line))
		assert.False(t, limited, name)
	}
}

func TestGeminiTransportFailuresAreTransient(t *testing.T) {
	h := vm.Gemini{}
	assert.True(t, h.TransientError([]byte(`{"type":"error","message":"fetch failed"}`)))
	assert.False(t, h.TransientError([]byte(`{"type":"message"}`)))
}
