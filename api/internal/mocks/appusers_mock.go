package mocks

import (
	"context"

	"tools.xdoubleu.com/internal/models"
)

// FakeAppUsersStore is a configurable test double for the narrow interface
// auth.LocalService needs from *repositories.AppUsersRepository. It lets
// tests force Upsert and GetByID to fail independently — something a single
// canceled context can't isolate, since both real DB calls share one ctx.
type FakeAppUsersStore struct {
	UpsertErr  error
	GetByIDErr error
	User       models.User
}

func (f *FakeAppUsersStore) Upsert(_ context.Context, _, _ string) error {
	return f.UpsertErr
}

func (f *FakeAppUsersStore) GetByID(_ context.Context, _ string) (*models.User, error) {
	if f.GetByIDErr != nil {
		return nil, f.GetByIDErr
	}
	user := f.User
	return &user, nil
}

func (f *FakeAppUsersStore) GetAll(_ context.Context) ([]models.User, error) {
	return []models.User{f.User}, nil
}
