package mcp

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"google.golang.org/protobuf/proto"

	fmtamount "tools.xdoubleu.com/apps/shoppinglist/internal/amount"
	"tools.xdoubleu.com/apps/shoppinglist/internal/services"
	shoppinglistv1 "tools.xdoubleu.com/gen/shoppinglist/v1"
	"tools.xdoubleu.com/internal/kit/constants"
	"tools.xdoubleu.com/internal/kit/contexttools"
	sharedmodels "tools.xdoubleu.com/internal/models"
	"tools.xdoubleu.com/internal/platform/mcptools"
)

const mcpAppName = "shoppinglist"

type planIDArgs struct {
	PlanID string `json:"plan_id" jsonschema:"meal-plan id"`
}

type exportArgs struct {
	PlanID         string   `json:"plan_id"                   jsonschema:"meal-plan id"`
	ExcludedGroups []string `json:"excluded_groups,omitempty" jsonschema:"groups to skip"`
}

type storeIDArgs struct {
	StoreID string `json:"store_id" jsonschema:"store id"`
}

type tools struct {
	shopping *services.ShoppingService
}

// Register exposes the app's read-only RPCs as MCP tools, scoped to the
// caller's family (stores: the caller's own).
func Register(srv *mcp.Server, shopping *services.ShoppingService) {
	h := &tools{shopping: shopping}

	mcptools.AddReadTool(srv, mcpAppName, "shoppinglist_get_custom_list",
		"The custom (manually added) shopping-list items.", h.getCustomList)
	mcptools.AddReadTool(srv, mcpAppName, "shoppinglist_get_meal_plan_export_items",
		"Aggregated shopping items for a meal plan.", h.getMealPlanExportItems)
	mcptools.AddReadTool(srv, mcpAppName, "shoppinglist_get_plan_ingredient_groups",
		"The recipe/ingredient groups available to exclude for a plan.",
		h.getPlanIngredientGroups)
	mcptools.AddReadTool(srv, mcpAppName, "shoppinglist_list_categories",
		"The family's shopping categories.", h.listCategories)
	mcptools.AddReadTool(srv, mcpAppName, "shoppinglist_list_stores",
		"The caller's own stores.", h.listStores)
	mcptools.AddReadTool(srv, mcpAppName, "shoppinglist_get_store_categories",
		"A store's categories in walk-through order.", h.getStoreCategories)
	mcptools.AddReadTool(srv, mcpAppName, "shoppinglist_list_item_names",
		"The known item-name catalog, their category mapping and merges.",
		h.listItemNames)
	mcptools.AddReadTool(srv, mcpAppName, "shoppinglist_list_item_categories",
		"The item-name to category assignments.", h.listItemCategories)
	mcptools.AddReadTool(srv, mcpAppName, "shoppinglist_list_ingredient_units",
		"Units the family's recipes use per (merged) ingredient name.",
		h.listIngredientUnits)
}

func callerID(ctx context.Context) (string, error) {
	user := contexttools.GetValue[sharedmodels.User](ctx, constants.UserContextKey)
	if user == nil {
		return "", errors.New("user not authenticated")
	}
	return user.ID, nil
}

func (h *tools) getCustomList(
	ctx context.Context, _ mcptools.NoArgs,
) (proto.Message, error) {
	userID, err := callerID(ctx)
	if err != nil {
		return nil, err
	}
	items, err := h.shopping.GetCustomList(ctx, userID)
	if err != nil {
		return nil, err
	}
	pb := make([]*shoppinglistv1.ShoppingItem, len(items))
	for i, item := range items {
		pb[i] = &shoppinglistv1.ShoppingItem{
			Id:     item.ID,
			Name:   item.Name,
			Amount: fmtamount.Format(item.Amount),
			Unit:   item.Unit,
		}
	}
	return &shoppinglistv1.GetCustomListResponse{Items: pb}, nil
}

func (h *tools) getMealPlanExportItems(
	ctx context.Context, args exportArgs,
) (proto.Message, error) {
	userID, err := callerID(ctx)
	if err != nil {
		return nil, err
	}
	planID, err := uuid.Parse(args.PlanID)
	if err != nil {
		return nil, errors.New("invalid plan ID")
	}
	today, end, pastSlots := services.ExportWindow(time.Now().UTC())
	items, err := h.shopping.GetMealPlanExportItems(
		ctx, planID, userID, today, end, pastSlots, args.ExcludedGroups,
	)
	if err != nil {
		return nil, err
	}
	pb := make([]*shoppinglistv1.ShoppingItem, len(items))
	for i, item := range items {
		pb[i] = &shoppinglistv1.ShoppingItem{
			Name:       item.Name,
			Amount:     fmtamount.Format(item.Amount),
			Unit:       item.Unit,
			RecipeName: item.RecipeName,
			GroupName:  item.GroupName,
		}
	}
	return &shoppinglistv1.GetMealPlanExportItemsResponse{Items: pb}, nil
}

