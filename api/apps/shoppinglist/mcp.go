package shoppinglist

import (
	"context"

	"connectrpc.com/connect"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"google.golang.org/protobuf/proto"

	shoppinglistv1 "tools.xdoubleu.com/gen/shoppinglist/v1"
	"tools.xdoubleu.com/internal/mcptools"
)

const mcpAppName = "shoppinglist"

type mcpPlanIDArgs struct {
	PlanID string `json:"plan_id" jsonschema:"meal-plan id"`
}

type mcpExportArgs struct {
	PlanID         string   `json:"plan_id"                   jsonschema:"meal-plan id"`
	ExcludedGroups []string `json:"excluded_groups,omitempty" jsonschema:"groups to skip"`
}

type mcpStoreIDArgs struct {
	StoreID string `json:"store_id" jsonschema:"store id"`
}

// RegisterMCPTools exposes the shoppinglist app's read-only RPCs on the combined
// apps MCP server. List data is scoped to the caller's family; stores are
// always the caller's own.
func (a *ShoppingList) RegisterMCPTools(srv *mcp.Server) {
	h := &shoppingConnectHandler{app: a}

	mcptools.AddReadTool(srv, mcpAppName, "shoppinglist_get_custom_list",
		"The custom (manually added) shopping-list items.", h.mcpGetCustomList)
	mcptools.AddReadTool(srv, mcpAppName, "shoppinglist_get_meal_plan_export_items",
		"Aggregated shopping items for a meal plan.", h.mcpGetMealPlanExportItems)
	mcptools.AddReadTool(srv, mcpAppName, "shoppinglist_get_plan_ingredient_groups",
		"The recipe/ingredient groups available to exclude for a plan.",
		h.mcpGetPlanIngredientGroups)
	mcptools.AddReadTool(srv, mcpAppName, "shoppinglist_list_categories",
		"The family's shopping categories.", h.mcpListCategories)
	mcptools.AddReadTool(srv, mcpAppName, "shoppinglist_list_stores",
		"The caller's own stores.", h.mcpListStores)
	mcptools.AddReadTool(srv, mcpAppName, "shoppinglist_get_store_categories",
		"A store's categories in walk-through order.", h.mcpGetStoreCategories)
	mcptools.AddReadTool(srv, mcpAppName, "shoppinglist_list_item_names",
		"The known item-name catalog and their category mapping.",
		h.mcpListItemNames)
	mcptools.AddReadTool(srv, mcpAppName, "shoppinglist_list_item_categories",
		"The item-name to category assignments.", h.mcpListItemCategories)
}

func (h *shoppingConnectHandler) mcpGetCustomList(
	ctx context.Context, _ mcptools.NoArgs,
) (proto.Message, error) {
	return mcptools.Unwrap(h.GetCustomList(ctx, connect.NewRequest(
		&shoppinglistv1.GetCustomListRequest{},
	)))
}

func (h *shoppingConnectHandler) mcpGetMealPlanExportItems(
	ctx context.Context, args mcpExportArgs,
) (proto.Message, error) {
	return mcptools.Unwrap(h.GetMealPlanExportItems(ctx, connect.NewRequest(
		&shoppinglistv1.GetMealPlanExportItemsRequest{
			PlanId:         args.PlanID,
			ExcludedGroups: args.ExcludedGroups,
		},
	)))
}

func (h *shoppingConnectHandler) mcpGetPlanIngredientGroups(
	ctx context.Context, args mcpPlanIDArgs,
) (proto.Message, error) {
	return mcptools.Unwrap(h.GetPlanIngredientGroups(ctx, connect.NewRequest(
		&shoppinglistv1.GetPlanIngredientGroupsRequest{PlanId: args.PlanID},
	)))
}

func (h *shoppingConnectHandler) mcpListCategories(
	ctx context.Context, _ mcptools.NoArgs,
) (proto.Message, error) {
	return mcptools.Unwrap(h.ListCategories(ctx, connect.NewRequest(
		&shoppinglistv1.ListCategoriesRequest{},
	)))
}

func (h *shoppingConnectHandler) mcpListStores(
	ctx context.Context, _ mcptools.NoArgs,
) (proto.Message, error) {
	return mcptools.Unwrap(h.ListStores(ctx, connect.NewRequest(
		&shoppinglistv1.ListStoresRequest{},
	)))
}

func (h *shoppingConnectHandler) mcpGetStoreCategories(
	ctx context.Context, args mcpStoreIDArgs,
) (proto.Message, error) {
	return mcptools.Unwrap(h.GetStoreCategories(ctx, connect.NewRequest(
		&shoppinglistv1.GetStoreCategoriesRequest{StoreId: args.StoreID},
	)))
}

func (h *shoppingConnectHandler) mcpListItemNames(
	ctx context.Context, _ mcptools.NoArgs,
) (proto.Message, error) {
	return mcptools.Unwrap(h.ListItemNames(ctx, connect.NewRequest(
		&shoppinglistv1.ListItemNamesRequest{},
	)))
}

func (h *shoppingConnectHandler) mcpListItemCategories(
	ctx context.Context, _ mcptools.NoArgs,
) (proto.Message, error) {
	return mcptools.Unwrap(h.ListItemCategories(ctx, connect.NewRequest(
		&shoppinglistv1.ListItemCategoriesRequest{},
	)))
}
