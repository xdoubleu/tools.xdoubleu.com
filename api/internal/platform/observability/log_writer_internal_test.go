package observability

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func discardText() slog.Handler { return slog.NewTextHandler(&bytes.Buffer{}, nil) }

func droppedCount(reason string) float64 {
	return testutil.ToFloat64(logEntriesDropped.WithLabelValues(reason))
}

func waitStarted(t *testing.T, inserter *fakeLogInserter) {
	t.Helper()
	select {
	case <-inserter.started:
	case <-time.After(5 * time.Second):
		t.Fatal("writer never called InsertBatch")
	}
}

func TestLogRepoHandler_Handle_DoesNotBlockOnStuckDB(t *testing.T) {
	inserter := newBlockingLogInserter()
	handler := newTestLogHandler(t, discardText(), inserter, logWriterOptions{
		queueSize: 4, batchSize: 1, flushInterval: time.Hour, insertTimeout: time.Hour,
	})
	logger := slog.New(handler)
	defer close(inserter.block)

	done := make(chan struct{})
	go func() {
		for range 100 {
			logger.Info("while the db is stuck")
		}
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("log calls blocked on a stuck insert")
	}
}

func TestLogWriter_BatchesUpToBatchSize(t *testing.T) {
	inserter := newFakeLogInserter()
	handler := newTestLogHandler(t, discardText(), inserter, logWriterOptions{
		queueSize: 16, batchSize: 3, flushInterval: time.Hour, insertTimeout: time.Second,
	})
	logger := slog.New(handler)

	for range 5 {
		logger.Info("batched")
	}
	assert.Eventually(t, func() bool {
		return assert.ObjectsAreEqual([]int{3}, inserter.batchSizes())
	}, 5*time.Second, 5*time.Millisecond)

	closeLogHandler(t, handler)
	assert.Equal(t, []int{3, 2}, inserter.batchSizes())
}

func TestLogWriter_FlushesPartialBatchOnInterval(t *testing.T) {
	inserter := newFakeLogInserter()
	handler := newTestLogHandler(t, discardText(), inserter, logWriterOptions{
		queueSize: 16, batchSize: 100, flushInterval: 10 * time.Millisecond,
		insertTimeout: time.Second,
	})

	slog.New(handler).Info("lonely")

	assert.Eventually(t, func() bool {
		return len(inserter.inserted()) == 1
	}, 5*time.Second, 5*time.Millisecond)
}

func TestLogWriter_DropsAndCountsWhenQueueFull(t *testing.T) {
	inserter := newBlockingLogInserter()
	handler := newTestLogHandler(t, discardText(), inserter, logWriterOptions{
		queueSize: 2, batchSize: 1, flushInterval: time.Hour, insertTimeout: time.Hour,
	})
	logger := slog.New(handler)
	before := droppedCount(dropReasonQueueFull)

	logger.Info("first")
	waitStarted(t, inserter) // the writer now holds "first" in a stuck insert
	logger.Info("second")
	logger.Info("third")
	logger.Info("fourth")
	logger.Info("fifth")

	assert.InDelta(t, 2, droppedCount(dropReasonQueueFull)-before, 0)

	close(inserter.block)
	closeLogHandler(t, handler)
	var messages []string
	for _, e := range inserter.inserted() {
		messages = append(messages, e.Message)
	}
	assert.Equal(t, []string{"first", "second", "third"}, messages)
}

func TestLogWriter_CountsFailedInserts(t *testing.T) {
	inserter := newFakeLogInserterErr(errors.New("db down"))
	handler := newTestLogHandler(t, discardText(), inserter, defaultLogWriterOptions())
	before := droppedCount(dropReasonInsertFailed)

	logger := slog.New(handler)
	logger.Info("lost one")
	logger.Info("lost two")
	closeLogHandler(t, handler)

	assert.InDelta(t, 2, droppedCount(dropReasonInsertFailed)-before, 0)
	assert.Equal(t, 1, inserter.callCount(), "an unreachable DB isn't retried")
}

func TestLogWriter_RejectedBatchRetriesRowByRow(t *testing.T) {
	inserter := newFakeLogInserter()
	inserter.rejects = "poison"
	handler := newTestLogHandler(t, discardText(), inserter, defaultLogWriterOptions())
	before := droppedCount(dropReasonInsertFailed)

	logger := slog.New(handler)
	logger.Info("ok one")
	logger.Info("poison")
	logger.Info("ok two")
	closeLogHandler(t, handler)

	var messages []string
	for _, e := range inserter.inserted() {
		messages = append(messages, e.Message)
	}
	assert.Equal(t, []string{"ok one", "ok two"}, messages)
	assert.InDelta(t, 1, droppedCount(dropReasonInsertFailed)-before, 0)
}

func TestLogWriter_CloseIsBoundedByContext(t *testing.T) {
	inserter := newBlockingLogInserter()
	handler := newLogRepoHandler(discardText(), inserter, logWriterOptions{
		queueSize: 4, batchSize: 1, flushInterval: time.Hour, insertTimeout: time.Hour,
	})
	logger := slog.New(handler)
	logger.Info("stuck")
	waitStarted(t, inserter)
	logger.Info("queued behind it")

	ctx, cancel := context.WithTimeout(t.Context(), 50*time.Millisecond)
	defer cancel()
	start := time.Now()
	err := handler.Close(ctx)

	require.ErrorIs(t, err, context.DeadlineExceeded)
	assert.Less(t, time.Since(start), 2*time.Second)
	select {
	case <-handler.writer.done:
	case <-time.After(5 * time.Second):
		t.Fatal("writer goroutine outlived Close's deadline")
	}
}

func TestLogWriter_CloseIsIdempotentAndStopsQueueing(t *testing.T) {
	inserter := newFakeLogInserter()
	handler := newTestLogHandler(t, discardText(), inserter, defaultLogWriterOptions())
	logger := slog.New(handler)

	logger.Info("before close")
	closeLogHandler(t, handler)
	logger.Info("after close")
	closeLogHandler(t, handler)

	inserted := inserter.inserted()
	require.Len(t, inserted, 1)
	assert.Equal(t, "before close", inserted[0].Message)
}
