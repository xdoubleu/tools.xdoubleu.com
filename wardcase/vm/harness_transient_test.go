package vm_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"tools.xdoubleu.com/wardcase/vm"
)

// These lines are the shapes the job VM's harnesses wrote when their provider was
// unreachable. The first is the real event from T-731's session (2026-11-21).

func TestAnUnreachableProviderIsATransientError(t *testing.T) {
	for name, line := range map[string]string{
		"no route": `{"type":"assistant","error":"server_error",` +
			`"message":{"role":"assistant","content":[{"type":"text","text":` +
			`"API Error: No internet route — check your connection or VPN ` +
			`(EHOSTUNREACH)"}]},"session_id":"s1"}`,
		"connection error": `{"type":"result","subtype":"error_during_execution",` +
			`"is_error":true,"result":"API Error: Connection error.",` +
			`"session_id":"s1"}`,
		"reset": `{"type":"result","is_error":true,` +
			`"result":"API Error: read ECONNRESET","session_id":"s1"}`,
		"timeout": `{"type":"result","is_error":true,` +
			`"result":"API Error: Request timed out.","session_id":"s1"}`,
	} {
		assert.True(t, vm.ClaudeCode{}.TransientError([]byte(line)), name)
		assert.True(t, vm.OpenCode{Model: "a/b"}.TransientError([]byte(line)), name)
	}
}

func TestWorkAndLimitsAreNotTransientErrors(t *testing.T) {
	for name, line := range map[string]string{
		"usage limit": `{"type":"result","is_error":true,` +
			`"result":"You've hit your session limit · resets 12pm (UTC)",` +
			`"session_id":"s1"}`,
		"agent prose": `{"type":"assistant","message":{"role":"assistant",` +
			`"content":[{"type":"text","text":"I fixed the connection error."}]},` +
			`"session_id":"s1"}`,
		"model error": `{"type":"result","is_error":true,` +
			`"result":"API Error: 400 invalid_request_error","session_id":"s1"}`,
		"clean line": `{"type":"assistant","message":{"role":"assistant"},` +
			`"session_id":"s1"}`,
	} {
		assert.False(t, vm.ClaudeCode{}.TransientError([]byte(line)), name)
	}
}
