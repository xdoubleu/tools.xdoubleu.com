package recipes

import (
	"context"
	"fmt"

	"google.golang.org/protobuf/proto"

	recipesv1 "tools.xdoubleu.com/gen/recipes/v1"
	"tools.xdoubleu.com/internal/platform/mcptools"
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
func (h *recipesConnectHandler) mcpUpdateRecipe(
	ctx context.Context, args mcpUpdateRecipeArgs,
) (proto.Message, error) {
	// Read the current draft flag so the update keeps it as-is.
	existingMsg, err := mcptools.Unwrap(
		h.GetRecipe(ctx, &recipesv1.GetRecipeRequest{Id: args.ID}),
	)
	if err != nil {
		return nil, err
	}
	existingResp, ok := existingMsg.(*recipesv1.GetRecipeResponse)
	if !ok {
		return nil, fmt.Errorf("unexpected GetRecipe response %T", existingMsg)
	}

	n := len(args.Ingredients)
	req := &recipesv1.UpdateRecipeRequest{
		Id:                          args.ID,
		Name:                        args.Name,
		Steps:                       args.Steps,
		BaseServings:                args.BaseServings,
		BatchServings:               args.BatchServings,
		MealType:                    args.MealType,
		Effort:                      args.Effort,
		IsDraft:                     existingResp.Recipe.IsDraft,
		IngredientNames:             make([]string, n),
		IngredientAmounts:           make([]float64, n),
		IngredientUnits:             make([]string, n),
		IngredientGroupNames:        make([]string, n),
		IngredientAlternativeGroups: make([]string, n),
	}
	for i, ing := range args.Ingredients {
		req.IngredientNames[i] = ing.Name
		req.IngredientAmounts[i] = ing.Amount
		req.IngredientUnits[i] = ing.Unit
		req.IngredientGroupNames[i] = ing.GroupName
		req.IngredientAlternativeGroups[i] = ing.AlternativeGroup
	}
	if _, err = h.UpdateRecipe(ctx, req); err != nil {
		return nil, err
	}
	return mcptools.Unwrap(
		h.GetRecipe(ctx, &recipesv1.GetRecipeRequest{Id: args.ID}),
	)
}
