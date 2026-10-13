package services

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"tools.xdoubleu.com/apps/taste/internal/models"
	"tools.xdoubleu.com/internal/mailer"
	"tools.xdoubleu.com/internal/notifications"
)

// maxEmailedPicks is how many of the best picks an email names.
const maxEmailedPicks = 5

// EmailSender queues an email off the caller's path.
type EmailSender interface {
	EnqueueTo(to, subject, body string, onResult notifications.OnResult)
}

type preferenceStore interface {
	EmailNewPicks(ctx context.Context, userID string) (bool, error)
	SetEmailNewPicks(ctx context.Context, userID string, enabled bool) error
}

type latestPicks interface {
	List(
		ctx context.Context,
		userID, domain, track string,
		savedOnly bool,
	) ([]models.Recommendation, *time.Time, string, error)
}

// NotifyService emails a user when a run has written new picks, unless they
// turned that off.
type NotifyService struct {
	prefs  preferenceStore
	picks  latestPicks
	mail   EmailSender
	webURL string
	logger *slog.Logger
}

// NewNotifyService sends through mail, which may be nil to send nothing.
func NewNotifyService(
	prefs preferenceStore,
	picks latestPicks,
	mail EmailSender,
	webURL string,
	logger *slog.Logger,
) *NotifyService {
	return &NotifyService{
		prefs: prefs, picks: picks, mail: mail, webURL: webURL, logger: logger,
	}
}

// EmailNewPicks reports whether userID gets the email.
func (s *NotifyService) EmailNewPicks(
	ctx context.Context,
	userID string,
) (bool, error) {
	return s.prefs.EmailNewPicks(ctx, userID)
}

// SetEmailNewPicks turns the email on or off for userID.
func (s *NotifyService) SetEmailNewPicks(
	ctx context.Context,
	userID string,
	enabled bool,
) error {
	return s.prefs.SetEmailNewPicks(ctx, userID, enabled)
}

// NotifyNewPicks emails the user the best of their latest picks. It sends
// nothing without an address, a mailer or picks, or when they opted out. A
// failure is logged, never returned: the picks are already stored.
func (s *NotifyService) NotifyNewPicks(ctx context.Context, userID, email string) {
	if s.mail == nil || email == "" {
		return
	}
	enabled, err := s.prefs.EmailNewPicks(ctx, userID)
	if err != nil {
		s.logger.WarnContext(ctx, "taste: couldn't read the email preference",
			slog.String("user_id", userID), slog.Any("error", err))
		return
	}
	if !enabled {
		return
	}
	recs, _, _, err := s.picks.List(ctx, userID, "", "", false)
	if err != nil {
		s.logger.WarnContext(ctx, "taste: couldn't read the new picks",
			slog.String("user_id", userID), slog.Any("error", err))
		return
	}
	if len(recs) == 0 {
		return
	}
	s.mail.EnqueueTo(email, subject(len(recs)), s.body(recs),
		func(ctx context.Context, sendErr error) error {
			if sendErr != nil && !errors.Is(sendErr, mailer.ErrNotConfigured) {
				s.logger.ErrorContext(ctx, "taste: failed to send the new picks email",
					slog.String("user_id", userID), slog.Any("error", sendErr))
			}
			return nil
		})
}

func subject(n int) string {
	if n == 1 {
		return "1 new recommendation for you"
	}
	return fmt.Sprintf("%d new recommendations for you", n)
}

// body names the best picks (recs is best first) and links to the rest.
func (s *NotifyService) body(recs []models.Recommendation) string {
	var b strings.Builder
	b.WriteString("Your taste profile has new picks on tools.xdoubleu.com.\n\n")
	for _, r := range recs[:min(len(recs), maxEmailedPicks)] {
		fmt.Fprintf(&b, "- %s (%s): %s\n", r.Title, r.Domain, r.Why)
	}
	fmt.Fprintf(&b, "\nSee all of them: %s/taste\n\n", s.webURL)
	b.WriteString("You can turn these emails off on the Recommendations card.")
	return b.String()
}
