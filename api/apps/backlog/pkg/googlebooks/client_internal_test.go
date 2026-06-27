package googlebooks

import (
	"context"
	"errors"
	"net/url"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// timeoutNetErr simulates a network timeout error — it is not context.Canceled
// or context.DeadlineExceeded, so isTransientErr must rely on Timeout().
type timeoutNetError struct{}

func (timeoutNetError) Error() string   { return "i/o timeout" }
func (timeoutNetError) Timeout() bool   { return true }
func (timeoutNetError) Temporary() bool { return true }

func TestIsTransientErr_ContextCanceled(t *testing.T) {
	assert.False(t, isTransientErr(context.Canceled))
}

func TestIsTransientErr_ContextDeadlineExceeded(t *testing.T) {
	assert.True(t, isTransientErr(context.DeadlineExceeded))
}

func TestIsTransientErr_URLErrorTimeout(t *testing.T) {
	err := &url.Error{
		Op:  "Get",
		URL: "http://example.com",
		Err: timeoutNetError{},
	}
	assert.True(t, isTransientErr(err))
}

func TestIsTransientErr_URLErrorNonTimeout(t *testing.T) {
	err := &url.Error{
		Op:  "Get",
		URL: "http://example.com",
		Err: errors.New("connection refused"),
	}
	assert.False(t, isTransientErr(err))
}

func TestIsTransientErr_OtherError(t *testing.T) {
	assert.False(t, isTransientErr(errors.New("some other error")))
}

func TestBackoffDelay_CapApplied(t *testing.T) {
	oldBase := backoffBase
	oldCap := backoffCap
	backoffBase = 10 * time.Millisecond
	backoffCap = 15 * time.Millisecond
	defer func() {
		backoffBase = oldBase
		backoffCap = oldCap
	}()

	// attempt=1: 10ms * 2^1 = 20ms > 15ms cap → should return the cap
	d := backoffDelay(1)
	assert.Equal(t, 15*time.Millisecond, d)
}
