package runner_test

import (
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"tools.xdoubleu.com/factory-runner/internal/runner"
)

// These lines are modelled on the shapes the Claude Code 2.1.285 binary contains
// (a `rate_limit_event` with `rate_limit_info`, and error results reading "usage
// limit reached" or "You've hit your ... limit"). No real limit hit has been seen
// on the wire, so they are UNVERIFIED; replace them with the first real capture.

func epochIn(d time.Duration) int64 { return time.Now().Add(d).Unix() }

func TestARejectedRateLimitEventIsALimitWithItsResetTime(t *testing.T) {
	reset := epochIn(3 * time.Hour)
	line := fmt.Sprintf(`{"type":"rate_limit_event","rate_limit_info":`+
		`{"status":"rejected","resetsAt":%d,"rateLimitType":"five_hour",`+
		`"isUsingOverage":false},"uuid":"u1","session_id":"s1"}`, reset)

	limited, at := runner.ClaudeCode{}.LimitFrom([]byte(line))

	assert.True(t, limited)
	assert.Equal(t, time.Unix(reset, 0).UTC(), at)
}

func TestAllowedAndWarningRateLimitEventsAreNotAStop(t *testing.T) {
	for _, status := range []string{"allowed", "allowed_warning", ""} {
		line := fmt.Sprintf(`{"type":"rate_limit_event","rate_limit_info":{"status":%q,`+
			`"resetsAt":%d,"rateLimitType":"seven_day"}}`, status, epochIn(time.Hour))
		limited, _ := runner.ClaudeCode{}.LimitFrom([]byte(line))
		assert.False(t, limited, status)
	}
}

func TestAnErrorResultSayingTheLimitWasReachedIsALimit(t *testing.T) {
	for name, tc := range map[string]struct {
		result string
		wantAt bool
	}{
		"usage limit":    {"Claude AI usage limit reached", false},
		"hit your limit": {"You've hit your limit · resets 9pm", false},
		"curly quote":    {"You’ve hit your weekly limit", false},
		"epoch form": {
			fmt.Sprintf("Claude AI usage limit reached|%d", epochIn(2*time.Hour)),
			true,
		},
	} {
		line := fmt.Sprintf(`{"type":"result","subtype":"success","is_error":true,`+
			`"result":%q,"session_id":"s1"}`, tc.result)
		limited, at := runner.ClaudeCode{}.LimitFrom([]byte(line))
		assert.True(t, limited, name)
		assert.Equal(t, tc.wantAt, !at.IsZero(), name)
	}
}

func TestOrdinaryOutputIsNotALimit(t *testing.T) {
	for name, line := range map[string]string{
		"a result that is not an error": `{"type":"result","is_error":false,` +
			`"result":"I hit the usage limit reached text in a file"}`,
		"an error about something else": `{"type":"result","is_error":true,` +
			`"result":"Tool failed: permission denied"}`,
		"an assistant message mentioning it": `{"type":"assistant","message":{"content":` +
			`[{"type":"text","text":"usage limit reached"}]}}`,
		"an unrelated event": `{"type":"system","subtype":"init","session_id":"s1"}`,
		"not json":           `rate_limit_event: usage limit reached`,
		"broken json": `{"type":"rate_limit_event","rate_limit_info":` +
			`{"status":"rejected"`,
		"a user event":        `{"type":"user","message":{"content":"is_error"}}`,
		"empty":               ``,
		"a result with limit": `{"type":"result","is_error":true,"result":"limit"}`,
	} {
		limited, _ := runner.ClaudeCode{}.LimitFrom([]byte(line))
		assert.False(t, limited, name)
	}
}

func TestAResetTimeThatCannotBeRightIsLeftUnsaid(t *testing.T) {
	for name, secs := range map[string]int64{
		"zero": 0, "negative": -5, "in the past": epochIn(-time.Hour),
		"months ahead": epochIn(90 * 24 * time.Hour),
	} {
		line := fmt.Sprintf(`{"type":"rate_limit_event","rate_limit_info":`+
			`{"status":"rejected","resetsAt":%d}}`, secs)
		limited, at := runner.ClaudeCode{}.LimitFrom([]byte(line))
		assert.True(t, limited, name)
		assert.True(t, at.IsZero(), name)
	}
}

func TestOpenCodeNeverReportsAClaudeLimit(t *testing.T) {
	line := `{"type":"rate_limit_event","rate_limit_info":{"status":"rejected"}}`
	limited, at := runner.OpenCode{Model: "a/b"}.LimitFrom([]byte(line))
	require.False(t, limited)
	assert.True(t, at.IsZero())
}
