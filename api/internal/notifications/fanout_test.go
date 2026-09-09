package notifications_test

import (
	"context"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"tools.xdoubleu.com/internal/logging"
	"tools.xdoubleu.com/internal/mailer"
	"tools.xdoubleu.com/internal/notifications"
	"tools.xdoubleu.com/internal/repositories"
	"tools.xdoubleu.com/internal/slack"
)

type fakeSlack struct {
	mu    sync.Mutex
	calls int
	err   error
}

func (f *fakeSlack) Send(_ context.Context, webhookURL, _, _ string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls++
	if webhookURL == "" {
		return slack.ErrNotConfigured
	}
	return f.err
}

func (f *fakeSlack) callCount() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.calls
}

type fakeChannelConfig struct {
	cfg repositories.NotificationChannelConfig
}

func (f fakeChannelConfig) Get(
	_ context.Context,
) (repositories.NotificationChannelConfig, error) {
	return f.cfg, nil
}

func enqueueAndWait(svc *notifications.Service) error {
	var gotErr error
	done := make(chan struct{})
	svc.Enqueue("subject", "body", func(_ context.Context, err error) error {
		gotErr = err
		close(done)
		return nil
	})
	<-done
	svc.WaitUntilDone()
	return gotErr
}

func newFanoutService(
	t *testing.T,
	mail mailer.Client,
	slackClient slack.Client,
	cfg repositories.NotificationChannelConfig,
) *notifications.Service {
	t.Helper()
	return notifications.New(
		t.Context(), logging.NewNopLogger(), mail, slackClient,
		fakeChannelConfig{cfg: cfg},
	)
}

func TestEnqueueEmailModeSendsOnlyEmail(t *testing.T) {
	t.Parallel()

	mail := newFakeMailer(nil)
	sl := &fakeSlack{} //nolint:exhaustruct // zero-value fields intended
	svc := newFanoutService(t, mail, sl, repositories.NotificationChannelConfig{
		ChannelMode:     repositories.ChannelModeEmail,
		SlackWebhookURL: "",
	})

	require.NoError(t, enqueueAndWait(svc))
	assert.Len(t, mail.sentMails(), 1)
	assert.Equal(t, 0, sl.callCount())
}

func TestEnqueueBothModeSendsEmailAndSlack(t *testing.T) {
	t.Parallel()

	mail := newFakeMailer(nil)
	sl := &fakeSlack{} //nolint:exhaustruct // zero-value fields intended
	svc := newFanoutService(t, mail, sl, repositories.NotificationChannelConfig{
		ChannelMode:     repositories.ChannelModeBoth,
		SlackWebhookURL: "https://hooks.slack.test/x",
	})

	require.NoError(t, enqueueAndWait(svc))
	assert.Len(t, mail.sentMails(), 1)
	assert.Equal(t, 1, sl.callCount())
}

func TestEnqueueSlackModeSendsOnlySlackWhenItSucceeds(t *testing.T) {
	t.Parallel()

	mail := newFakeMailer(nil)
	sl := &fakeSlack{} //nolint:exhaustruct // zero-value fields intended
	svc := newFanoutService(t, mail, sl, repositories.NotificationChannelConfig{
		ChannelMode:     repositories.ChannelModeSlack,
		SlackWebhookURL: "https://hooks.slack.test/x",
	})

	require.NoError(t, enqueueAndWait(svc))
	assert.Equal(t, 1, sl.callCount())
	assert.Empty(t, mail.sentMails())
}

func TestEnqueueSlackModeFallsBackToEmailOnSlackError(t *testing.T) {
	t.Parallel()

	mail := newFakeMailer(nil)
	sl := &fakeSlack{err: assert.AnError} //nolint:exhaustruct // mu zero-value
	svc := newFanoutService(t, mail, sl, repositories.NotificationChannelConfig{
		ChannelMode:     repositories.ChannelModeSlack,
		SlackWebhookURL: "https://hooks.slack.test/x",
	})

	require.NoError(t, enqueueAndWait(svc))
	assert.Equal(t, 1, sl.callCount())
	assert.Len(t, mail.sentMails(), 1)
}

func TestEnqueueSlackModeFallsBackToEmailWhenWebhookUnset(t *testing.T) {
	t.Parallel()

	mail := newFakeMailer(nil)
	sl := &fakeSlack{} //nolint:exhaustruct // zero-value fields intended
	svc := newFanoutService(t, mail, sl, repositories.NotificationChannelConfig{
		ChannelMode:     repositories.ChannelModeSlack,
		SlackWebhookURL: "",
	})

	require.NoError(t, enqueueAndWait(svc))
	assert.Len(t, mail.sentMails(), 1)
}

func TestEnqueueReturnsErrNotConfiguredWhenNoChannelDelivers(t *testing.T) {
	t.Parallel()

	mail := newFakeMailer(mailer.ErrNotConfigured)
	sl := &fakeSlack{} //nolint:exhaustruct // zero-value fields intended
	svc := newFanoutService(t, mail, sl, repositories.NotificationChannelConfig{
		ChannelMode:     repositories.ChannelModeSlack,
		SlackWebhookURL: "",
	})

	assert.ErrorIs(t, enqueueAndWait(svc), mailer.ErrNotConfigured)
}

func TestEnqueueEmailBypassesChannelSwitch(t *testing.T) {
	t.Parallel()

	mail := newFakeMailer(nil)
	sl := &fakeSlack{} //nolint:exhaustruct // zero-value fields intended
	svc := newFanoutService(t, mail, sl, repositories.NotificationChannelConfig{
		ChannelMode:     repositories.ChannelModeSlack,
		SlackWebhookURL: "https://hooks.slack.test/x",
	})

	done := make(chan struct{})
	svc.EnqueueEmail("s", "b", func(_ context.Context, _ error) error {
		close(done)
		return nil
	})
	<-done
	svc.WaitUntilDone()

	assert.Len(t, mail.sentMails(), 1)
	assert.Equal(t, 0, sl.callCount())
}