func (h *tools) getPlanIngredientGroups(
	ctx context.Context, args planIDArgs,
) (proto.Message, error) {
	userID, err := callerID(ctx)
	if err != nil {
		return nil, err
	}
	planID, err := uuid.Parse(args.PlanID)
	if err != nil {
		return nil, errors.New("invalid plan ID")
	}
	today, end, pastSlots := services.ExportWindow(time.Now().UTC())
	groups, err := h.shopping.GetPlanIngredientGroups(
		ctx, planID, userID, today, end, pastSlots,
	)
	if err != nil {
		return nil, err
	}
	pb := make([]*shoppinglistv1.PlanIngredientGroup, len(groups))
	for i, g := range groups {
		pb[i] = &shoppinglistv1.PlanIngredientGroup{
			RecipeName: g.RecipeName,
			GroupName:  g.GroupName,
		}
	}
	return &shoppinglistv1.GetPlanIngredientGroupsResponse{Groups: pb}, nil
}

func (h *tools) listCategories(
	ctx context.Context, _ mcptools.NoArgs,
) (proto.Message, error) {
	userID, err := callerID(ctx)
	if err != nil {
		return nil, err
	}
	categories, err := h.shopping.ListCategories(ctx, userID)
	if err != nil {
		return nil, err
	}
	pb := make([]*shoppinglistv1.Category, len(categories))
	for i, c := range categories {
		pb[i] = &shoppinglistv1.Category{Id: c.ID, Name: c.Name}
	}
	return &shoppinglistv1.ListCategoriesResponse{Categories: pb}, nil
}

func (h *tools) listStores(
	ctx context.Context, _ mcptools.NoArgs,
) (proto.Message, error) {
	userID, err := callerID(ctx)
	if err != nil {
		return nil, err
	}
	stores, err := h.shopping.ListStores(ctx, userID)
	if err != nil {
		return nil, err
	}
	pb := make([]*shoppinglistv1.Store, len(stores))
	for i, s := range stores {
		pb[i] = &shoppinglistv1.Store{Id: s.ID, Name: s.Name}
	}
	return &shoppinglistv1.ListStoresResponse{Stores: pb}, nil
}

func (h *tools) getStoreCategories(
	ctx context.Context, args storeIDArgs,
) (proto.Message, error) {
	storeID, err := uuid.Parse(args.StoreID)
	if err != nil {
		return nil, errors.New("invalid ID")
	}
	userID, err := callerID(ctx)
	if err != nil {
		return nil, err
	}
	categories, err := h.shopping.GetStoreCategories(ctx, userID, storeID)
	if err != nil {
		return nil, err
	}
	pb := make([]*shoppinglistv1.Category, len(categories))
	for i, c := range categories {
		pb[i] = &shoppinglistv1.Category{Id: c.ID, Name: c.Name}
	}
	return &shoppinglistv1.GetStoreCategoriesResponse{Categories: pb}, nil
}

func (h *tools) listItemNames(
	ctx context.Context, _ mcptools.NoArgs,
) (proto.Message, error) {
	userID, err := callerID(ctx)
	if err != nil {
		return nil, err
	}
	names, err := h.shopping.ListItemNames(ctx, userID)
	if err != nil {
		return nil, err
	}
	pb := make([]*shoppinglistv1.ItemName, len(names))
	for i, n := range names {
		pb[i] = &shoppinglistv1.ItemName{
			Name:       n.Name,
			CategoryId: n.CategoryID,
			Excluded:   n.Excluded,
			MergedInto: n.MergedInto,
		}
	}
	return &shoppinglistv1.ListItemNamesResponse{Names: pb}, nil
}

func (h *tools) listItemCategories(
	ctx context.Context, _ mcptools.NoArgs,
) (proto.Message, error) {
	userID, err := callerID(ctx)
	if err != nil {
		return nil, err
	}
	items, err := h.shopping.ListItemCategories(ctx, userID)
	if err != nil {
		return nil, err
	}
	pb := make([]*shoppinglistv1.ItemCategory, len(items))
	for i, ic := range items {
		pb[i] = &shoppinglistv1.ItemCategory{Name: ic.Name, CategoryId: ic.CategoryID}
	}
	return &shoppinglistv1.ListItemCategoriesResponse{Items: pb}, nil
}

func (h *tools) listIngredientUnits(
	ctx context.Context, _ mcptools.NoArgs,
) (proto.Message, error) {
	userID, err := callerID(ctx)
	if err != nil {
		return nil, err
	}
	units, err := h.shopping.ListIngredientUnits(ctx, userID)
	if err != nil {
		return nil, err
	}
	pb := make([]*shoppinglistv1.IngredientUnit, len(units))
	for i, u := range units {
		pb[i] = &shoppinglistv1.IngredientUnit{
			Name:      u.Name,
			Unit:      u.Unit,
			RecipeIds: u.RecipeIDs,
		}
	}
	return &shoppinglistv1.ListIngredientUnitsResponse{Units: pb}, nil
}
