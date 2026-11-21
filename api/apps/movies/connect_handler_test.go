package movies_test

import (
	"context"
	"testing"

	"connectrpc.com/connect/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	moviesconnect "tools.xdoubleu.com/apps/movies/connect"
	"tools.xdoubleu.com/apps/movies/internal/repositories"
	"tools.xdoubleu.com/apps/movies/internal/services"
	moviesv1 "tools.xdoubleu.com/gen/movies/v1"
	"tools.xdoubleu.com/internal/kit/constants"
	sharedmocks "tools.xdoubleu.com/internal/mocks"
	sharedmodels "tools.xdoubleu.com/internal/models"
)

func TestHandler_GetStatsErrors(t *testing.T) {
	svc := services.New(
		repositories.New(testDB), sharedmocks.NewMockedAuthService(userID), nil, testFamily,
	)
	h := moviesconnect.NewHandler(svc.Movies)
	req := &moviesv1.GetStatsRequest{}

	_, err := h.GetStats(context.Background(), req)
	assert.Equal(t, connect.CodeUnauthenticated, connect.CodeOf(err))

	//nolint:exhaustruct // only the ID matters
	user := sharedmodels.User{ID: userID}
	ctx, cancel := context.WithCancel(
		context.WithValue(context.Background(), constants.UserContextKey, user),
	)
	cancel()
	_, err = h.GetStats(ctx, req)
	require.Error(t, err)
	assert.Equal(t, connect.CodeInternal, connect.CodeOf(err))
}
