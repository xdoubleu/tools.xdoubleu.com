package mealplans

import (
	"context"
	"errors"

	"connectrpc.com/connect"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"google.golang.org/protobuf/proto"

	mealplansv1 "tools.xdoubleu.com/gen/mealplans/v1"
	"tools.xdoubleu.com/internal/mcptools"
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

// RegisterMCPTools exposes the mealplans app's read-only RPCs on the combined
// apps MCP server. Every tool returns plans in the calling user's family.
func (a *MealPlans) RegisterMCPTools(srv *mcp.Server) {
	h := &mealplansConnectHandler{app: a}

	mcptools.AddReadTool(srv, mcpAppName, "mealplans_list_plans",
		"The meal plans in the user's family.", h.mcpListPlans)
	mcptools.AddReadTool(srv, mcpAppName, "mealplans_get_plan",
		"A single meal plan's week of meals with the referenced recipes.",
		h.mcpGetPlan)
	mcptools.AddReadTool(srv, mcpAppName, "mealplans_suggest_recipes",
		"Recipe suggestions for a given plan slot.", h.mcpSuggestRecipes)
}

// mcpListPlans lists the user's plans directly through the service layer, so it
// never triggers the default-plan creation the ListPlans RPC does on an empty
// account — keeping the tool strictly read-only.
func (h *mealplansConnectHandler) mcpListPlans(
	ctx context.Context, _ mcptools.NoArgs,
) (proto.Message, error) {
	user := getUser(ctx)
	if user == nil {
		return nil, connect.NewError(
			connect.CodeUnauthenticated, errors.New("user not authenticated"),
		)
	}
	list, hasMore, err := h.app.services.Plans.List(ctx, user.ID, 0, 0)
	if err != nil {
		return nil, mapError(err)
	}
	return &mealplansv1.ListPlansResponse{
		Plans:   protoPlans(list),
		HasMore: hasMore,
	}, nil
}

func (h *mealplansConnectHandler) mcpGetPlan(
	ctx context.Context, args mcpGetPlanArgs,
) (proto.Message, error) {
	return mcptools.Unwrap(h.GetPlan(ctx, connect.NewRequest(
		&mealplansv1.GetPlanRequest{Id: args.ID, Offset: args.Offset},
	)))
}

func (h *mealplansConnectHandler) mcpSuggestRecipes(
	ctx context.Context, args mcpSuggestArgs,
) (proto.Message, error) {
	return mcptools.Unwrap(h.SuggestRecipes(ctx, connect.NewRequest(
		&mealplansv1.SuggestRecipesRequest{
			PlanId:   args.PlanID,
			MealDate: args.MealDate,
			MealSlot: args.MealSlot,
		},
	)))
}
