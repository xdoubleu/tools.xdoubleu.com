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

// mcpOwnerArgs selects whose list to read: empty is the caller's own list; a
// non-empty owner must be a list shared with the caller.
type mcpOwnerArgs struct {
	OwnerUserID string `json:"owner_user_id,omitempty" jsonschema:"owner; empty=own list"`
}

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
// apps MCP server. List data is scoped to the caller (own or shared lists);
// stores are always the caller's own.
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
		"The user-defined shopping categories.", h.mcpListCategories)
	mcptools.AddReadTool(srv, mcpAppName, "shoppinglist_list_stores",
		"The caller's own stores.", h.mcpListStores)
	mcptools.AddReadTool(srv, mcpAppName, "shoppinglist_get_store_categories",
		"A store's categories in walk-through order.", h.mcpGetStoreCategories)
	mcptools.AddReadTool(srv, mcpAppName, "shoppinglist_list_item_names",
		"The known item-name catalog and their category mapping.",
		h.mcpListItemNames)
	mcptools.AddReadTool(srv, mcpAppName, "shoppinglist_list_item_categories",
		"The item-name to category assignments.", h.mcpListItemCategories)
	mcptools.AddReadTool(srv, mcpAppName, "shoppinglist_list_shares",
		"The users the caller has shared their list with.", h.mcpListShares)
	mcptools.AddReadTool(srv, mcpAppName, "shoppinglist_list_accessible_lists",
		"Lists the caller can act on: their own plus lists shared with them.",
		h.mcpListAccessibleLists)
}

func (h *shoppingConnectHandler) mcpGetCustomList(
	ctx context.Context, args mcpOwnerArgs,
) (proto.Message, error) {
	return mcptools.Unwrap(h.GetCustomList(ctx, connect.NewRequest(
		&shoppinglistv1.GetCustomListRequest{OwnerUserId: args.OwnerUserID},
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
	ctx context.Context, args mcpOwnerArgs,
) (proto.Message, error) {
	return mcptools.Unwrap(h.ListCategories(ctx, connect.NewRequest(
		&shoppinglistv1.ListCategoriesRequest{OwnerUserId: args.OwnerUserID},
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
	ctx context.Context, args mcpOwnerArgs,
) (proto.Message, error) {
	return mcptools.Unwrap(h.ListItemNames(ctx, connect.NewRequest(
		&shoppinglistv1.ListItemNamesRequest{OwnerUserId: args.OwnerUserID},
	)))
}

func (h *shoppingConnectHandler) mcpListItemCategories(
	ctx context.Context, args mcpOwnerArgs,
) (proto.Message, error) {
	return mcptools.Unwrap(h.ListItemCategories(ctx, connect.NewRequest(
		&shoppinglistv1.ListItemCategoriesRequest{OwnerUserId: args.OwnerUserID},
	)))
}

func (h *shoppingConnectHandler) mcpListShares(
	ctx context.Context, _ mcptools.NoArgs,
) (proto.Message, error) {
	return mcptools.Unwrap(h.ListShoppingListShares(ctx, connect.NewRequest(
		&shoppinglistv1.ListShoppingListSharesRequest{},
	)))
}

func (h *shoppingConnectHandler) mcpListAccessibleLists(
	ctx context.Context, _ mcptools.NoArgs,
) (proto.Message, error) {
	return mcptools.Unwrap(h.ListAccessibleLists(ctx, connect.NewRequest(
		&shoppinglistv1.ListAccessibleListsRequest{},
	)))
}
