package mcp

import (
	"context"
	"testing"

	"connectrpc.com/connect/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"tools.xdoubleu.com/internal/kit/constants"
	sharedmodels "tools.xdoubleu.com/internal/models"
)

func TestWriteToolsRequireUser(t *testing.T) {
	ctx := t.Context()
	tl := &tools{}

	_, err := tl.mcpCreateRecipe(ctx, mcpCreateRecipeArgs{})
	require.Error(t, err)
	assert.Equal(t, connect.CodeUnauthenticated, connect.CodeOf(err))

	_, err = tl.mcpUpdateRecipe(ctx, mcpUpdateRecipeArgs{})
	require.Error(t, err)
	assert.Equal(t, connect.CodeUnauthenticated, connect.CodeOf(err))
}

func TestToolsRejectMalformedRecipeID(t *testing.T) {
	//nolint:exhaustruct // only the ID matters
	ctx := context.WithValue(
		t.Context(), constants.UserContextKey, sharedmodels.User{ID: "u"},
	)
	tl := &tools{}

	_, err := tl.mcpUpdateRecipe(ctx, mcpUpdateRecipeArgs{ID: "nope"})
	assert.Equal(t, connect.CodeInvalidArgument, connect.CodeOf(err))

	_, err = tl.mcpGetRecipe(ctx, mcpRecipeArgs{ID: "nope"})
	assert.Equal(t, connect.CodeInvalidArgument, connect.CodeOf(err))
}
