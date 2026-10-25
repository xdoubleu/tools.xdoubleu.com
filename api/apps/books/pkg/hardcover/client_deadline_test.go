package hardcover_test

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"tools.xdoubleu.com/apps/books/pkg/hardcover"
	"tools.xdoubleu.com/internal/logging"
)

// A caller with a deadline (a request) gets fewer attempts, so a 5xx streak
// gives up instead of retrying past its budget.
func TestGetByISBN_WithDeadline_FewerAttempts(t *testing.T) {
	var attempts atomic.Int32
	cleanup := buildServer(http.HandlerFunc(
		func(w http.ResponseWriter, _ *http.Request) {
			attempts.Add(1)
			w.WriteHeader(http.StatusInternalServerError)
		},
	))
	defer cleanup()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	c := hardcover.New(logging.NewNopLogger(), "token")
	_, err := c.GetByISBN(ctx, "9780000000000")
	require.Error(t, err)
	assert.Equal(t, int32(2), attempts.Load())
}

// A deadline with room for every attempt (a job's run deadline) keeps the
// full retry schedule.
func TestGetByISBN_WithLongDeadline_AllAttempts(t *testing.T) {
	var attempts atomic.Int32
	cleanup := buildServer(http.HandlerFunc(
		func(w http.ResponseWriter, _ *http.Request) {
			attempts.Add(1)
			w.WriteHeader(http.StatusInternalServerError)
		},
	))
	defer cleanup()

	ctx, cancel := context.WithTimeout(context.Background(), time.Hour)
	defer cancel()
	c := hardcover.New(logging.NewNopLogger(), "token")
	_, err := c.GetByISBN(ctx, "9780000000000")
	require.Error(t, err)
	assert.Equal(t, int32(4), attempts.Load())
}

// Each attempt gets a share of the deadline, so a stalled first attempt
// leaves time for the retry.
func TestGetByISBN_WithDeadline_RetriesAfterStall(t *testing.T) {
	var attempts atomic.Int32
	cleanup := buildServer(http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			if attempts.Add(1) == 1 {
				// Drained, so the server notices the client giving up.
				_, _ = io.Copy(io.Discard, r.Body)
				<-r.Context().Done()
				return
			}
			_, _ = w.Write(json.RawMessage(`{"data":{"editions":[]}}`))
		},
	))
	defer cleanup()

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	c := hardcover.New(logging.NewNopLogger(), "token")
	_, err := c.GetByISBN(ctx, "9780000000000")
	require.ErrorIs(t, err, hardcover.ErrNotFound)
	assert.Equal(t, int32(2), attempts.Load())
}
