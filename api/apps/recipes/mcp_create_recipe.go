package recipes

import (
	"context"

	"connectrpc.com/connect"
	"google.golang.org/protobuf/proto"

	recipesv1 "tools.xdoubleu.com/gen/recipes/v1"
	"tools.xdoubleu.com/internal/mcptools"
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
	"- base_servings from the source, else 2."

type mcpIngredientArg struct {
	Name      string  `json:"name"                 jsonschema:"with descriptors"`
	Amount    float64 `json:"amount,omitempty"     jsonschema:"quantity"`
	Unit      string  `json:"unit,omitempty"       jsonschema:"empty if countable"`
	GroupName string  `json:"group_name,omitempty" jsonschema:"sub-component"`
}

type mcpCreateRecipeArgs struct {
	Name         string             `json:"name"          jsonschema:"recipe name"`
	Steps        []string           `json:"steps"         jsonschema:"numbered steps"`
	BaseServings int32              `json:"base_servings" jsonschema:"servings"`
	Ingredients  []mcpIngredientArg `json:"ingredients"   jsonschema:"in order"`

	BatchServings *int32 `json:"batch_servings,omitempty" jsonschema:"batch size"`
}

// mcpCreateRecipe always creates a draft; the user un-drafts it in the app.
func (h *recipesConnectHandler) mcpCreateRecipe(
	ctx context.Context, args mcpCreateRecipeArgs,
) (proto.Message, error) {
	n := len(args.Ingredients)
	req := &recipesv1.CreateRecipeRequest{
		Name:                 args.Name,
		Steps:                args.Steps,
		BaseServings:         args.BaseServings,
		BatchServings:        args.BatchServings,
		IngredientNames:      make([]string, n),
		IngredientAmounts:    make([]float64, n),
		IngredientUnits:      make([]string, n),
		IngredientGroupNames: make([]string, n),
		IsDraft:              true,
	}
	for i, ing := range args.Ingredients {
		req.IngredientNames[i] = ing.Name
		req.IngredientAmounts[i] = ing.Amount
		req.IngredientUnits[i] = ing.Unit
		req.IngredientGroupNames[i] = ing.GroupName
	}
	return mcptools.Unwrap(h.CreateRecipe(ctx, connect.NewRequest(req)))
}
