package contacts

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"

	"github.com/google/uuid"

	"tools.xdoubleu.com/internal/auth"
	essentialogger "tools.xdoubleu.com/internal/logging"
	"tools.xdoubleu.com/internal/mailer"
	"tools.xdoubleu.com/internal/models"
	"tools.xdoubleu.com/internal/repositories"
)

type Service interface {
	List(ctx context.Context, ownerUserID string) ([]models.Contact, error)
	ListPending(ctx context.Context, ownerUserID string) ([]models.Contact, error)
	ListIncoming(ctx context.Context, userID string) ([]models.Contact, error)
	AddByEmail(ctx context.Context, ownerUserID, email, displayName string) error
	Accept(ctx context.Context, rowID uuid.UUID, acceptorID, displayName string) error
	Decline(ctx context.Context, rowID uuid.UUID, acceptorID string) error
	Update(ctx context.Context, id uuid.UUID, ownerUserID, displayName string) error
	Delete(ctx context.Context, id uuid.UUID, ownerUserID string) error
}

type contactsService struct {
	repo   *repositories.ContactsRepository
	auth   auth.Service
	mail   mailer.Client
	webURL string
	logger *slog.Logger
}

func New(
	repo *repositories.ContactsRepository,
	authService auth.Service,
	mail mailer.Client,
	webURL string,
	logger *slog.Logger,
) Service {
	return &contactsService{
		repo:   repo,
		auth:   authService,
		mail:   mail,
		webURL: webURL,
		logger: logger,
	}
}

func (s *contactsService) List(
	ctx context.Context,
	ownerUserID string,
) ([]models.Contact, error) {
	return s.repo.List(ctx, ownerUserID)
}

func (s *contactsService) ListPending(
	ctx context.Context,
	ownerUserID string,
) ([]models.Contact, error) {
	return s.repo.ListPending(ctx, ownerUserID)
}

func (s *contactsService) ListIncoming(
	ctx context.Context,
	userID string,
) ([]models.Contact, error) {
	return s.repo.ListIncoming(ctx, userID)
}

func (s *contactsService) AddByEmail(
	ctx context.Context,
	ownerUserID, email, displayName string,
) error {
	users, err := s.auth.GetAllUsers(ctx)
	if err != nil {
		return err
	}

	var recipient *models.User
	senderEmail := ownerUserID
	for i, u := range users {
		if u.Email == email {
			recipient = &users[i]
		}
		if u.ID == ownerUserID {
			senderEmail = u.Email
		}
	}
	if recipient == nil {
		return &notFoundError{}
	}

	name := displayName
	if name == "" {
		name = email
	}
	if err = s.repo.Add(ctx, ownerUserID, recipient.ID, name); err != nil {
		return err
	}

	s.sendContactRequestEmail(ctx, recipient.Email, senderEmail)
	return nil
}

// sendContactRequestEmail notifies the recipient by email; failures never
// fail the contact request itself — the request is already persisted, and
// email delivery is a secondary notification, matching how IssueNotifierJob
// treats mailer.ErrNotConfigured as a degraded (not failed) state.
func (s *contactsService) sendContactRequestEmail(
	ctx context.Context,
	to, senderEmail string,
) {
	subject := fmt.Sprintf("%s wants to add you as a contact", senderEmail)
	body := fmt.Sprintf(
		"%s sent you a contact request on tools.xdoubleu.com.\n\n"+
			"Accept or decline it here: %s/contacts",
		senderEmail, s.webURL,
	)
	if err := s.mail.SendTo(ctx, to, subject, body); err != nil &&
		!errors.Is(err, mailer.ErrNotConfigured) {
		s.logger.ErrorContext(ctx, "contacts: failed to send contact request email",
			essentialogger.ErrAttr(err))
	}
}

func (s *contactsService) Accept(
	ctx context.Context,
	rowID uuid.UUID,
	acceptorID, displayName string,
) error {
	return s.repo.Accept(ctx, rowID, acceptorID, displayName)
}

func (s *contactsService) Decline(
	ctx context.Context,
	rowID uuid.UUID,
	acceptorID string,
) error {
	return s.repo.Decline(ctx, rowID, acceptorID)
}

func (s *contactsService) Update(
	ctx context.Context,
	id uuid.UUID,
	ownerUserID, displayName string,
) error {
	return s.repo.UpdateDisplayName(ctx, id, ownerUserID, displayName)
}

func (s *contactsService) Delete(
	ctx context.Context,
	id uuid.UUID,
	ownerUserID string,
) error {
	return s.repo.Delete(ctx, id, ownerUserID)
}

type notFoundError struct{}

func (e *notFoundError) Error() string {
	return "no user found with that email address"
}

func (e *notFoundError) HTTPStatus() int {
	return http.StatusNotFound
}

// IsNotFound reports whether err is (or wraps) the "no user found" error
// AddByEmail returns for expected user input, distinguishing it from real
// internal failures so callers don't report it as one.
func IsNotFound(err error) bool {
	var nfErr *notFoundError
	return errors.As(err, &nfErr)
}
