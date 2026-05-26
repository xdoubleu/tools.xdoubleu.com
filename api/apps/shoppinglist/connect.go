package shoppinglist

import (
	"context"
	"errors"
	"fmt"
	"net/http"
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
	daysPerWeek = 7
	hoursPerDay = 24
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

func (h *shoppingConnectHandler) GetShoppingList(
	ctx context.Context,
	req *connect.Request[shoppinglistv1.GetShoppingListRequest],
) (*connect.Response[shoppinglistv1.GetShoppingListResponse], error) {
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

	today := time.Now().UTC().Truncate(hoursPerDay * time.Hour)
	end := today.AddDate(0, 0, daysPerWeek-1)

	items, err := h.app.services.Shopping.GetList(ctx, planID, user.ID, today, end)
	if err != nil {
		return nil, mapError(err)
	}

	pbItems := make([]*shoppinglistv1.ShoppingItem, len(items))
	for i, item := range items {
		pbItems[i] = &shoppinglistv1.ShoppingItem{
			Name:   item.Name,
			Amount: format.ToFractionCeiling(item.Amount),
			Unit:   item.Unit,
		}
	}

	return connect.NewResponse(&shoppinglistv1.GetShoppingListResponse{
		Items: pbItems,
	}), nil
}
