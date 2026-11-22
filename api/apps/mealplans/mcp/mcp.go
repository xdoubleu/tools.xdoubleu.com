package mcp

import (
	"context"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"google.golang.org/protobuf/proto"

	mealplansconnect "tools.xdoubleu.com/apps/mealplans/connect"
	"tools.xdoubleu.com/apps/mealplans/internal/services"
	mealplansv1 "tools.xdoubleu.com/gen/mealplans/v1"
	"tools.xdoubleu.com/internal/platform/mcptools"
)

const mcpAppName = "mealplans"

type mcpGetPlanArgs struct {
	ID     string `json:"id"               jsonschema:"meal-plan id"`
	Offset int32  `json:"offset,omitempty" jsonschema:"week offset (0 = this week)"`
}

type mcpSuggestArgs struct {
	PlanID   string `json:"plan_id"   jsonschema:"meal-plan id"`
	MealDate string `json:"meal_date" jsonschema:"meal date (YYYY-MM-DD)"`
	MealSlot string `json:"meal_slot" jsonschema:"slot (breakfast|noon|evening)"`
}

// Register exposes the app's read-only RPCs as MCP tools, scoped to the
// caller's family.
func Register(srv *mcp.Server, plans *services.PlanService) {
	h := &tools{rpc: mealplansconnect.NewHandler(plans)}

	mcptools.AddReadTool(srv, mcpAppName, "mealplans_list_plans",
		"The meal plans in the user's family; a user with none gets a default plan.",
		h.listPlans)
	mcptools.AddReadTool(srv, mcpAppName, "mealplans_get_plan",
		"A single meal plan's week of meals; recipe meals carry recipe_ref {id, name}.",
		h.getPlan)
	mcptools.AddReadTool(srv, mcpAppName, "mealplans_suggest_recipes",
		"Recipe suggestions for a given plan slot.", h.suggestRecipes)
}

type tools struct {
	rpc *mealplansconnect.Handler
}

func (h *tools) listPlans(
	ctx context.Context, _ mcptools.NoArgs,
) (proto.Message, error) {
	return mcptools.Unwrap(h.rpc.ListPlans(ctx, &mealplansv1.ListPlansRequest{}))
}

func (h *tools) getPlan(
	ctx context.Context, args mcpGetPlanArgs,
) (proto.Message, error) {
	return mcptools.Unwrap(
		h.rpc.GetPlan(ctx, &mealplansv1.GetPlanRequest{Id: args.ID, Offset: args.Offset}),
	)
}

func (h *tools) suggestRecipes(
	ctx context.Context, args mcpSuggestArgs,
) (proto.Message, error) {
	return mcptools.Unwrap(h.rpc.SuggestRecipes(ctx, &mealplansv1.SuggestRecipesRequest{
		PlanId:   args.PlanID,
		MealDate: args.MealDate,
		MealSlot: args.MealSlot,
	}))
}
