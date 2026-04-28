package contacts

import (
	"context"
	"net/http"

	"github.com/google/uuid"
	"tools.xdoubleu.com/internal/auth"
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
	Delete(ctx context.Context, id uuid.UUID, ownerUserID string) error
}

type contactsService struct {
	repo *repositories.ContactsRepository
	auth auth.Service
}

func New(repo *repositories.ContactsRepository, authService auth.Service) Service {
	return &contactsService{repo: repo, auth: authService}
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
	users, err := s.auth.GetAllUsers()
	if err != nil {
		return err
	}

	for _, u := range users {
		if u.Email == email {
			name := displayName
			if name == "" {
				name = email
			}
			return s.repo.Add(ctx, ownerUserID, u.ID, name)
		}
	}

	return &notFoundError{}
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
