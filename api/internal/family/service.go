// Package family implements the shared "family" concept from issue #1349: a
// user configures a family (of at most one, per the issue's confirmed
// decision) whose members will eventually share one set of recipes, one meal
// plan set, and one shopping list as a single unit, replacing the
// owner-centric per-app sharing model. This package lands the family entity
// itself — membership, invites (reusing the contacts invite/accept pattern),
// and leaving — as the substrate the three apps re-key onto in a follow-up
// (see issue #1349's tracking issue for the coupling that makes that its own
// piece of work: shoppinglist's catalog queries already join directly into
// recipes/mealplans by owner user ID, so re-keying needs to happen for all
// three together, not one app at a time).
package family

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"

	"github.com/google/uuid"

	iapp "tools.xdoubleu.com/internal/app"
	"tools.xdoubleu.com/internal/auth"
	essentialogger "tools.xdoubleu.com/internal/logging"
	"tools.xdoubleu.com/internal/mailer"
	"tools.xdoubleu.com/internal/models"
	"tools.xdoubleu.com/internal/notifications"
	"tools.xdoubleu.com/internal/repositories"
)

// Membership describes the caller's current family: FamilyID and the other
// members' user IDs (self excluded). An implicit family-of-one has no
// members.
type Membership struct {
	FamilyID uuid.UUID
	Members  []string
}

type Service interface {
	// GetMembership returns the caller's family membership, lazily creating
	// their implicit family-of-one if they don't have one yet.
	GetMembership(ctx context.Context, userID string) (Membership, error)
	// InviteByEmail invites the user with the given email to join
	// fromUserID's family, creating that family first if needed.
	InviteByEmail(ctx context.Context, fromUserID, email string) error
	// GetIncomingInvite returns the pending invite addressed to userID, if any.
	GetIncomingInvite(ctx context.Context, userID string) (models.FamilyInvite, bool, error)
	Accept(ctx context.Context, userID string) error
	Decline(ctx context.Context, userID string) error
	// Leave removes userID from their family. Their family-scoped data,
	// once apps re-key onto family_id, stays with the family — it cannot be
	// un-merged (issue #1349's confirmed decision).
	Leave(ctx context.Context, userID string) error
}

type familyService struct {
	repo          *repositories.FamilyRepository
	auth          auth.Service
	notifications *notifications.Service
	webURL        string
	logger        *slog.Logger
}

func New(
	repo *repositories.FamilyRepository,
	authService auth.Service,
	notifications *notifications.Service,
	webURL string,
	logger *slog.Logger,
) Service {
	return &familyService{
		repo:          repo,
		auth:          authService,
		notifications: notifications,
		webURL:        webURL,
		logger:        logger,
	}
}

func (s *familyService) GetMembership(
	ctx context.Context,
	userID string,
) (Membership, error) {
	familyID, err := s.repo.EnsureFamily(ctx, userID)
	if err != nil {
		return Membership{}, err
	}

	all, err := s.repo.ListMembers(ctx, familyID)
	if err != nil {
		return Membership{}, err
	}

	members := make([]string, 0, len(all))
	for _, m := range all {
		if m != userID {
			members = append(members, m)
		}
	}
	return Membership{FamilyID: familyID, Members: members}, nil
}

func (s *familyService) InviteByEmail(
	ctx context.Context,
	fromUserID, email string,
) error {
	users, err := s.auth.GetAllUsers(ctx)
	if err != nil {
		return err
	}

	var recipient *models.User
	senderEmail := fromUserID
	for i, u := range users {
		if u.Email == email {
			recipient = &users[i]
		}
		if u.ID == fromUserID {
			senderEmail = u.Email
		}
	}
	if recipient == nil {
		return &notFoundError{}
	}
	if recipient.ID == fromUserID {
		return &iapp.HTTPError{
			Status:  http.StatusBadRequest,
			Message: "You cannot invite yourself",
		}
	}

	if err = s.repo.Invite(ctx, fromUserID, recipient.ID); err != nil {
		return err
	}

	s.sendInviteEmail(recipient.Email, senderEmail)
	return nil
}

// sendInviteEmail queues a notification email, following the same
// failure-tolerant pattern as internal/contacts' sendContactRequestEmail —
// the invite is already persisted, so a delivery failure only degrades the
// notification, never the request itself.
func (s *familyService) sendInviteEmail(to, senderEmail string) {
	subject := fmt.Sprintf("%s invited you to join their family", senderEmail)
	body := fmt.Sprintf(
		"%s invited you to share one recipe book, meal plan and shopping "+
			"list together on tools.xdoubleu.com.\n\n"+
			"Accept or decline it here: %s/family",
		senderEmail, s.webURL,
	)
	s.notifications.EnqueueTo(
		to,
		subject,
		body,
		func(ctx context.Context, err error) error {
			if err != nil && !errors.Is(err, mailer.ErrNotConfigured) {
				s.logger.ErrorContext(
					ctx,
					"family: failed to send invite email",
					essentialogger.ErrAttr(err),
				)
			}
			return nil
		},
	)
}

func (s *familyService) GetIncomingInvite(
	ctx context.Context,
	userID string,
) (models.FamilyInvite, bool, error) {
	return s.repo.GetInvite(ctx, userID)
}

func (s *familyService) Accept(ctx context.Context, userID string) error {
	_, err := s.repo.AcceptInvite(ctx, userID)
	return err
}

func (s *familyService) Decline(ctx context.Context, userID string) error {
	return s.repo.DeclineInvite(ctx, userID)
}

func (s *familyService) Leave(ctx context.Context, userID string) error {
	return s.repo.Leave(ctx, userID)
}

type notFoundError struct{}

func (e *notFoundError) Error() string {
	return "no user found with that email address"
}

func (e *notFoundError) HTTPStatus() int {
	return http.StatusNotFound
}

// IsNotFound reports whether err is (or wraps) the "no user found" error
// InviteByEmail returns for expected user input.
func IsNotFound(err error) bool {
	var nfErr *notFoundError
	return errors.As(err, &nfErr)
}
