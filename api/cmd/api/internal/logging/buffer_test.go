package logging_test

import (
	"context"
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"tools.xdoubleu.com/cmd/api/internal/logging"
)

func TestUserLogBuffer_Record_And_Get(t *testing.T) {
	buf := logging.NewUserLogBuffer(3)
	buf.Record("user1", logging.LogEntry{
		Time:    time.Now(),
		Level:   "INFO",
		Message: "msg1",
	})

	entries := buf.Get("user1")
	require.Len(t, entries, 1)
	assert.Equal(t, "msg1", entries[0].Message)
}

func TestUserLogBuffer_MaxSize(t *testing.T) {
	buf := logging.NewUserLogBuffer(2)
	buf.Record("user1", logging.LogEntry{Time: time.Now(), Level: "INFO", Message: "a"})
	buf.Record("user1", logging.LogEntry{Time: time.Now(), Level: "INFO", Message: "b"})
	buf.Record("user1", logging.LogEntry{Time: time.Now(), Level: "INFO", Message: "c"})

	entries := buf.Get("user1")
	require.Len(t, entries, 2)
	assert.Equal(t, "b", entries[0].Message)
	assert.Equal(t, "c", entries[1].Message)
}

func TestUserLogBuffer_Get_Empty(t *testing.T) {
	buf := logging.NewUserLogBuffer(10)
	entries := buf.Get("unknown-user")
	assert.Empty(t, entries)
}

func TestUserLogHandler_WithAttrs(t *testing.T) {
	buf := logging.NewUserLogBuffer(10)
	inner := slog.NewTextHandler(io.Discard, nil)
	handler := logging.NewUserLogHandler(inner, buf)

	h2 := handler.WithAttrs([]slog.Attr{slog.String("key", "val")})
	assert.NotNil(t, h2)
}

func TestUserLogHandler_WithGroup(t *testing.T) {
	buf := logging.NewUserLogBuffer(10)
	inner := slog.NewTextHandler(io.Discard, nil)
	handler := logging.NewUserLogHandler(inner, buf)

	h2 := handler.WithGroup("mygroup")
	assert.NotNil(t, h2)
}

func TestUserLogHandler_Handle_WithUserID(t *testing.T) {
	buf := logging.NewUserLogBuffer(10)
	inner := slog.NewTextHandler(io.Discard, nil)
	handler := logging.NewUserLogHandler(inner, buf)

	ctx := context.WithValue(
		context.Background(),
		logging.UserIDContextKey,
		"test-user",
	)

	rec := slog.NewRecord(time.Now(), slog.LevelInfo, "test message", 0)
	err := handler.Handle(ctx, rec)
	require.NoError(t, err)

	entries := buf.Get("test-user")
	require.Len(t, entries, 1)
	assert.Equal(t, "test message", entries[0].Message)
	assert.Equal(t, "INFO", entries[0].Level)
}

func TestUserLogHandler_Handle_WithoutUserID(t *testing.T) {
	buf := logging.NewUserLogBuffer(10)
	inner := slog.NewTextHandler(io.Discard, nil)
	handler := logging.NewUserLogHandler(inner, buf)

	rec := slog.NewRecord(time.Now(), slog.LevelInfo, "anonymous message", 0)
	err := handler.Handle(context.Background(), rec)
	require.NoError(t, err)

	// No user ID in context — nothing should be buffered.
	entries := buf.Get("")
	assert.Empty(t, entries)
}

func TestUserLogHandler_Enabled(t *testing.T) {
	buf := logging.NewUserLogBuffer(10)
	inner := slog.NewTextHandler(io.Discard, nil)
	handler := logging.NewUserLogHandler(inner, buf)

	assert.True(t, handler.Enabled(context.Background(), slog.LevelInfo))
}
