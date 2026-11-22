package mcp

import (
	"context"
	"testing"

	"connectrpc.com/connect/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"tools.xdoubleu.com/internal/kit/constants"
	sharedmodels "tools.xdoubleu.com/internal/models"
	"tools.xdoubleu.com/internal/platform/mcptools"
)

func TestToolsRequireUser(t *testing.T) {
	ctx := t.Context()
	tl := &tools{}
	calls := map[string]func() error{
		"stats":  func() error { _, err := tl.mcpGetStats(ctx, mcpSharedArgs{}); return err },
		"events": func() error { _, err := tl.mcpListEvents(ctx, mcpListEventsArgs{}); return err },
		"profile": func() error {
			_, err := tl.mcpGetProfile(ctx, mcpSharedArgs{})
			return err
		},
		"recommendations": func() error {
			_, err := tl.mcpListRecommendations(ctx, mcpListRecommendationsArgs{})
			return err
		},
		"shows": func() error {
			_, err := tl.mcpListFavouriteShows(ctx, mcptools.NoArgs{})
			return err
		},
		"add": func() error {
			_, err := tl.mcpAddFavouriteShow(ctx, mcpAddFavouriteShowArgs{})
			return err
		},
		"remove": func() error {
			_, err := tl.mcpRemoveFavouriteShow(ctx, mcpRemoveFavouriteShowArgs{})
			return err
		},
	}
	for name, call := range calls {
		err := call()
		require.Error(t, err, name)
		assert.Equal(t, connect.CodeUnauthenticated, connect.CodeOf(err), name)
	}
}

func TestRemoveFavouriteShowRejectsMalformedID(t *testing.T) {
	//nolint:exhaustruct // only the ID matters
	ctx := context.WithValue(
		t.Context(), constants.UserContextKey, sharedmodels.User{ID: "u"},
	)
	_, err := (&tools{}).mcpRemoveFavouriteShow(
		ctx, mcpRemoveFavouriteShowArgs{ID: "nope"},
	)
	assert.Equal(t, connect.CodeInvalidArgument, connect.CodeOf(err))
}
