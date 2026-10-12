package shoppinglist

import (
	"context"
	"strconv"
	"time"

	"connectrpc.com/connect/v2"
	"github.com/google/uuid"

	"tools.xdoubleu.com/apps/shoppinglist/internal/services"
	shoppinglistv1 "tools.xdoubleu.com/gen/shoppinglist/v1"
	"tools.xdoubleu.com/gen/shoppinglist/v1/shoppinglistv1connect"
	"tools.xdoubleu.com/internal/connecttools"
	"tools.xdoubleu.com/internal/constants"
	"tools.xdoubleu.com/internal/contexttools"
	"tools.xdoubleu.com/internal/format"
	sharedmodels "tools.xdoubleu.com/internal/models"
)

type shoppingConnectHandler struct {
	app *ShoppingList
}

var _ shoppinglistv1connect.ShoppingListServiceHandler = (*shoppingConnectHandler)(nil)

func getUser(ctx context.Context) *sharedmodels.User {
	return contexttools.GetValue[sharedmodels.User](ctx, constants.UserContextKey)
}

// callerID returns the authenticated caller's user ID.
func (h *shoppingConnectHandler) callerID(ctx context.Context) (string, error) {
	user := getUser(ctx)
	if user == nil {
		return "", errUnauthenticated()
	}
	return user.ID, nil
}

func mapError(err error) error {
	return connecttools.MapError(err)
}

func (h *shoppingConnectHandler) GetCustomList(
	ctx context.Context,
	_ *shoppinglistv1.GetCustomListRequest,
) (*shoppinglistv1.GetCustomListResponse, error) {
	userID, err := h.callerID(ctx)
	if err != nil {
		return nil, err
	}

	items, err := h.app.services.Shopping.GetCustomList(ctx, userID)
	if err != nil {
		return nil, mapError(err)
	}

	pb := make([]*shoppinglistv1.ShoppingItem, len(items))
	for i, item := range items {
		pb[i] = &shoppinglistv1.ShoppingItem{
			Id:     item.ID,
			Name:   item.Name,
			Amount: format.ToAmount(item.Amount),
			Unit:   item.Unit,
		}
	}

	return &shoppinglistv1.GetCustomListResponse{Items: pb}, nil
}

func (h *shoppingConnectHandler) CreateShoppingItem(
	ctx context.Context,
	req *shoppinglistv1.CreateShoppingItemRequest,
) (*shoppinglistv1.CreateShoppingItemResponse, error) {
	if req.Name == "" {
		return nil, connect.Errorf(
			connect.CodeInvalidArgument,
			"name is required",
		)
	}

	amount, err := strconv.ParseFloat(req.Amount, 64)
	if err != nil || amount < 0 {
		return nil, connect.Errorf(
			connect.CodeInvalidArgument,
			"amount must be a non-negative number",
		)
	}

	id, err := connecttools.ParseClientID(req.Id)
	if err != nil {
		return nil, err
	}

	userID, err := h.callerID(ctx)
	if err != nil {
		return nil, err
	}

	item, err := h.app.services.Shopping.AddItem(
		ctx, userID, id, req.Name, req.Unit, amount,
	)
	if err != nil {
		return nil, mapError(err)
	}

	return &shoppinglistv1.CreateShoppingItemResponse{
		Item: &shoppinglistv1.ShoppingItem{
			Id:     item.ID,
			Name:   item.Name,
			Amount: format.ToAmount(item.Amount),
			Unit:   item.Unit,
		},
	}, nil
}

