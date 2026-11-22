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

//nolint:exhaustruct // zero values are the point of these tests
func TestToolsRequireUser(t *testing.T) {
	ctx := t.Context()
	tl := tools{}
	calls := map[string]func() error{
		"feeds": func() error {
			_, err := tl.mcpListFeeds(ctx, mcptools.NoArgs{})
			return err
		},
		"items": func() error {
			_, err := tl.mcpListItems(ctx, mcpListItemsArgs{})
			return err
		},
		"item": func() error {
			_, err := tl.mcpGetItem(ctx, mcpGetItemArgs{})
			return err
		},
		"rules": func() error {
			_, err := tl.mcpListFilterRules(ctx, mcptools.NoArgs{})
			return err
		},
		"suggestions": func() error {
			_, err := tl.mcpGetFilterRuleSuggestions(ctx, mcptools.NoArgs{})
			return err
		},
	}
	for name, call := range calls {
		err := call()
		require.Error(t, err, name)
		assert.Equal(t, connect.CodeUnauthenticated, connect.CodeOf(err), name)
	}
}

//nolint:exhaustruct // zero values are the point of these tests
func TestToolsRejectMalformedIDs(t *testing.T) {
	//nolint:exhaustruct // only the ID matters
	ctx := context.WithValue(
		t.Context(), constants.UserContextKey, sharedmodels.User{ID: "u"},
	)
	tl := tools{}

	_, err := tl.mcpListItems(ctx, mcpListItemsArgs{FeedID: "nope"})
	assert.Equal(t, connect.CodeInvalidArgument, connect.CodeOf(err))

	_, err = tl.mcpGetItem(ctx, mcpGetItemArgs{ItemID: "nope"})
	assert.Equal(t, connect.CodeInvalidArgument, connect.CodeOf(err))
}
