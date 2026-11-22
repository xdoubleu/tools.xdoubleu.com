package mcp

import (
	"context"
	"time"

	"connectrpc.com/connect/v2"
	"github.com/google/uuid"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"google.golang.org/protobuf/proto"

	"tools.xdoubleu.com/apps/mealplans/internal/protomap"
	"tools.xdoubleu.com/apps/mealplans/internal/services"
	"tools.xdoubleu.com/internal/connecttools"
	"tools.xdoubleu.com/internal/kit/constants"
	"tools.xdoubleu.com/internal/kit/contexttools"
	sharedmodels "tools.xdoubleu.com/internal/models"
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
	h := &tools{plans: plans}

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
	plans *services.PlanService
}

func userID(ctx context.Context) (string, error) {
	user := contexttools.GetValue[sharedmodels.User](ctx, constants.UserContextKey)
	if user == nil {
		return "", connect.Errorf(connect.CodeUnauthenticated, "user not authenticated")
	}
	return user.ID, nil
}

func parsePlanID(raw string) (uuid.UUID, error) {
	id, err := uuid.Parse(raw)
	if err != nil {
		return uuid.Nil, connect.Errorf(connect.CodeInvalidArgument, "invalid plan ID")
	}
	return id, nil
}

func (h *tools) listPlans(
	ctx context.Context, _ mcptools.NoArgs,
) (proto.Message, error) {
	uid, err := userID(ctx)
	if err != nil {
		return nil, err
	}
	list, hasMore, err := h.plans.List(ctx, uid, 0, 0)
	if err != nil {
		return nil, connecttools.MapError(err)
	}
	return protomap.ListPlans(list, hasMore), nil
}

func (h *tools) getPlan(
	ctx context.Context, args mcpGetPlanArgs,
) (proto.Message, error) {
	uid, err := userID(ctx)
	if err != nil {
		return nil, err
	}
	id, err := parsePlanID(args.ID)
	if err != nil {
		return nil, err
	}
	offset := int(args.Offset)
	plan, windowStart, windowEnd, err := h.plans.GetWithWeek(ctx, id, uid, offset)
	if err != nil {
		return nil, connecttools.MapError(err)
	}
	return protomap.GetPlan(plan, uid, offset, windowStart, windowEnd), nil
}

func (h *tools) suggestRecipes(
	ctx context.Context, args mcpSuggestArgs,
) (proto.Message, error) {
	uid, err := userID(ctx)
	if err != nil {
		return nil, err
	}
	planID, err := parsePlanID(args.PlanID)
	if err != nil {
		return nil, err
	}
	mealDate, err := time.Parse(time.DateOnly, args.MealDate)
	if err != nil {
		return nil, connect.Errorf(connect.CodeInvalidArgument, "invalid meal date")
	}
	suggestions, err := h.plans.SuggestRecipes(ctx, planID, uid, mealDate, args.MealSlot)
	if err != nil {
		return nil, connecttools.MapError(err)
	}
	return protomap.SuggestRecipes(suggestions), nil
}
