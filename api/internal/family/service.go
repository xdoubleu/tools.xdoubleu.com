// Package family owns the family entity (membership, invites, display names,
// leaving). Members share recipes, meal plans and shopping lists, keyed by the
// family_id this package hands out. A user belongs to at most one family.
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

// Membership is the caller's family: the other members' IDs and their display
// names (empty when unset). An implicit family-of-one has no members.
type Membership struct {
	FamilyID        uuid.UUID
	Members         []string
	DisplayNames    map[string]string
	SelfDisplayName string
}

type Service interface {
	// GetMembership returns the caller's membership, creating a family-of-one
	// if needed.
	GetMembership(ctx context.Context, userID string) (Membership, error)
	// InviteByEmail invites email's user to fromUserID's family.
	InviteByEmail(ctx context.Context, fromUserID, email string) error
	// GetIncomingInvite returns the pending invite addressed to userID, if any.
	GetIncomingInvite(
		ctx context.Context,
		userID string,
	) (models.FamilyInvite, bool, error)
	Accept(ctx context.Context, userID string) error
	Decline(ctx context.Context, userID string) error
	// SetDisplayName sets the caller's own display name within their family.
	SetDisplayName(ctx context.Context, userID, displayName string) error
	// Leave removes userID from their family; family-scoped data stays with the
	// family.
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

	names, err := s.repo.MemberDisplayNames(ctx, familyID)
	if err != nil {
		return Membership{}, err
	}

	members := make([]string, 0, len(all))
	for _, m := range all {
		if m != userID {
			members = append(members, m)
		}
	}
	return Membership{
		FamilyID:        familyID,
		Members:         members,
		DisplayNames:    names,
		SelfDisplayName: names[userID],
	}, nil
}

func (s *familyService) SetDisplayName(
	ctx context.Context,
	userID, displayName string,
) error {
	return s.repo.SetDisplayName(ctx, userID, displayName)
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

// sendInviteEmail sends off the request path; the invite is already stored.
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

// IsNotFound reports whether err is InviteByEmail's "no user found" error.
func IsNotFound(err error) bool {
	var nfErr *notFoundError
	return errors.As(err, &nfErr)
}
