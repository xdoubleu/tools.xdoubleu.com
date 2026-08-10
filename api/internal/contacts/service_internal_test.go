package contacts

import (
	"bytes"
	"context"
	"log/slog"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"tools.xdoubleu.com/internal/logging"
	"tools.xdoubleu.com/internal/mailer"
	"tools.xdoubleu.com/internal/notifications"
)

// fakeMailer lets sendContactRequestEmail tests control SendTo's outcome
// without a real Resend server.
type fakeMailer struct {
	err error
}

func (f *fakeMailer) Send(_ context.Context, _, _ string) error {
	return nil
}

func (f *fakeMailer) SendTo(_ context.Context, _, _, _ string) error {
	return f.err
}

// newTestService builds a contactsService with only the fields
// sendContactRequestEmail needs, backed by a buffer-capturing logger so
// tests can assert on (or rule out) its error log.
func newTestService(
	t *testing.T,
	mail mailer.Client,
) (*contactsService, *notifications.Service, *bytes.Buffer) {
	t.Helper()
	var buf bytes.Buffer
	logger := slog.New(logging.NewBufLogHandler(&buf, nil))
	notifSvc := notifications.New(t.Context(), logging.NewNopLogger(), mail)
	//nolint:exhaustruct // repo/auth are unused by sendContactRequestEmail
	svc := &contactsService{
		notifications: notifSvc,
		webURL:        "http://localhost:3000",
		logger:        logger,
	}
	return svc, notifSvc, &buf
}

func TestSendContactRequestEmail_Success_NoLog(t *testing.T) {
	svc, notifSvc, buf := newTestService(t, &fakeMailer{err: nil})

	svc.sendContactRequestEmail("to@example.com", "sender@example.com")
	notifSvc.WaitUntilDone()

	assert.Empty(t, buf.String())
}

func TestSendContactRequestEmail_ErrNotConfigured_NoLog(t *testing.T) {
	svc, notifSvc, buf := newTestService(t, &fakeMailer{err: mailer.ErrNotConfigured})

	svc.sendContactRequestEmail("to@example.com", "sender@example.com")
	notifSvc.WaitUntilDone()

	assert.Empty(t, buf.String())
}

func TestSendContactRequestEmail_RealSendError_Logs(t *testing.T) {
	svc, notifSvc, buf := newTestService(t, &fakeMailer{err: assert.AnError})

	svc.sendContactRequestEmail("to@example.com", "sender@example.com")
	notifSvc.WaitUntilDone()

	require.NotEmpty(t, buf.String())
	assert.Contains(t, buf.String(), "contacts: failed to send contact request email")
}
