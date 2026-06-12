package shoppinglist

import (
	"context"
	"fmt"

	"connectrpc.com/connect"

	shoppinglistv1 "tools.xdoubleu.com/gen/shoppinglist/v1"
)

func (h *shoppingConnectHandler) ShareShoppingList(
	ctx context.Context,
	req *connect.Request[shoppinglistv1.ShareShoppingListRequest],
) (*connect.Response[shoppinglistv1.ShareShoppingListResponse], error) {
	user := getUser(ctx)
	if user == nil {
		return nil, errUnauthenticated()
	}
	if req.Msg.ContactUserId == "" {
		return nil, connect.NewError(
			connect.CodeInvalidArgument,
			fmt.Errorf("contact user ID is required"),
		)
	}

	err := h.app.services.Sharing.Share(
		ctx, user.ID, req.Msg.ContactUserId, req.Msg.CanEdit,
	)
	if err != nil {
		return nil, mapError(err)
	}

	return connect.NewResponse(&shoppinglistv1.ShareShoppingListResponse{}), nil
}

func (h *shoppingConnectHandler) UnshareShoppingList(
	ctx context.Context,
	req *connect.Request[shoppinglistv1.UnshareShoppingListRequest],
) (*connect.Response[shoppinglistv1.UnshareShoppingListResponse], error) {
	user := getUser(ctx)
	if user == nil {
		return nil, errUnauthenticated()
	}
	if req.Msg.TargetUserId == "" {
		return nil, connect.NewError(
			connect.CodeInvalidArgument,
			fmt.Errorf("target user ID is required"),
		)
	}

	if err := h.app.services.Sharing.Unshare(
		ctx, user.ID, req.Msg.TargetUserId,
	); err != nil {
		return nil, mapError(err)
	}

	return connect.NewResponse(&shoppinglistv1.UnshareShoppingListResponse{}), nil
}

func (h *shoppingConnectHandler) ListShoppingListShares(
	ctx context.Context,
	_ *connect.Request[shoppinglistv1.ListShoppingListSharesRequest],
) (*connect.Response[shoppinglistv1.ListShoppingListSharesResponse], error) {
	user := getUser(ctx)
	if user == nil {
		return nil, errUnauthenticated()
	}

	shares, err := h.app.services.Sharing.ListShares(ctx, user.ID)
	if err != nil {
		return nil, mapError(err)
	}

	pb := make([]*shoppinglistv1.ShoppingListShare, len(shares))
	for i, s := range shares {
		pb[i] = &shoppinglistv1.ShoppingListShare{
			UserId:      s.UserID,
			CanEdit:     s.CanEdit,
			DisplayName: s.DisplayName,
		}
	}

	return connect.NewResponse(&shoppinglistv1.ListShoppingListSharesResponse{
		Shares: pb,
	}), nil
}

func (h *shoppingConnectHandler) ListAccessibleLists(
	ctx context.Context,
	_ *connect.Request[shoppinglistv1.ListAccessibleListsRequest],
) (*connect.Response[shoppinglistv1.ListAccessibleListsResponse], error) {
	user := getUser(ctx)
	if user == nil {
		return nil, errUnauthenticated()
	}

	owners, err := h.app.services.Sharing.AccessibleOwners(ctx, user.ID)
	if err != nil {
		return nil, mapError(err)
	}

	pb := make([]*shoppinglistv1.ListOwner, len(owners))
	for i, o := range owners {
		pb[i] = &shoppinglistv1.ListOwner{
			UserId:      o.UserID,
			DisplayName: o.DisplayName,
			CanEdit:     o.CanEdit,
			IsSelf:      o.IsSelf,
		}
	}

	return connect.NewResponse(&shoppinglistv1.ListAccessibleListsResponse{
		Owners: pb,
	}), nil
}
