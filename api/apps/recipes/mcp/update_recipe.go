package mcp

import (
	"context"

	"connectrpc.com/connect/v2"
	"github.com/google/uuid"
	"google.golang.org/protobuf/proto"

	recipesconnect "tools.xdoubleu.com/apps/recipes/connect"
	"tools.xdoubleu.com/apps/recipes/internal/services"
)

// mcpUpdateRecipeDescription carries the edit conventions, so every MCP
// client gets them.
const mcpUpdateRecipeDescription = "Replace one recipe in the user's family " +
	"recipe book, preserving whether it is a draft.\n" +
	"Read the recipe first with recipes_get_recipe, then send the complete " +
	"recipe back with your changes; UpdateRecipe is a full replacement, so " +
	"omitting an ingredient or step removes it.\n" +
	"Conventions:\n" +
	"- meal_type: \"light_meal\" or \"main_meal\"; effort: \"low\" or " +
	"\"high\"; empty/omitted leaves the field uncategorised.\n" +
	"- \"x or y\" ingredients: one entry per option sharing the same " +
	"alternative_group label; the first is the default.\n" +
	"- group_name only for sub-components (a sauce, a pesto)."

type mcpUpdateRecipeArgs struct {
	ID           string             `json:"id"            jsonschema:"recipe id"`
	Name         string             `json:"name"          jsonschema:"recipe name"`
	Steps        []string           `json:"steps"         jsonschema:"numbered steps"`
	BaseServings int32              `json:"base_servings" jsonschema:"servings"`
	Ingredients  []mcpIngredientArg `json:"ingredients"   jsonschema:"in order"`
	MealType     string             `json:"meal_type"     jsonschema:"meal type"`
	Effort       string             `json:"effort"        jsonschema:"effort"`

	BatchServings *int32 `json:"batch_servings,omitempty" jsonschema:"batch size"`
}

// mcpUpdateRecipe preserves the recipe's current draft status: an agent can
// edit a recipe but never publish or unpublish it (adr-0026). The update is a
// full replacement of the recipe's content.
func (h *tools) mcpUpdateRecipe(
	ctx context.Context, args mcpUpdateRecipeArgs,
) (proto.Message, error) {
	userID, err := recipesconnect.CallerID(ctx)
	if err != nil {
		return nil, err
	}
	id, err := uuid.Parse(args.ID)
	if err != nil {
		return nil, connect.Errorf(connect.CodeInvalidArgument, "invalid recipe ID")
	}
	//nolint:exhaustruct // the draft flag is kept by the service
	in := services.RecipeInput{
		ID:            id,
		Name:          args.Name,
		Steps:         args.Steps,
		BaseServings:  int(args.BaseServings),
		BatchServings: batchServings(args.BatchServings),
		MealType:      args.MealType,
		Effort:        args.Effort,
		Ingredients:   ingredientInputs(args.Ingredients),
	}
	updated, err := h.recipes.UpdateKeepingDraft(ctx, userID, in)
	if err != nil {
		return nil, recipesconnect.MapError(err)
	}
	return recipesconnect.ScaledRecipeResponse(userID, updated), nil
}
