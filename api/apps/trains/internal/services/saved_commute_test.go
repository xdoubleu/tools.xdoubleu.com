package services_test

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"tools.xdoubleu.com/apps/trains/internal/mocks"
	"tools.xdoubleu.com/apps/trains/internal/models"
	"tools.xdoubleu.com/apps/trains/internal/services"
)

type memStore struct{ rows []models.SavedCommute }

func (m *memStore) ListByUser(
	_ context.Context, _ string,
) ([]models.SavedCommute, error) {
	return m.rows, nil
}

func (m *memStore) Create(
	_ context.Context, sc models.SavedCommute,
) (models.SavedCommute, error) {
	sc.ID = uuid.New()
	m.rows = append(m.rows, sc)
	return sc, nil
}

func (m *memStore) Update(
	_ context.Context, sc models.SavedCommute,
) (models.SavedCommute, error) {
	return sc, nil
}

func (m *memStore) Delete(context.Context, uuid.UUID, string) error { return nil }

func TestSavedCommutesService_OverStore(t *testing.T) {
	ctx := context.Background()
	store := &memStore{rows: nil}
	svc := services.New(mocks.NewFixtureClient(t), store).SavedCommutes

	_, err := svc.Create(ctx, "u", " ", brusselsNorth, liege)
	require.ErrorIs(t, err, services.ErrInvalidCommute)
	_, err = svc.Create(ctx, "u", "x", liege, liege)
	require.ErrorIs(t, err, services.ErrInvalidCommute)
	_, err = svc.Create(ctx, "u", "x", brusselsNorth, "nope")
	require.ErrorIs(t, err, services.ErrInvalidCommute)
	assert.Empty(t, store.rows, "invalid commutes never reach the store")

	created, err := svc.Create(ctx, "u", " work ", brusselsNorth, liege)
	require.NoError(t, err)
	assert.Equal(t, "work", created.Label)
	assert.Equal(t, "Brussel-Noord", created.Origin.NameNL, "stations are named")

	list, err := svc.List(ctx, "u")
	require.NoError(t, err)
	require.Len(t, list, 1)
}
