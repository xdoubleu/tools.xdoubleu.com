// Package mcp exposes the games app's read RPCs as MCP tools.
package mcp

import (
	"context"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"google.golang.org/protobuf/proto"

	"tools.xdoubleu.com/apps/games/connect"
	gamesv1 "tools.xdoubleu.com/gen/games/v1"
	"tools.xdoubleu.com/internal/platform/mcptools"
)

const mcpAppName = "games"

type gameIDArgs struct {
	GameID int32 `json:"game_id" jsonschema:"games-app game id"`
}

type steamRangeArgs struct {
	DateStart string `json:"date_start,omitempty" jsonschema:"window start (YYYY-MM-DD)"`
	DateEnd   string `json:"date_end,omitempty"   jsonschema:"window end (YYYY-MM-DD)"`
}

// bucketArgs indexes services.DistributionLabels (0 = 0-9%, ..., 10 = 100%).
// Keep the documented range in step with it, or agents miss buckets.
type bucketArgs struct {
	Bucket int32 `json:"bucket,omitempty" jsonschema:"bucket (0-10; 10=100%)"`
}

// tools adapts the ConnectRPC handler to MCP tool signatures.
type tools struct{ h *connect.Handler }

// Register exposes the app's read-only RPCs as MCP tools, scoped to the
// caller.
func Register(srv *mcp.Server, handler *connect.Handler) {
	t := tools{h: handler}

	mcptools.AddReadTool(srv, mcpAppName, "games_get_steam",
		"Steam backlog overview: not-started/in-progress/completed games, "+
			"backlog size, completion-rate progress, and the delisted games "+
			"absent from every list (in_completion_average says which of "+
			"those still count towards the rate).", t.mcpGetSteam)
	mcptools.AddReadTool(srv, mcpAppName, "games_get_steam_game",
		"A single Steam game with its achievements.", t.mcpGetSteamGame)
	mcptools.AddReadTool(srv, mcpAppName, "games_get_steam_distribution",
		"Games in one completion-rate distribution bucket.",
		t.mcpGetSteamDistribution)
	mcptools.AddReadTool(srv, mcpAppName, "games_get_recently_active_games",
		"Games with recently unlocked achievements.",
		t.mcpGetRecentlyActiveGames)
	mcptools.AddReadTool(srv, mcpAppName, "games_get_wishlist",
		"The user's synced Steam wishlist, highest priority first.",
		t.mcpGetWishlist)
	mcptools.AddReadTool(srv, mcpAppName, "games_get_integrations",
		"The user's games-app integration settings (Steam user id).",
		t.mcpGetIntegrations)
}

func (t tools) mcpGetSteam(
	ctx context.Context, args steamRangeArgs,
) (proto.Message, error) {
	return mcptools.Unwrap(
		t.h.GetSteam(
			ctx,
			&gamesv1.GetSteamRequest{DateStart: args.DateStart, DateEnd: args.DateEnd},
		),
	)
}

func (t tools) mcpGetSteamGame(
	ctx context.Context, args gameIDArgs,
) (proto.Message, error) {
	return mcptools.Unwrap(
		t.h.GetSteamGame(ctx, &gamesv1.GetSteamGameRequest{GameId: args.GameID}),
	)
}

func (t tools) mcpGetSteamDistribution(
	ctx context.Context, args bucketArgs,
) (proto.Message, error) {
	return mcptools.Unwrap(
		t.h.GetSteamDistribution(
			ctx,
			&gamesv1.GetSteamDistributionRequest{Bucket: args.Bucket},
		),
	)
}

func (t tools) mcpGetRecentlyActiveGames(
	ctx context.Context, _ mcptools.NoArgs,
) (proto.Message, error) {
	return mcptools.Unwrap(
		t.h.GetRecentlyActiveGames(ctx, &gamesv1.GetRecentlyActiveGamesRequest{}),
	)
}

func (t tools) mcpGetIntegrations(
	ctx context.Context, _ mcptools.NoArgs,
) (proto.Message, error) {
	return mcptools.Unwrap(t.h.GetIntegrations(ctx, &gamesv1.GetIntegrationsRequest{}))
}

func (t tools) mcpGetWishlist(
	ctx context.Context, _ mcptools.NoArgs,
) (proto.Message, error) {
	return mcptools.Unwrap(t.h.GetWishlist(ctx, &gamesv1.GetWishlistRequest{}))
}
