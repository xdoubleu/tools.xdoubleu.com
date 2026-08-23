package observability

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"tools.xdoubleu.com/internal/models"
)

type fakeLogInserter struct {
	mu       sync.Mutex
	inserted []models.LogEntry
	err      error
}

func (f *fakeLogInserter) Insert(_ context.Context, entry models.LogEntry) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.err != nil {
		return f.err
	}
	f.inserted = append(f.inserted, entry)
	return nil
}

//nolint:exhaustruct //mu and inserted start zero-valued on purpose
func newFakeLogInserter() *fakeLogInserter {
	return &fakeLogInserter{}
}

//nolint:exhaustruct //mu and inserted start zero-valued on purpose
func newFakeLogInserterErr(err error) *fakeLogInserter {
	return &fakeLogInserter{err: err}
}

func TestLogRepoHandler_Handle_InsertsAndForwards(t *testing.T) {
	var buf bytes.Buffer
	next := slog.NewTextHandler(&buf, nil)
	inserter := newFakeLogInserter()
	handler := NewLogRepoHandler(next, inserter)

	logger := slog.New(handler)
	logger.Info("something happened", "key", "value")

	require.Len(t, inserter.inserted, 1)
	entry := inserter.inserted[0]
	assert.Equal(t, "api", entry.Source)
	assert.Equal(t, slog.LevelInfo.String(), entry.Level)
	assert.Equal(t, "something happened", entry.Message)
	assert.Contains(t, string(entry.AttrsJSON), `"key":"value"`)
	assert.Contains(t, buf.String(), "something happened")
}

func TestLogRepoHandler_Handle_NoAttrsOmitsJSON(t *testing.T) {
	var buf bytes.Buffer
	next := slog.NewTextHandler(&buf, nil)
	inserter := newFakeLogInserter()
	handler := NewLogRepoHandler(next, inserter)

	logger := slog.New(handler)
	logger.Info("no attrs here")

	require.Len(t, inserter.inserted, 1)
	assert.Nil(t, inserter.inserted[0].AttrsJSON)
}

func TestLogRepoHandler_Handle_InsertFailureStillForwards(t *testing.T) {
	var buf bytes.Buffer
	next := slog.NewTextHandler(&buf, nil)
	inserter := newFakeLogInserterErr(errors.New("db down"))
	handler := NewLogRepoHandler(next, inserter)

	logger := slog.New(handler)
	logger.Error("db is unreachable")

	assert.Contains(t, buf.String(), "db is unreachable")
}

func TestLogRepoHandler_Enabled_DelegatesToNext(t *testing.T) {
	//nolint:exhaustruct //AddSource/ReplaceAttr default to zero value
	opts := &slog.HandlerOptions{Level: slog.LevelWarn}
	next := slog.NewTextHandler(&bytes.Buffer{}, opts)
	handler := NewLogRepoHandler(next, newFakeLogInserter())

	assert.False(t, handler.Enabled(context.Background(), slog.LevelInfo))
	assert.True(t, handler.Enabled(context.Background(), slog.LevelWarn))
}

func TestLogRepoHandler_WithAttrs_PreservesInserter(t *testing.T) {
	var buf bytes.Buffer
	next := slog.NewTextHandler(&buf, nil)
	inserter := newFakeLogInserter()
	handler := NewLogRepoHandler(next, inserter)

	// Bound via WithAttrs, "component" lives in the handler's own state, not
	// the record itself, so it won't reach insertBestEffort's JSON — this
	// test only asserts the wrapped handler still routes through to the
	// inserter and to next.
	withAttrs := handler.WithAttrs([]slog.Attr{slog.String("component", "test")})
	logger := slog.New(withAttrs)
	logger.Info("with component attr")

	require.Len(t, inserter.inserted, 1)
	assert.Equal(t, "with component attr", inserter.inserted[0].Message)
	assert.Contains(t, buf.String(), "component=test")
}

func TestLogRepoHandler_WithGroup_PreservesInserter(t *testing.T) {
	var buf bytes.Buffer
	next := slog.NewTextHandler(&buf, nil)
	inserter := newFakeLogInserter()
	handler := NewLogRepoHandler(next, inserter)

	grouped := handler.WithGroup("mygroup")
	logger := slog.New(grouped)
	logger.Info("grouped message")

	require.Len(t, inserter.inserted, 1)
	assert.Equal(t, "grouped message", inserter.inserted[0].Message)
}
