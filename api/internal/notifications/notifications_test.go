package notifications_test

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"strconv"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"tools.xdoubleu.com/internal/logging"
	"tools.xdoubleu.com/internal/mailer"
	"tools.xdoubleu.com/internal/notifications"
)

type sentMail struct {
	to, subject, body string
}

type fakeMailer struct {
	mu   sync.Mutex
	sent []sentMail
	err  error
}

func (f *fakeMailer) Send(_ context.Context, subject, body string) error {
	return f.record("", subject, body)
}

func (f *fakeMailer) SendTo(_ context.Context, to, subject, body string) error {
	return f.record(to, subject, body)
}

func (f *fakeMailer) record(to, subject, body string) error {
	if f.err != nil {
		return f.err
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	f.sent = append(f.sent, sentMail{to: to, subject: subject, body: body})
	return nil
}

func (f *fakeMailer) sentMails() []sentMail {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]sentMail(nil), f.sent...)
}

//nolint:exhaustruct // mu is a zero-value sync.Mutex
func newFakeMailer(err error) *fakeMailer {
	return &fakeMailer{err: err}
}

func TestEnqueueDeliversAndReportsSuccess(t *testing.T) {
	t.Parallel()

	mail := newFakeMailer(nil)
	svc := notifications.New(t.Context(), logging.NewNopLogger(), mail)

	var gotErr error
	called := make(chan struct{})
	svc.Enqueue("subject", "body", func(_ context.Context, err error) error {
		gotErr = err
		close(called)
		return nil
	})

	<-called
	svc.WaitUntilDone()

	require.NoError(t, gotErr)
	require.Len(t, mail.sentMails(), 1)
	assert.Equal(t, "subject", mail.sentMails()[0].subject)
}

func TestEnqueueToDeliversToRecipient(t *testing.T) {
	t.Parallel()

	mail := newFakeMailer(nil)
	svc := notifications.New(t.Context(), logging.NewNopLogger(), mail)

	called := make(chan struct{})
	svc.EnqueueTo(
		"to@example.com",
		"subject",
		"body",
		func(_ context.Context, _ error) error {
			close(called)
			return nil
		},
	)

	<-called
	svc.WaitUntilDone()

	require.Len(t, mail.sentMails(), 1)
	assert.Equal(t, "to@example.com", mail.sentMails()[0].to)
}

func TestEnqueuePassesThroughErrNotConfigured(t *testing.T) {
	t.Parallel()

	mail := newFakeMailer(mailer.ErrNotConfigured)
	svc := notifications.New(t.Context(), logging.NewNopLogger(), mail)

	var gotErr error
	called := make(chan struct{})
	svc.Enqueue("subject", "body", func(_ context.Context, err error) error {
		gotErr = err
		close(called)
		return nil
	})

	<-called
	svc.WaitUntilDone()

	assert.ErrorIs(t, gotErr, mailer.ErrNotConfigured)
}

func TestEnqueuePassesThroughRealError(t *testing.T) {
	t.Parallel()

	mail := newFakeMailer(assert.AnError)
	svc := notifications.New(t.Context(), logging.NewNopLogger(), mail)

	var gotErr error
	called := make(chan struct{})
	svc.Enqueue("subject", "body", func(_ context.Context, err error) error {
		gotErr = err
		close(called)
		return nil
	})

	<-called
	svc.WaitUntilDone()

	assert.ErrorIs(t, gotErr, assert.AnError)
}

func TestEnqueueNilOnResultDoesNotPanic(t *testing.T) {
	t.Parallel()

	mail := newFakeMailer(nil)
	svc := notifications.New(t.Context(), logging.NewNopLogger(), mail)

	svc.Enqueue("subject", "body", nil)
	svc.WaitUntilDone()

	require.Len(t, mail.sentMails(), 1)
}

func TestEnqueueLogsOnResultError(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	mail := newFakeMailer(nil)
	logger := slog.New(logging.NewBufLogHandler(&buf, nil))
	svc := notifications.New(t.Context(), logger, mail)

	svc.Enqueue("subject", "body", func(_ context.Context, _ error) error {
		return errors.New("onResult failed")
	})
	svc.WaitUntilDone()

	assert.Contains(t, buf.String(), "onResult failed")
}

func TestEnqueueDeliversStrictlyInOrder(t *testing.T) {
	t.Parallel()

	mail := newFakeMailer(nil)
	svc := notifications.New(t.Context(), logging.NewNopLogger(), mail)

	const n = 20
	for i := range n {
		svc.Enqueue(strconv.Itoa(i), "body", nil)
	}
	svc.WaitUntilDone()

	sent := mail.sentMails()
	require.Len(t, sent, n)
	for i := range n {
		assert.Equal(t, strconv.Itoa(i), sent[i].subject)
	}
}
