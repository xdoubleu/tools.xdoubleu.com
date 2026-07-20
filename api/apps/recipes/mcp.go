package recipes

import (
	"context"

	"connectrpc.com/connect"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"google.golang.org/protobuf/proto"

	recipesv1 "tools.xdoubleu.com/gen/recipes/v1"
	"tools.xdoubleu.com/internal/mcptools"
)

const mcpAppName = "recipes"

type mcpRecipeArgs struct {
	ID       string `json:"id"                 jsonschema:"recipe id"`
	Servings int32  `json:"servings,omitempty" jsonschema:"scale ingredients to servings"`
}

// RegisterMCPTools exposes the recipes app's read-only RPCs on the combined apps
// MCP server. Every tool returns recipes the calling user owns or has been
// shared.
func (a *Recipes) RegisterMCPTools(srv *mcp.Server) {
	h := &recipesConnectHandler{app: a}

	mcptools.AddReadTool(srv, mcpAppName, "recipes_list_recipes",
		"All recipes the user owns or has been shared.", h.mcpListRecipes)
	mcptools.AddReadTool(srv, mcpAppName, "recipes_get_recipe",
		"A single recipe with its ingredients scaled to the requested servings.",
		h.mcpGetRecipe)
	mcptools.AddReadTool(srv, mcpAppName, "recipes_list_recipe_book_shares",
		"The users the caller has shared their recipe book with.",
		h.mcpListRecipeBookShares)
}

func (h *recipesConnectHandler) mcpListRecipes(
	ctx context.Context, _ mcptools.NoArgs,
) (proto.Message, error) {
	return mcptools.Unwrap(h.ListRecipes(ctx, connect.NewRequest(
		&recipesv1.ListRecipesRequest{},
	)))
}

func (h *recipesConnectHandler) mcpGetRecipe(
	ctx context.Context, args mcpRecipeArgs,
) (proto.Message, error) {
	return mcptools.Unwrap(h.GetRecipe(ctx, connect.NewRequest(
		&recipesv1.GetRecipeRequest{Id: args.ID, Servings: args.Servings},
	)))
}

func (h *recipesConnectHandler) mcpListRecipeBookShares(
	ctx context.Context, _ mcptools.NoArgs,
) (proto.Message, error) {
	return mcptools.Unwrap(h.ListRecipeBookShares(ctx, connect.NewRequest(
		&recipesv1.ListRecipeBookSharesRequest{},
	)))
}
