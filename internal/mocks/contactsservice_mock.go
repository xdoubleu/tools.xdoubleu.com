package mocks

import (
	"context"

	"github.com/google/uuid"
	"tools.xdoubleu.com/internal/contacts"
	"tools.xdoubleu.com/internal/models"
)

func NewMockedContactsService() contacts.Service {
	return &MockedContactsService{}
}

type MockedContactsService struct{}

func (m *MockedContactsService) List(
	_ context.Context,
	_ string,
) ([]models.Contact, error) {
	return []models.Contact{}, nil
}

func (m *MockedContactsService) ListPending(
	_ context.Context,
	_ string,
) ([]models.Contact, error) {
	return []models.Contact{}, nil
}

func (m *MockedContactsService) ListIncoming(
	_ context.Context,
	_ string,
) ([]models.Contact, error) {
	return []models.Contact{}, nil
}

func (m *MockedContactsService) AddByEmail(
	_ context.Context,
	_, _, _ string,
) error {
	return nil
}

func (m *MockedContactsService) Accept(
	_ context.Context,
	_ uuid.UUID,
	_, _ string,
) error {
	return nil
}

func (m *MockedContactsService) Decline(
	_ context.Context,
	_ uuid.UUID,
	_ string,
) error {
	return nil
}

func (m *MockedContactsService) Delete(
	_ context.Context,
	_ uuid.UUID,
	_ string,
) error {
	return nil
}
