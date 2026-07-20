package games

import (
	"context"

	"connectrpc.com/connect"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"google.golang.org/protobuf/proto"

	gamesv1 "tools.xdoubleu.com/gen/games/v1"
	"tools.xdoubleu.com/internal/mcptools"
)

const mcpAppName = "games"

type gameIDArgs struct {
	GameID int32 `json:"game_id" jsonschema:"games-app game id"`
}

type steamRangeArgs struct {
	DateStart string `json:"date_start,omitempty" jsonschema:"window start (YYYY-MM-DD)"`
	DateEnd   string `json:"date_end,omitempty"   jsonschema:"window end (YYYY-MM-DD)"`
}

type bucketArgs struct {
	Bucket int32 `json:"bucket,omitempty" jsonschema:"distribution bucket (0-9)"`
}

// RegisterMCPTools exposes the games app's read-only RPCs on the combined apps
// MCP server. Every tool returns the calling user's own Steam data.
func (a *Games) RegisterMCPTools(srv *mcp.Server) {
	h := &gamesConnectHandler{app: a}

	mcptools.AddReadTool(srv, mcpAppName, "games_get_steam",
		"Steam backlog overview: not-started/in-progress/completed games, "+
			"backlog size, and completion-rate progress.", h.mcpGetSteam)
	mcptools.AddReadTool(srv, mcpAppName, "games_get_steam_game",
		"A single Steam game with its achievements.", h.mcpGetSteamGame)
	mcptools.AddReadTool(srv, mcpAppName, "games_get_steam_distribution",
		"Games in one completion-rate distribution bucket.",
		h.mcpGetSteamDistribution)
	mcptools.AddReadTool(srv, mcpAppName, "games_get_recently_active_games",
		"Games with recently unlocked achievements.",
		h.mcpGetRecentlyActiveGames)
	mcptools.AddReadTool(srv, mcpAppName, "games_get_integrations",
		"The user's games-app integration settings (Steam user id).",
		h.mcpGetIntegrations)
}

func (h *gamesConnectHandler) mcpGetSteam(
	ctx context.Context, args steamRangeArgs,
) (proto.Message, error) {
	return mcptools.Unwrap(h.GetSteam(ctx, connect.NewRequest(
		&gamesv1.GetSteamRequest{DateStart: args.DateStart, DateEnd: args.DateEnd},
	)))
}

func (h *gamesConnectHandler) mcpGetSteamGame(
	ctx context.Context, args gameIDArgs,
) (proto.Message, error) {
	return mcptools.Unwrap(h.GetSteamGame(ctx, connect.NewRequest(
		&gamesv1.GetSteamGameRequest{GameId: args.GameID},
	)))
}

func (h *gamesConnectHandler) mcpGetSteamDistribution(
	ctx context.Context, args bucketArgs,
) (proto.Message, error) {
	return mcptools.Unwrap(h.GetSteamDistribution(ctx, connect.NewRequest(
		&gamesv1.GetSteamDistributionRequest{Bucket: args.Bucket},
	)))
}

func (h *gamesConnectHandler) mcpGetRecentlyActiveGames(
	ctx context.Context, _ mcptools.NoArgs,
) (proto.Message, error) {
	return mcptools.Unwrap(h.GetRecentlyActiveGames(ctx, connect.NewRequest(
		&gamesv1.GetRecentlyActiveGamesRequest{},
	)))
}

func (h *gamesConnectHandler) mcpGetIntegrations(
	ctx context.Context, _ mcptools.NoArgs,
) (proto.Message, error) {
	return mcptools.Unwrap(h.GetIntegrations(ctx, connect.NewRequest(
		&gamesv1.GetIntegrationsRequest{},
	)))
}
