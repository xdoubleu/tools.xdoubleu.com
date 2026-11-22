package mcp

import (
	"context"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"google.golang.org/protobuf/proto"

	recipesconnect "tools.xdoubleu.com/apps/recipes/connect"
	"tools.xdoubleu.com/apps/recipes/internal/services"
	recipesv1 "tools.xdoubleu.com/gen/recipes/v1"
	"tools.xdoubleu.com/internal/platform/mcptools"
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

type tools struct {
	reads   *recipesconnect.Handler
	recipes *services.RecipeService
	videos  *services.VideoService
}

// Register exposes the app's read RPCs, a video-text fetch and
// draft-preserving create/update write tools, scoped to the caller's family
// (docs/adr-0026-recipes-mcp-write-tool.md).
func Register(
	srv *mcp.Server,
	recipes *services.RecipeService,
	videos *services.VideoService,
) {
	h := &tools{
		reads:   recipesconnect.NewHandler(recipes),
		recipes: recipes,
		videos:  videos,
	}

	mcptools.AddReadTool(srv, mcpAppName, "recipes_list_recipes",
		"Recipes in the user's family recipe book, sorted by name. "+
			"Summaries only: instructions and ingredients are omitted, use "+
			"recipes_get_recipe for those. "+
			"Page with limit/offset while has_more is true.", h.mcpListRecipes)
	mcptools.AddReadTool(srv, mcpAppName, "recipes_get_recipe",
		"A single recipe with its ingredients scaled to the requested servings.",
		h.mcpGetRecipe)
	mcptools.AddWriteTool(srv, mcpAppName, "recipes_create_recipe",
		mcpCreateRecipeDescription, h.mcpCreateRecipe)
	mcptools.AddWriteTool(srv, mcpAppName, "recipes_update_recipe",
		mcpUpdateRecipeDescription, h.mcpUpdateRecipe)
	mcptools.AddReadTool(srv, mcpAppName, "recipes_fetch_video",
		mcpFetchVideoDescription, h.mcpFetchVideo)
}

func (h *tools) mcpListRecipes(
	ctx context.Context, args mcpListRecipesArgs,
) (proto.Message, error) {
	return mcptools.Unwrap(
		h.reads.ListRecipes(
			ctx,
			&recipesv1.ListRecipesRequest{Limit: args.Limit, Offset: args.Offset},
		),
	)
}

func (h *tools) mcpGetRecipe(
	ctx context.Context, args mcpRecipeArgs,
) (proto.Message, error) {
	return mcptools.Unwrap(
		h.reads.GetRecipe(
			ctx,
			&recipesv1.GetRecipeRequest{Id: args.ID, Servings: args.Servings},
		),
	)
}
