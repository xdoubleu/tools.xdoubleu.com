package recipes

import (
	"context"
	"fmt"

	"connectrpc.com/connect"

	recipesv1 "tools.xdoubleu.com/gen/recipes/v1"
)

func (h *recipesConnectHandler) ShareRecipeBook(
	ctx context.Context,
	req *connect.Request[recipesv1.ShareRecipeBookRequest],
) (*connect.Response[recipesv1.ShareRecipeBookResponse], error) {
	user := getUser(ctx)
	if user == nil {
		return nil, connect.NewError(
			connect.CodeUnauthenticated,
			fmt.Errorf("user not authenticated"),
		)
	}

	if req.Msg.ContactUserId == "" {
		return nil, connect.NewError(
			connect.CodeInvalidArgument,
			fmt.Errorf("contact user ID is required"),
		)
	}

	err := h.app.services.Recipes.ShareBook(
		ctx, user.ID, req.Msg.ContactUserId, req.Msg.CanEdit,
	)
	if err != nil {
		return nil, mapError(err)
	}

	return connect.NewResponse(&recipesv1.ShareRecipeBookResponse{}), nil
}

func (h *recipesConnectHandler) UnshareRecipeBook(
	ctx context.Context,
	req *connect.Request[recipesv1.UnshareRecipeBookRequest],
) (*connect.Response[recipesv1.UnshareRecipeBookResponse], error) {
	user := getUser(ctx)
	if user == nil {
		return nil, connect.NewError(
			connect.CodeUnauthenticated,
			fmt.Errorf("user not authenticated"),
		)
	}

	if req.Msg.TargetUserId == "" {
		return nil, connect.NewError(
			connect.CodeInvalidArgument,
			fmt.Errorf("target user ID is required"),
		)
	}

	err := h.app.services.Recipes.UnshareBook(ctx, user.ID, req.Msg.TargetUserId)
	if err != nil {
		return nil, mapError(err)
	}

	return connect.NewResponse(&recipesv1.UnshareRecipeBookResponse{}), nil
}

func (h *recipesConnectHandler) ListRecipeBookShares(
	ctx context.Context,
	_ *connect.Request[recipesv1.ListRecipeBookSharesRequest],
) (*connect.Response[recipesv1.ListRecipeBookSharesResponse], error) {
	user := getUser(ctx)
	if user == nil {
		return nil, connect.NewError(
			connect.CodeUnauthenticated,
			fmt.Errorf("user not authenticated"),
		)
	}

	shares, err := h.app.services.Recipes.ListBookShares(ctx, user.ID)
	if err != nil {
		return nil, mapError(err)
	}

	pbShares := make([]*recipesv1.RecipeBookShare, len(shares))
	for i, s := range shares {
		pbShares[i] = &recipesv1.RecipeBookShare{
			UserId:      s.UserID,
			CanEdit:     s.CanEdit,
			DisplayName: s.DisplayName,
		}
	}

	return connect.NewResponse(&recipesv1.ListRecipeBookSharesResponse{
		Shares: pbShares,
	}), nil
}
