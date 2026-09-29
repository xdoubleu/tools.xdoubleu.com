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

type mcpListRecipesArgs struct {
	Limit  int32 `json:"limit,omitempty"  jsonschema:"max recipes, default 50"`
	Offset int32 `json:"offset,omitempty" jsonschema:"pagination offset"`
}

type mcpRecipeArgs struct {
	ID       string `json:"id"                 jsonschema:"recipe id"`
	Servings int32  `json:"servings,omitempty" jsonschema:"scale ingredients to servings"`
}

// RegisterMCPTools exposes the app's read RPCs and a draft-only create as MCP
// tools, scoped to the caller's family (docs/adr-0026-recipes-mcp-write-tool.md).
func (a *Recipes) RegisterMCPTools(srv *mcp.Server) {
	h := &recipesConnectHandler{app: a}

	mcptools.AddReadTool(srv, mcpAppName, "recipes_list_recipes",
		"Recipes in the user's family recipe book, sorted by name. "+
			"Page with limit/offset while has_more is true.", h.mcpListRecipes)
	mcptools.AddReadTool(srv, mcpAppName, "recipes_get_recipe",
		"A single recipe with its ingredients scaled to the requested servings.",
		h.mcpGetRecipe)
	mcptools.AddWriteTool(srv, mcpAppName, "recipes_create_recipe",
		mcpCreateRecipeDescription, h.mcpCreateRecipe)
	mcptools.AddReadTool(srv, mcpAppName, "recipes_fetch_video",
		mcpFetchVideoDescription, h.mcpFetchVideo)
}

func (h *recipesConnectHandler) mcpListRecipes(
	ctx context.Context, args mcpListRecipesArgs,
) (proto.Message, error) {
	return mcptools.Unwrap(h.ListRecipes(ctx, connect.NewRequest(
		&recipesv1.ListRecipesRequest{Limit: args.Limit, Offset: args.Offset},
	)))
}

func (h *recipesConnectHandler) mcpGetRecipe(
	ctx context.Context, args mcpRecipeArgs,
) (proto.Message, error) {
	return mcptools.Unwrap(h.GetRecipe(ctx, connect.NewRequest(
		&recipesv1.GetRecipeRequest{Id: args.ID, Servings: args.Servings},
	)))
}
