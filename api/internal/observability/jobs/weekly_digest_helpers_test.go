package jobs_test

import (
	"bytes"
	"context"
	"log/slog"
	"testing"

	"tools.xdoubleu.com/internal/logging"
	"tools.xdoubleu.com/internal/notifications"
	"tools.xdoubleu.com/internal/repositories"
)

// fakeMailer records both the subject and body of every send.
type fakeMailer struct {
	sent   []string
	bodies []string
	err    error
}

func (f *fakeMailer) Send(_ context.Context, subject, body string) error {
	if f.err != nil {
		return f.err
	}
	f.sent = append(f.sent, subject)
	f.bodies = append(f.bodies, body)
	return nil
}

func (f *fakeMailer) SendTo(_ context.Context, _, _, _ string) error {
	return nil
}

// alwaysEnabledSettings makes every notification source enabled, for tests
// that aren't exercising the settings gate itself.
type alwaysEnabledSettings struct{}

func (alwaysEnabledSettings) IsEnabled(
	_ context.Context,
	_ repositories.NotificationSource,
) (bool, error) {
	return true, nil
}

// disabledSourceSettings reports enabled state from an explicit per-source
// map, for tests exercising the settings gate.
type disabledSourceSettings struct {
	enabled map[repositories.NotificationSource]bool
}

func (d disabledSourceSettings) IsEnabled(
	_ context.Context,
	source repositories.NotificationSource,
) (bool, error) {
	return d.enabled[source], nil
}

func testLogger() *slog.Logger {
	logger, _ := testLoggerWithBuf()
	return logger
}

func testLoggerWithBuf() (*slog.Logger, *bytes.Buffer) {
	buf := &bytes.Buffer{}
	return slog.New(slog.NewTextHandler(buf, nil)), buf
}

// testNotifications wraps mail in a notifications.Service for the job to
// enqueue onto; deliveries happen on a background worker (issue #923), so
// tests must call WaitUntilDone before asserting on mail state.
func testNotifications(t *testing.T, mail *fakeMailer) *notifications.Service {
	t.Helper()
	return notifications.New(t.Context(), logging.NewNopLogger(), mail)
}
