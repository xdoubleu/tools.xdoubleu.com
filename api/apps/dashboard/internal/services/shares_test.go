package services_test

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"tools.xdoubleu.com/apps/dashboard/internal/services"
	"tools.xdoubleu.com/internal/kit/database"
	sharedmodels "tools.xdoubleu.com/internal/models"
)

type fakeTokens struct {
	err    error
	called bool
	kind   sharedmodels.DashboardKind
}

func (f *fakeTokens) ResolveToken(
	_ context.Context, _ string, kind sharedmodels.DashboardKind,
) (string, string, error) {
	f.called, f.kind = true, kind
	if f.err != nil {
		return "", "", f.err
	}
	return "owner", "Owner", nil
}

func TestShareService_Resolve(t *testing.T) {
	tokens := &fakeTokens{err: nil, called: false, kind: ""}
	svc := services.NewShareService(tokens)

	id, name, err := svc.Resolve(t.Context(), "tok", sharedmodels.DashboardKindGames)

	require.NoError(t, err)
	assert.Equal(t, "owner", id)
	assert.Equal(t, "Owner", name)
	assert.Equal(t, sharedmodels.DashboardKindGames, tokens.kind)
}

func TestShareService_EmptyTokenSkipsLookup(t *testing.T) {
	tokens := &fakeTokens{err: nil, called: false, kind: ""}
	svc := services.NewShareService(tokens)

	_, _, err := svc.Resolve(t.Context(), "", sharedmodels.DashboardKindGames)

	assert.ErrorIs(t, err, services.ErrUnknownDashboard)
	assert.False(t, tokens.called)
}

func TestShareService_NotFoundIsUnknownDashboard(t *testing.T) {
	tokens := &fakeTokens{err: database.ErrResourceNotFound, called: false, kind: ""}
	svc := services.NewShareService(tokens)

	_, _, err := svc.Resolve(t.Context(), "x", sharedmodels.DashboardKindReading)

	assert.ErrorIs(t, err, services.ErrUnknownDashboard)
}

func TestShareService_OtherErrorsPassThrough(t *testing.T) {
	boom := errors.New("db down")
	svc := services.NewShareService(&fakeTokens{err: boom, called: false, kind: ""})

	_, _, err := svc.Resolve(t.Context(), "x", sharedmodels.DashboardKindReading)

	assert.ErrorIs(t, err, boom)
	assert.NotErrorIs(t, err, services.ErrUnknownDashboard)
}