func (h *shoppingConnectHandler) UpdateShoppingItem(
	ctx context.Context,
	req *shoppinglistv1.UpdateShoppingItemRequest,
) (*shoppinglistv1.UpdateShoppingItemResponse, error) {
	itemID, err := uuid.Parse(req.ItemId)
	if err != nil {
		return nil, connect.Errorf(
			connect.CodeInvalidArgument,
			"invalid item ID",
		)
	}

	if req.Name == "" {
		return nil, connect.Errorf(
			connect.CodeInvalidArgument,
			"name is required",
		)
	}

	amount, err := strconv.ParseFloat(req.Amount, 64)
	if err != nil || amount < 0 {
		return nil, connect.Errorf(
			connect.CodeInvalidArgument,
			"amount must be a non-negative number",
		)
	}

	userID, err := h.callerID(ctx)
	if err != nil {
		return nil, err
	}

	item, err := h.app.services.Shopping.UpdateItem(
		ctx, userID, itemID, req.Name, req.Unit, amount,
	)
	if err != nil {
		return nil, mapError(err)
	}

	return &shoppinglistv1.UpdateShoppingItemResponse{
		Item: &shoppinglistv1.ShoppingItem{
			Id:     item.ID,
			Name:   item.Name,
			Amount: format.ToAmount(item.Amount),
			Unit:   item.Unit,
		},
	}, nil
}

func (h *shoppingConnectHandler) DeleteShoppingItem(
	ctx context.Context,
	req *shoppinglistv1.DeleteShoppingItemRequest,
) (*shoppinglistv1.DeleteShoppingItemResponse, error) {
	itemID, err := uuid.Parse(req.ItemId)
	if err != nil {
		return nil, connect.Errorf(
			connect.CodeInvalidArgument,
			"invalid item ID",
		)
	}

	userID, err := h.callerID(ctx)
	if err != nil {
		return nil, err
	}

	if err = h.app.services.Shopping.DeleteItem(ctx, userID, itemID); err != nil {
		return nil, mapError(err)
	}

	return &shoppinglistv1.DeleteShoppingItemResponse{}, nil
}

func (h *shoppingConnectHandler) GetMealPlanExportItems(
	ctx context.Context,
	req *shoppinglistv1.GetMealPlanExportItemsRequest,
) (*shoppinglistv1.GetMealPlanExportItemsResponse, error) {
	user := getUser(ctx)
	if user == nil {
		return nil, connect.Errorf(
			connect.CodeUnauthenticated,
			"user not authenticated",
		)
	}

	planID, err := uuid.Parse(req.PlanId)
	if err != nil {
		return nil, connect.Errorf(
			connect.CodeInvalidArgument,
			"invalid plan ID",
		)
	}

	today, end, pastSlots := services.ExportWindow(time.Now().UTC())

	items, err := h.app.services.Shopping.GetMealPlanExportItems(
		ctx, planID, user.ID, today, end, pastSlots, req.ExcludedGroups,
	)
	if err != nil {
		return nil, mapError(err)
	}

	pb := make([]*shoppinglistv1.ShoppingItem, len(items))
	for i, item := range items {
		pb[i] = &shoppinglistv1.ShoppingItem{
			Name:       item.Name,
			Amount:     format.ToAmount(item.Amount),
			Unit:       item.Unit,
			RecipeName: item.RecipeName,
			GroupName:  item.GroupName,
		}
	}

	return &shoppinglistv1.GetMealPlanExportItemsResponse{
		Items: pb,
	}, nil
}

func (h *shoppingConnectHandler) GetPlanIngredientGroups(
	ctx context.Context,
	req *shoppinglistv1.GetPlanIngredientGroupsRequest,
) (*shoppinglistv1.GetPlanIngredientGroupsResponse, error) {
	user := getUser(ctx)
	if user == nil {
		return nil, connect.Errorf(
			connect.CodeUnauthenticated,
			"user not authenticated",
		)
	}

	planID, err := uuid.Parse(req.PlanId)
	if err != nil {
		return nil, connect.Errorf(
			connect.CodeInvalidArgument,
			"invalid plan ID",
		)
	}

	today, end, pastSlots := services.ExportWindow(time.Now().UTC())

	groups, err := h.app.services.Shopping.GetPlanIngredientGroups(
		ctx, planID, user.ID, today, end, pastSlots,
	)
	if err != nil {
		return nil, mapError(err)
	}

	pb := make([]*shoppinglistv1.PlanIngredientGroup, len(groups))
	for i, g := range groups {
		pb[i] = &shoppinglistv1.PlanIngredientGroup{
			RecipeName: g.RecipeName,
			GroupName:  g.GroupName,
		}
	}

	return &shoppinglistv1.GetPlanIngredientGroupsResponse{
		Groups: pb,
	}, nil
}
