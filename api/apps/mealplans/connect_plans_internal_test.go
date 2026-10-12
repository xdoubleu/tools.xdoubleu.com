package mealplans

import (
	"context"
	"testing"

	"connectrpc.com/connect/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	mealplansv1 "tools.xdoubleu.com/gen/mealplans/v1"
)

func TestRotateICalToken_Unauthenticated(t *testing.T) {
	h := &mealplansConnectHandler{app: nil}
	_, err := h.RotateICalToken(
		context.Background(),
		&mealplansv1.RotateICalTokenRequest{Id: "x"},
	)
	require.Error(t, err)
	assert.Equal(t, connect.CodeUnauthenticated, connect.CodeOf(err))
}
