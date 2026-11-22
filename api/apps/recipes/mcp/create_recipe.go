package mcp

import (
	"context"

	"google.golang.org/protobuf/proto"

	recipesconnect "tools.xdoubleu.com/apps/recipes/connect"
	"tools.xdoubleu.com/apps/recipes/internal/services"
	recipesv1 "tools.xdoubleu.com/gen/recipes/v1"
)

// mcpCreateRecipeDescription carries the import conventions, so every MCP
// client gets them.
const mcpCreateRecipeDescription = "Save one recipe to the user's family " +
	"recipe book as a draft for them to review.\n" +
	"Before importing, call recipes_list_recipes and skip recipes whose name " +
	"exactly matches an existing one.\n" +
	"PDF/DOCX: one call per recipe in the document. Recipe sites: prefer the " +
	"page's schema.org Recipe data; if the page can't be read (e.g. Colruyt, " +
	"AH block bots), ask the user to paste the text or a screenshot.\n" +
	"Conventions:\n" +
	"- Flemish Dutch; translate name, ingredients and steps from other " +
	"languages.\n" +
	"- Units: g, kg, ml, dl, l, el, tl, teen, pak, pakje, zakje, pot, blikje, " +
	"bokaal. Countable items: empty unit, never \"stuk\". Convert cups/oz to " +
	"metric.\n" +
	"- Omit \"to taste\" seasoning (peper, zout, nootmuskaat) from " +
	"ingredients; mention it in a step.\n" +
	"- Descriptors go in the ingredient name (\"kipfilet in blokjes\"), no " +
	"comma prep notes.\n" +
	"- Steps numbered \"1. ...\", one per entry; last entry \"Bron: <url>\" " +
	"when imported from a URL.\n" +
	"- group_name only for sub-components (a sauce, a pesto).\n" +
	"- \"x or y\" ingredients: one entry per option sharing the same " +
	"alternative_group label; the first is the default.\n" +
	"- base_servings from the source, else 2."

type mcpIngredientArg struct {
	Name      string  `json:"name"                 jsonschema:"with descriptors"`
	Amount    float64 `json:"amount,omitempty"     jsonschema:"quantity"`
	Unit      string  `json:"unit,omitempty"       jsonschema:"empty if countable"`
	GroupName string  `json:"group_name,omitempty" jsonschema:"sub-component"`

	AlternativeGroup string `json:"alternative_group,omitempty" jsonschema:"x-or-y label"`
}

type mcpCreateRecipeArgs struct {
	Name         string             `json:"name"          jsonschema:"recipe name"`
	Steps        []string           `json:"steps"         jsonschema:"numbered steps"`
	BaseServings int32              `json:"base_servings" jsonschema:"servings"`
	Ingredients  []mcpIngredientArg `json:"ingredients"   jsonschema:"in order"`

	BatchServings *int32 `json:"batch_servings,omitempty" jsonschema:"batch size"`
}

// mcpCreateRecipe always creates a draft; the user un-drafts it in the app.
func (h *tools) mcpCreateRecipe(
	ctx context.Context, args mcpCreateRecipeArgs,
) (proto.Message, error) {
	userID, err := recipesconnect.CallerID(ctx)
	if err != nil {
		return nil, err
	}
	//nolint:exhaustruct // new recipes have no ID, categories or draft input
	in := services.RecipeInput{
		Name:          args.Name,
		Steps:         args.Steps,
		BaseServings:  int(args.BaseServings),
		BatchServings: batchServings(args.BatchServings),
		Ingredients:   ingredientInputs(args.Ingredients),
	}
	created, err := h.recipes.Create(ctx, userID, in, services.AsAgentDraft())
	if err != nil {
		return nil, recipesconnect.MapError(err)
	}
	return &recipesv1.CreateRecipeResponse{
		Recipe: recipesconnect.ProtoRecipe(created),
	}, nil
}

func batchServings(v *int32) *int {
	if v == nil {
		return nil
	}
	n := int(*v)
	return &n
}

func ingredientInputs(args []mcpIngredientArg) []services.IngredientInput {
	out := make([]services.IngredientInput, len(args))
	for i, ing := range args {
		out[i] = services.IngredientInput{
			Name:             ing.Name,
			Amount:           ing.Amount,
			Unit:             ing.Unit,
			GroupName:        ing.GroupName,
			AlternativeGroup: ing.AlternativeGroup,
		}
	}
	return out
}
