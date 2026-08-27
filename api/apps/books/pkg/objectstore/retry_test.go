package objectstore_test

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"tools.xdoubleu.com/apps/books/pkg/objectstore"
)

// countingClient wraps FakeClient's Delete, failing the first failUntil
// calls before succeeding (or always failing when failUntil is negative).
type countingClient struct {
	*objectstore.FakeClient
	calls     atomic.Int32
	failUntil int32
	deleteErr error
}

func newCountingClient(failUntil int32) *countingClient {
	//nolint:exhaustruct // calls is zero-value-ready (atomic.Int32)
	return &countingClient{
		FakeClient: objectstore.NewFake(),
		failUntil:  failUntil,
		deleteErr:  errors.New("transient delete failure"),
	}
}

func (c *countingClient) Delete(ctx context.Context, key string) error {
	n := c.calls.Add(1)
	if c.failUntil < 0 || n <= c.failUntil {
		return c.deleteErr
	}
	return c.FakeClient.Delete(ctx, key)
}

func TestDeleteWithRetry_SucceedsAfterTransientFailures(t *testing.T) {
	objectstore.SetBackoffBase(time.Millisecond)
	t.Cleanup(func() { objectstore.SetBackoffBase(500 * time.Millisecond) })

	client := newCountingClient(2)
	err := objectstore.DeleteWithRetry(t.Context(), client, "books/x/y.epub")

	require.NoError(t, err)
	assert.Equal(t, int32(3), client.calls.Load())
}

func TestDeleteWithRetry_ReturnsLastErrorAfterExhaustingAttempts(t *testing.T) {
	objectstore.SetBackoffBase(time.Millisecond)
	t.Cleanup(func() { objectstore.SetBackoffBase(500 * time.Millisecond) })

	client := newCountingClient(-1)
	err := objectstore.DeleteWithRetry(t.Context(), client, "books/x/y.epub")

	require.Error(t, err)
	assert.Equal(t, int32(4), client.calls.Load())
}

func TestDeleteWithRetry_SucceedsFirstTry(t *testing.T) {
	client := newCountingClient(0)
	err := objectstore.DeleteWithRetry(t.Context(), client, "books/x/y.epub")

	require.NoError(t, err)
	assert.Equal(t, int32(1), client.calls.Load())
}

func TestDeleteWithRetry_ContextCanceledBailsImmediately(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	cancel()

	//nolint:exhaustruct // calls is zero-value-ready (atomic.Int32)
	client := &countingClient{
		FakeClient: objectstore.NewFake(),
		failUntil:  -1,
		deleteErr:  context.Canceled,
	}
	err := objectstore.DeleteWithRetry(ctx, client, "books/x/y.epub")

	require.ErrorIs(t, err, context.Canceled)
	assert.Equal(t, int32(1), client.calls.Load())
}
