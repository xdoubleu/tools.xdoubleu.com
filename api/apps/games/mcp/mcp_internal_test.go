package mcp

import (
	"testing"

	"connectrpc.com/connect/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"tools.xdoubleu.com/internal/platform/mcptools"
)

//nolint:exhaustruct // zero values are the point of these tests
func TestToolsRequireUser(t *testing.T) {
	ctx := t.Context()
	tl := tools{}
	calls := map[string]func() error{
		"steam": func() error { _, err := tl.mcpGetSteam(ctx, steamRangeArgs{}); return err },
		"game":  func() error { _, err := tl.mcpGetSteamGame(ctx, gameIDArgs{}); return err },
		"dist": func() error {
			_, err := tl.mcpGetSteamDistribution(ctx, bucketArgs{})
			return err
		},
		"recent": func() error {
			_, err := tl.mcpGetRecentlyActiveGames(ctx, mcptools.NoArgs{})
			return err
		},
		"integrations": func() error {
			_, err := tl.mcpGetIntegrations(ctx, mcptools.NoArgs{})
			return err
		},
		"wishlist": func() error {
			_, err := tl.mcpGetWishlist(ctx, mcptools.NoArgs{})
			return err
		},
	}
	for name, call := range calls {
		err := call()
		require.Error(t, err, name)
		assert.Equal(t, connect.CodeUnauthenticated, connect.CodeOf(err), name)
	}
}
