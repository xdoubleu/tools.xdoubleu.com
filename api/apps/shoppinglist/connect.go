package shoppinglist

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"connectrpc.com/connect"
	"github.com/google/uuid"
	"github.com/xdoubleu/essentia/v4/pkg/contexttools"
	"github.com/xdoubleu/essentia/v4/pkg/database"

	shoppinglistv1 "tools.xdoubleu.com/gen/shoppinglist/v1"
	"tools.xdoubleu.com/gen/shoppinglist/v1/shoppinglistv1connect"
	iapp "tools.xdoubleu.com/internal/app"
	"tools.xdoubleu.com/internal/constants"
	"tools.xdoubleu.com/internal/format"
	sharedmodels "tools.xdoubleu.com/internal/models"
)

const (
	daysPerWeek     = 7
	hoursPerDay     = 24
	breakfastCutoff = 12
	noonCutoff      = 17
	eveningCutoff   = 22

	slotBreakfast = "breakfast"
	slotNoon      = "noon"
	slotEvening   = "evening"
)

type shoppingConnectHandler struct {
	app *ShoppingList
}

var _ shoppinglistv1connect.ShoppingListServiceHandler = (*shoppingConnectHandler)(nil)

func getUser(ctx context.Context) *sharedmodels.User {
	return contexttools.GetValue[sharedmodels.User](ctx, constants.UserContextKey)
}

func mapError(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, database.ErrResourceNotFound) {
		return connect.NewError(connect.CodeNotFound, err)
	}
	var httpErr *iapp.HTTPError
	if errors.As(err, &httpErr) {
		switch httpErr.Status {
		case http.StatusBadRequest:
			return connect.NewError(connect.CodeInvalidArgument, err)
		case http.StatusNotFound:
			return connect.NewError(connect.CodeNotFound, err)
		case http.StatusForbidden:
			return connect.NewError(connect.CodePermissionDenied, err)
		default:
			return connect.NewError(connect.CodeInternal, err)
		}
	}
	return connect.NewError(connect.CodeInternal, err)
}

func (h *shoppingConnectHandler) GetCustomList(
	ctx context.Context,
	_ *connect.Request[shoppinglistv1.GetCustomListRequest],
) (*connect.Response[shoppinglistv1.GetCustomListResponse], error) {
	user := getUser(ctx)
	if user == nil {
		return nil, connect.NewError(
			connect.CodeUnauthenticated,
			fmt.Errorf("user not authenticated"),
		)
	}

	items, err := h.app.services.Shopping.GetCustomList(ctx, user.ID)
	if err != nil {
		return nil, mapError(err)
	}

	pb := make([]*shoppinglistv1.ShoppingItem, len(items))
	for i, item := range items {
		pb[i] = &shoppinglistv1.ShoppingItem{
			Id:     item.ID,
			Name:   item.Name,
			Amount: format.ToFractionCeiling(item.Amount),
			Unit:   item.Unit,
		}
	}

	return connect.NewResponse(&shoppinglistv1.GetCustomListResponse{Items: pb}), nil
}

func (h *shoppingConnectHandler) AddShoppingItem(
	ctx context.Context,
	req *connect.Request[shoppinglistv1.AddShoppingItemRequest],
) (*connect.Response[shoppinglistv1.AddShoppingItemResponse], error) {
	user := getUser(ctx)
	if user == nil {
		return nil, connect.NewError(
			connect.CodeUnauthenticated,
			fmt.Errorf("user not authenticated"),
		)
	}

	if req.Msg.Name == "" {
		return nil, connect.NewError(
			connect.CodeInvalidArgument,
			fmt.Errorf("name is required"),
		)
	}

	amount, err := strconv.ParseFloat(req.Msg.Amount, 64)
	if err != nil || amount < 0 {
		return nil, connect.NewError(
			connect.CodeInvalidArgument,
			fmt.Errorf("amount must be a non-negative number"),
		)
	}

	item, err := h.app.services.Shopping.AddItem(
		ctx, user.ID, req.Msg.Name, req.Msg.Unit, amount,
	)
	if err != nil {
		return nil, mapError(err)
	}

	return connect.NewResponse(&shoppinglistv1.AddShoppingItemResponse{
		Item: &shoppinglistv1.ShoppingItem{
			Id:     item.ID,
			Name:   item.Name,
			Amount: format.ToFractionCeiling(item.Amount),
			Unit:   item.Unit,
		},
	}), nil
}

func (h *shoppingConnectHandler) DeleteShoppingItem(
	ctx context.Context,
	req *connect.Request[shoppinglistv1.DeleteShoppingItemRequest],
) (*connect.Response[shoppinglistv1.DeleteShoppingItemResponse], error) {
	user := getUser(ctx)
	if user == nil {
		return nil, connect.NewError(
			connect.CodeUnauthenticated,
			fmt.Errorf("user not authenticated"),
		)
	}

	itemID, err := uuid.Parse(req.Msg.ItemId)
	if err != nil {
		return nil, connect.NewError(
			connect.CodeInvalidArgument,
			fmt.Errorf("invalid item ID"),
		)
	}

	if err = h.app.services.Shopping.DeleteItem(ctx, user.ID, itemID); err != nil {
		return nil, mapError(err)
	}

	return connect.NewResponse(&shoppinglistv1.DeleteShoppingItemResponse{}), nil
}

func (h *shoppingConnectHandler) GetMealPlanExportItems(
	ctx context.Context,
	req *connect.Request[shoppinglistv1.GetMealPlanExportItemsRequest],
) (*connect.Response[shoppinglistv1.GetMealPlanExportItemsResponse], error) {
	user := getUser(ctx)
	if user == nil {
		return nil, connect.NewError(
			connect.CodeUnauthenticated,
			fmt.Errorf("user not authenticated"),
		)
	}

	planID, err := uuid.Parse(req.Msg.PlanId)
	if err != nil {
		return nil, connect.NewError(
			connect.CodeInvalidArgument,
			fmt.Errorf("invalid plan ID"),
		)
	}

	now := time.Now().UTC()
	today := now.Truncate(hoursPerDay * time.Hour)

	pastSlots := []string{}
	switch {
	case now.Hour() >= eveningCutoff:
		today = today.AddDate(0, 0, 1)
	case now.Hour() >= noonCutoff:
		pastSlots = []string{slotBreakfast, slotNoon}
	case now.Hour() >= breakfastCutoff:
		pastSlots = []string{slotBreakfast}
	}

	end := today.AddDate(0, 0, daysPerWeek-1)

	items, err := h.app.services.Shopping.GetMealPlanExportItems(
		ctx, planID, user.ID, today, end, pastSlots,
	)
	if err != nil {
		return nil, mapError(err)
	}

	pb := make([]*shoppinglistv1.ShoppingItem, len(items))
	for i, item := range items {
		pb[i] = &shoppinglistv1.ShoppingItem{
			Name:   item.Name,
			Amount: format.ToFractionCeiling(item.Amount),
			Unit:   item.Unit,
		}
	}

	return connect.NewResponse(&shoppinglistv1.GetMealPlanExportItemsResponse{
		Items: pb,
	}), nil
}
