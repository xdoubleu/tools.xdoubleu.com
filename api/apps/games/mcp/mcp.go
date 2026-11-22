// Package mcp exposes the games app's read RPCs as MCP tools.
package mcp

import (
	"context"

	"connectrpc.com/connect/v2"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"google.golang.org/protobuf/proto"

	"tools.xdoubleu.com/apps/games/internal/protomap"
	"tools.xdoubleu.com/apps/games/internal/services"
	gamesv1 "tools.xdoubleu.com/gen/games/v1"
	"tools.xdoubleu.com/internal/connecttools"
	"tools.xdoubleu.com/internal/kit/constants"
	"tools.xdoubleu.com/internal/kit/contexttools"
	sharedmodels "tools.xdoubleu.com/internal/models"
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

type tools struct{ s *services.Services }

// Register exposes the app's read-only RPCs as MCP tools, scoped to the
// caller.
func Register(srv *mcp.Server, svc *services.Services) {
	t := tools{s: svc}

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

func userID(ctx context.Context) (string, error) {
	user := contexttools.GetValue[sharedmodels.User](ctx, constants.UserContextKey)
	if user == nil {
		return "", connect.NewError(connect.CodeUnauthenticated, "unauthorized")
	}
	return user.ID, nil
}

func (t tools) mcpGetSteam(
	ctx context.Context, args steamRangeArgs,
) (proto.Message, error) {
	uid, err := userID(ctx)
	if err != nil {
		return nil, err
	}
	dateStart, dateEnd := protomap.DateRange(args.DateStart, args.DateEnd)
	o, err := t.s.Library.GetSteam(ctx, uid, dateStart, dateEnd)
	if err != nil {
		return nil, connecttools.MapError(err)
	}
	return &gamesv1.GetSteamResponse{Steam: protomap.Steam(o)}, nil
}

func (t tools) mcpGetSteamGame(
	ctx context.Context, args gameIDArgs,
) (proto.Message, error) {
	uid, err := userID(ctx)
	if err != nil {
		return nil, err
	}
	d, err := t.s.Library.GetSteamGame(ctx, uid, int(args.GameID))
	if err != nil {
		return nil, connecttools.MapError(err)
	}
	return &gamesv1.GetSteamGameResponse{Data: protomap.SteamGame(d)}, nil
}

func (t tools) mcpGetSteamDistribution(
	ctx context.Context, args bucketArgs,
) (proto.Message, error) {
	uid, err := userID(ctx)
	if err != nil {
		return nil, err
	}
	label, bucketGames, err := t.s.Progress.GetDistributionBucket(
		ctx, uid, int(args.Bucket),
	)
	if err != nil {
		return nil, connecttools.MapError(
			err,
			connecttools.Code(services.ErrInvalidBucket, connect.CodeInvalidArgument),
		)
	}
	return &gamesv1.GetSteamDistributionResponse{
		Data: &gamesv1.SteamDistributionResponse{
			Label: label,
			Games: protomap.Games(bucketGames),
		},
	}, nil
}

func (t tools) mcpGetRecentlyActiveGames(
	ctx context.Context, _ mcptools.NoArgs,
) (proto.Message, error) {
	uid, err := userID(ctx)
	if err != nil {
		return nil, err
	}
	games, err := t.s.Steam.GetRecentlyActive(ctx, uid)
	if err != nil {
		return nil, connecttools.MapError(err)
	}
	return &gamesv1.GetRecentlyActiveGamesResponse{
		Games: protomap.RecentGames(games),
	}, nil
}

func (t tools) mcpGetIntegrations(
	ctx context.Context, _ mcptools.NoArgs,
) (proto.Message, error) {
	uid, err := userID(ctx)
	if err != nil {
		return nil, err
	}
	integrations, err := t.s.Integrations.Get(ctx, uid)
	if err != nil {
		return nil, connecttools.MapError(err)
	}
	return &gamesv1.GetIntegrationsResponse{
		Integrations: &gamesv1.Integrations{SteamUserId: integrations.SteamUserID},
	}, nil
}

func (t tools) mcpGetWishlist(
	ctx context.Context, _ mcptools.NoArgs,
) (proto.Message, error) {
	uid, err := userID(ctx)
	if err != nil {
		return nil, err
	}
	items, err := t.s.Steam.GetWishlist(ctx, uid)
	if err != nil {
		return nil, connecttools.MapError(err)
	}
	return &gamesv1.GetWishlistResponse{Games: protomap.Wishlist(items)}, nil
}
