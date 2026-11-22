package connect_test

import (
	"context"
	"errors"
	"testing"

	"connectrpc.com/connect/v2"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"

	tastec "tools.xdoubleu.com/apps/taste/connect"
	"tools.xdoubleu.com/apps/taste/internal/services"
	tastev1 "tools.xdoubleu.com/gen/taste/v1"
	"tools.xdoubleu.com/internal/kit/constants"
	sharedmodels "tools.xdoubleu.com/internal/models"
)

type brokenFamily struct{}

func (brokenFamily) EnsureFamily(context.Context, string) (uuid.UUID, error) {
	return uuid.Nil, errors.New("db down")
}

type noApps struct{}

func (noApps) CanAccess(context.Context, string) bool { return false }

type noJobs struct{}

func (noJobs) ForceRun(string) {}

func newHandler() *tastec.Handler {
	//nolint:exhaustruct // only the scope is reached
	return tastec.NewHandler(
		&services.Services{Scope: services.NewScopeService(brokenFamily{}, noApps{})},
		noJobs{},
	)
}

func TestHandler_UnauthenticatedCallerIsRejected(t *testing.T) {
	_, err := newHandler().GetStats(context.Background(), &tastev1.GetStatsRequest{})
	assert.Equal(t, connect.CodeUnauthenticated, connect.CodeOf(err))
}

func TestHandler_SharedScopeFailureIsMapped(t *testing.T) {
	ctx := context.WithValue(
		context.Background(), constants.UserContextKey,
		//nolint:exhaustruct // only the ID matters
		sharedmodels.User{ID: "u"},
	)
	_, err := newHandler().GetStats(ctx, &tastev1.GetStatsRequest{Shared: true})
	assert.Equal(t, connect.CodeInternal, connect.CodeOf(err))
}
