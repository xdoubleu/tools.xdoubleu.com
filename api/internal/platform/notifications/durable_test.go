package notifications_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"tools.xdoubleu.com/internal/kit/logging"
	"tools.xdoubleu.com/internal/platform/mailer"
	"tools.xdoubleu.com/internal/platform/notifications"
)

func TestQueuedSendIsDeliveredByAFreshServiceOverTheSameStore(t *testing.T) {
	t.Parallel()

	store := notifications.NewMemStore()

	// A previous process queued the mail but was gone before it sent.
	_, err := store.Enqueue(t.Context(), "to@example.com", "subject", "body")
	require.NoError(t, err)

	mail := newFakeMailer(nil)
	second := notifications.NewDurable(t.Context(), logging.NewNopLogger(), mail, store)
	require.NoError(t, second.Shutdown(t.Context()))

	require.Len(t, mail.sentMails(), 1)
	assert.Equal(t, "to@example.com", mail.sentMails()[0].to)
}

func TestFailingSendRetriesThenStopsAtTheCap(t *testing.T) {
	t.Parallel()

	mail := newFakeMailer(assert.AnError)
	svc := notifications.NewDurable(
		t.Context(), logging.NewNopLogger(), mail, notifications.NewMemStore(),
		notifications.WithRetry(3, time.Millisecond),
	)

	var gotErr error
	calls := 0
	svc.EnqueueEmail("subject", "body", func(_ context.Context, err error) error {
		calls++
		gotErr = err
		return nil
	})
	svc.WaitUntilDone()

	assert.Equal(t, 3, mail.attempts())
	assert.Equal(t, 1, calls, "OnResult runs once, on the final outcome")
	require.ErrorIs(t, gotErr, assert.AnError)
}

func TestRetryingSendStaysPendingAndJobIdentity(t *testing.T) {
	t.Parallel()

	svc := notifications.NewDurable(
		t.Context(), logging.NewNopLogger(), newFakeMailer(assert.AnError),
		notifications.NewMemStore(), notifications.WithRetry(2, time.Hour),
	)
	svc.EnqueueEmail("subject", "body", nil)
	require.Eventually(t, func() bool {
		d, _ := svc.Depth(t.Context())
		return d.Pending == 1
	}, time.Second, time.Millisecond)

	assert.Equal(t, "notification_outbox", svc.ID())
	assert.Positive(t, svc.RunEvery())
}

func TestNotConfiguredIsNotRetried(t *testing.T) {
	t.Parallel()

	mail := newFakeMailer(mailer.ErrNotConfigured)
	svc := notifications.NewDurable(
		t.Context(), logging.NewNopLogger(), mail, notifications.NewMemStore(),
		notifications.WithRetry(3, time.Millisecond),
	)
	svc.EnqueueEmail("subject", "body", nil)
	svc.WaitUntilDone()

	assert.Equal(t, 1, mail.attempts())
}
