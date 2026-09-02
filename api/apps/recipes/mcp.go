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
// MCP server. Every tool returns recipes belonging to the calling user's
// family.
func (a *Recipes) RegisterMCPTools(srv *mcp.Server) {
	h := &recipesConnectHandler{app: a}

	mcptools.AddReadTool(srv, mcpAppName, "recipes_list_recipes",
		"All recipes in the user's family recipe book.", h.mcpListRecipes)
	mcptools.AddReadTool(srv, mcpAppName, "recipes_get_recipe",
		"A single recipe with its ingredients scaled to the requested servings.",
		h.mcpGetRecipe)
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
