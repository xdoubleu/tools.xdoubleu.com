package services

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"tools.xdoubleu.com/apps/taste/internal/models"
	"tools.xdoubleu.com/internal/logging"
	"tools.xdoubleu.com/internal/mailer"
	"tools.xdoubleu.com/internal/notifications"
)

type sentMail struct{ to, subject, body string }

// fakeMail records emails and answers each with result.
type fakeMail struct {
	sent   []sentMail
	result error
}

func (f *fakeMail) EnqueueTo(
	to, subject, body string, onResult notifications.OnResult,
) {
	f.sent = append(f.sent, sentMail{to, subject, body})
	if onResult != nil {
		_ = onResult(context.Background(), f.result)
	}
}

type fakePrefs struct {
	enabled bool
	err     error
	set     *bool
}

func (f *fakePrefs) EmailNewPicks(context.Context, string) (bool, error) {
	return f.enabled, f.err
}

func (f *fakePrefs) SetEmailNewPicks(_ context.Context, _ string, on bool) error {
	f.set = &on
	return f.err
}

type fakePicks struct {
	recs []models.Recommendation
	err  error
}

func (f fakePicks) List(
	context.Context, string, string, string, bool,
) ([]models.Recommendation, models.PicksStatus, error) {
	return f.recs, models.PicksStatus{}, f.err
}

func newMail(result error) *fakeMail { return &fakeMail{sent: nil, result: result} }

func prefsOf(enabled bool, err error) *fakePrefs {
	return &fakePrefs{enabled: enabled, err: err, set: nil}
}

func picksOf(recs []models.Recommendation, err error) fakePicks {
	return fakePicks{recs: recs, err: err}
}

func pickOf(title, domain, why string) models.Recommendation {
	//nolint:exhaustruct // what the email reads
	return models.Recommendation{Title: title, Domain: domain, Why: why}
}

func notifier(
	prefs *fakePrefs,
	picks fakePicks,
	mail EmailSender,
) *NotifyService {
	return NewNotifyService(prefs, picks, mail, "https://web.example",
		logging.NewNopLogger())
}

func TestNotifyNewPicks_SendsTheBestPicks(t *testing.T) {
	mail := newMail(nil)
	recs := []models.Recommendation{
		pickOf("Hyperion", "books", "Like Dune."),
		pickOf("P2", "games", "w2"), pickOf("P3", "movies", "w3"),
		pickOf("P4", "series", "w4"), pickOf("P5", "podcasts", "w5"),
		pickOf("P6", "books", "w6"),
	}
	n := notifier(prefsOf(true, nil), picksOf(recs, nil), mail)

	n.NotifyNewPicks(context.Background(), "u", "me@example.com")

	require.Len(t, mail.sent, 1)
	got := mail.sent[0]
	assert.Equal(t, "me@example.com", got.to)
	assert.Equal(t, "6 new recommendations for you", got.subject)
	assert.Contains(t, got.body, "- Hyperion (books): Like Dune.\n")
	assert.Contains(t, got.body, "- P5 (podcasts): w5\n")
	assert.NotContains(t, got.body, "P6", "only the best five are named")
	assert.Contains(t, got.body, "https://web.example/taste")
}

func TestNotifyNewPicks_SingularSubject(t *testing.T) {
	mail := newMail(nil)
	notifier(prefsOf(true, nil),
		picksOf([]models.Recommendation{pickOf("A", "books", "w")}, nil), mail).
		NotifyNewPicks(context.Background(), "u", "me@example.com")
	require.Len(t, mail.sent, 1)
	assert.Equal(t, "1 new recommendation for you", mail.sent[0].subject)
}

func TestNotifyNewPicks_SendsNothing(t *testing.T) {
	one := picksOf([]models.Recommendation{pickOf("A", "books", "w")}, nil)
	tests := []struct {
		name  string
		prefs *fakePrefs
		picks fakePicks
		email string
		mail  bool
	}{
		{"opted out", prefsOf(false, nil), one, "me@example.com", true},
		{"no picks", prefsOf(true, nil), picksOf(nil, nil), "me@example.com", true},
		{"no address", prefsOf(true, nil), one, "", true},
		{"preference unreadable", prefsOf(false, errors.New("db")), one,
			"me@example.com", true},
		{"picks unreadable", prefsOf(true, nil),
			picksOf(nil, errors.New("db")), "me@example.com", true},
		{"no mailer", prefsOf(true, nil), one, "me@example.com", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mail := newMail(nil)
			var sender EmailSender
			if tt.mail {
				sender = mail
			}
			notifier(tt.prefs, tt.picks, sender).
				NotifyNewPicks(context.Background(), "u", tt.email)
			assert.Empty(t, mail.sent)
		})
	}
}

func TestNotifyNewPicks_SendFailureIsNotAnError(t *testing.T) {
	one := picksOf([]models.Recommendation{pickOf("A", "books", "w")}, nil)
	for _, result := range []error{mailer.ErrNotConfigured, errors.New("resend down")} {
		mail := newMail(result)
		notifier(prefsOf(true, nil), one, mail).
			NotifyNewPicks(context.Background(), "u", "me@example.com")
		assert.Len(t, mail.sent, 1)
	}
}

func TestNotifyService_Preference(t *testing.T) {
	prefs := prefsOf(true, nil)
	n := notifier(prefs, picksOf(nil, nil), nil)

	on, err := n.EmailNewPicks(context.Background(), "u")
	require.NoError(t, err)
	assert.True(t, on)

	require.NoError(t, n.SetEmailNewPicks(context.Background(), "u", false))
	require.NotNil(t, prefs.set)
	assert.False(t, *prefs.set)
}
