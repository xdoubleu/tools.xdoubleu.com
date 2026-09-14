package anthropicadmin

import (
	"context"
	"errors"
	"net/url"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestBackoffDelay_CapsAtBackoffCap(t *testing.T) {
	originalBase, originalCap := backoffBase, backoffCap
	t.Cleanup(func() { backoffBase, backoffCap = originalBase, originalCap })

	backoffBase = 10 * time.Second
	backoffCap = 15 * time.Second

	assert.Equal(t, 10*time.Second, backoffDelay(0))
	assert.Equal(t, 15*time.Second, backoffDelay(3))
}

func TestIsTransientErr(t *testing.T) {
	t.Run("context.Canceled is not transient", func(t *testing.T) {
		assert.False(t, isTransientErr(context.Canceled))
	})
	t.Run("context.DeadlineExceeded is transient", func(t *testing.T) {
		assert.True(t, isTransientErr(context.DeadlineExceeded))
	})
	t.Run("timeout url.Error is transient", func(t *testing.T) {
		err := &url.Error{Op: "Get", URL: "http://example.com", Err: stubTimeoutError{}}
		assert.True(t, isTransientErr(err))
	})
	t.Run("non-timeout url.Error is not transient", func(t *testing.T) {
		err := &url.Error{Op: "Get", URL: "http://example.com", Err: errors.New("boom")}
		assert.False(t, isTransientErr(err))
	})
	t.Run("plain error is not transient", func(t *testing.T) {
		assert.False(t, isTransientErr(errors.New("boom")))
	})
}

// stubTimeoutError implements net.Error's Timeout() method, the shape url.Error's
// own Timeout() delegates to.
type stubTimeoutError struct{}

func (stubTimeoutError) Error() string   { return "timeout" }
func (stubTimeoutError) Timeout() bool   { return true }
func (stubTimeoutError) Temporary() bool { return true }
