package observability

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"slices"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"tools.xdoubleu.com/internal/models"
)

// fakeLogInserter records each batch. With block set, every call waits until
// block closes or ctx ends; started then receives once per call.
type fakeLogInserter struct {
	mu      sync.Mutex
	batches [][]models.LogEntry
	calls   int
	err     error
	// rejects is the server error for any batch holding this message.
	rejects string
	block   chan struct{}
	started chan struct{}
}

func (f *fakeLogInserter) InsertBatch(
	ctx context.Context,
	entries []models.LogEntry,
) error {
	if f.block != nil {
		select {
		case f.started <- struct{}{}:
		default:
		}
		select {
		case <-f.block:
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls++
	if f.err != nil {
		return f.err
	}
	if f.rejects != "" && slices.ContainsFunc(entries, func(e models.LogEntry) bool {
		return e.Message == f.rejects
	}) {
		//nolint:exhaustruct //only the code matters here
		return &pgconn.PgError{Code: "22P05"}
	}
	f.batches = append(f.batches, slices.Clone(entries))
	return nil
}

func (f *fakeLogInserter) callCount() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.calls
}

func (f *fakeLogInserter) inserted() []models.LogEntry {
	f.mu.Lock()
	defer f.mu.Unlock()
	return slices.Concat(f.batches...)
}

func (f *fakeLogInserter) batchSizes() []int {
	f.mu.Lock()
	defer f.mu.Unlock()
	sizes := make([]int, len(f.batches))
	for i, b := range f.batches {
		sizes[i] = len(b)
	}
	return sizes
}

//nolint:exhaustruct //mu and batches start zero-valued on purpose
func newFakeLogInserter() *fakeLogInserter {
	return &fakeLogInserter{}
}

//nolint:exhaustruct //mu and batches start zero-valued on purpose
func newFakeLogInserterErr(err error) *fakeLogInserter {
	return &fakeLogInserter{err: err}
}

// newBlockingLogInserter's calls wait until block is closed.
//
//nolint:exhaustruct //mu and batches start zero-valued on purpose
func newBlockingLogInserter() *fakeLogInserter {
	return &fakeLogInserter{
		block:   make(chan struct{}),
		started: make(chan struct{}, 100),
	}
}

// newTestLogHandler closes the handler at cleanup so its writer never leaks.
func newTestLogHandler(
	t *testing.T,
	next slog.Handler,
	inserter logBatchInserter,
	opts logWriterOptions,
) *LogRepoHandler {
	t.Helper()
	handler := newLogRepoHandler(next, inserter, opts)
	t.Cleanup(func() { closeLogHandler(t, handler) })
	return handler
}

func closeLogHandler(t *testing.T, handler *LogRepoHandler) {
	t.Helper()
	// t.Context() is already canceled when cleanups run.
	ctx, cancel := context.WithTimeout(
		context.WithoutCancel(t.Context()), 5*time.Second,
	)
	defer cancel()
	require.NoError(t, handler.Close(ctx))
}

func TestLogRepoHandler_Handle_InsertsAndForwards(t *testing.T) {
	var buf bytes.Buffer
	next := slog.NewTextHandler(&buf, nil)
	inserter := newFakeLogInserter()
	handler := NewLogRepoHandler(next, inserter)

	logger := slog.New(handler)
	logger.Info("something happened", "key", "value")
	closeLogHandler(t, handler)

	inserted := inserter.inserted()
	require.Len(t, inserted, 1)
	entry := inserted[0]
	assert.Equal(t, "api", entry.Source)
	assert.Equal(t, slog.LevelInfo.String(), entry.Level)
	assert.Equal(t, "something happened", entry.Message)
	assert.Contains(t, string(entry.AttrsJSON), `"key":"value"`)
	assert.Contains(t, buf.String(), "something happened")
}

func TestLogRepoHandler_Handle_NoAttrsOmitsJSON(t *testing.T) {
	inserter := newFakeLogInserter()
	handler := newTestLogHandler(t, slog.NewTextHandler(&bytes.Buffer{}, nil),
		inserter, defaultLogWriterOptions())

	slog.New(handler).Info("no attrs here")
	closeLogHandler(t, handler)

	inserted := inserter.inserted()
	require.Len(t, inserted, 1)
	assert.Nil(t, inserted[0].AttrsJSON)
}

func TestLogRepoHandler_Handle_InsertFailureStillForwards(t *testing.T) {
	var buf bytes.Buffer
	next := slog.NewTextHandler(&buf, nil)
	inserter := newFakeLogInserterErr(errors.New("db down"))
	handler := newTestLogHandler(t, next, inserter, defaultLogWriterOptions())

	slog.New(handler).Error("db is unreachable")

	assert.Contains(t, buf.String(), "db is unreachable")
}

func TestLogRepoHandler_Enabled_DelegatesToNext(t *testing.T) {
	//nolint:exhaustruct //AddSource/ReplaceAttr default to zero value
	opts := &slog.HandlerOptions{Level: slog.LevelWarn}
	next := slog.NewTextHandler(&bytes.Buffer{}, opts)
	handler := newTestLogHandler(t, next, newFakeLogInserter(),
		defaultLogWriterOptions())

	assert.False(t, handler.Enabled(context.Background(), slog.LevelInfo))
	assert.True(t, handler.Enabled(context.Background(), slog.LevelWarn))
}

func TestLogRepoHandler_WithAttrs_PreservesInserter(t *testing.T) {
	var buf bytes.Buffer
	next := slog.NewTextHandler(&buf, nil)
	inserter := newFakeLogInserter()
	handler := newTestLogHandler(t, next, inserter, defaultLogWriterOptions())

	// WithAttrs state isn't part of the record, so this only checks routing.
	withAttrs := handler.WithAttrs([]slog.Attr{slog.String("component", "test")})
	slog.New(withAttrs).Info("with component attr")
	closeLogHandler(t, handler)

	inserted := inserter.inserted()
	require.Len(t, inserted, 1)
	assert.Equal(t, "with component attr", inserted[0].Message)
	assert.Contains(t, buf.String(), "component=test")
}

func TestLogRepoHandler_WithGroup_PreservesInserter(t *testing.T) {
	inserter := newFakeLogInserter()
	handler := newTestLogHandler(t, slog.NewTextHandler(&bytes.Buffer{}, nil),
		inserter, defaultLogWriterOptions())

	slog.New(handler.WithGroup("mygroup")).Info("grouped message")
	closeLogHandler(t, handler)

	inserted := inserter.inserted()
	require.Len(t, inserted, 1)
	assert.Equal(t, "grouped message", inserted[0].Message)
}

type redacted struct{}

func (redacted) LogValue() slog.Value { return slog.StringValue("[redacted]") }

func TestLogRepoHandler_Handle_ErrorAndLogValuerAttrsKeepTheirText(t *testing.T) {
	inserter := newFakeLogInserter()
	handler := newTestLogHandler(t, slog.NewTextHandler(&bytes.Buffer{}, nil),
		inserter, defaultLogWriterOptions())

	slog.New(handler).Error(
		"failed",
		"error", errors.New("not valid PEM"),
		"secret", redacted{},
		slog.Group("req", "error", errors.New("nested"), "status", 500),
	)
	closeLogHandler(t, handler)

	inserted := inserter.inserted()
	require.Len(t, inserted, 1)
	assert.JSONEq(
		t,
		`{"error":"not valid PEM","secret":"[redacted]",`+
			`"req":{"error":"nested","status":500}}`,
		string(inserted[0].AttrsJSON),
	)
}
