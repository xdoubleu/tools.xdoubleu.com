package shoppinglist

import (
	"context"
	"fmt"

	"connectrpc.com/connect"
	"github.com/google/uuid"

	shoppinglistv1 "tools.xdoubleu.com/gen/shoppinglist/v1"
)

func (h *shoppingConnectHandler) ListCategories(
	ctx context.Context,
	req *connect.Request[shoppinglistv1.ListCategoriesRequest],
) (*connect.Response[shoppinglistv1.ListCategoriesResponse], error) {
	ownerID, err := h.resolveOwner(ctx, req.Msg.OwnerUserId, false)
	if err != nil {
		return nil, err
	}

	categories, err := h.app.services.Shopping.ListCategories(ctx, ownerID)
	if err != nil {
		return nil, mapError(err)
	}

	pb := make([]*shoppinglistv1.Category, len(categories))
	for i, c := range categories {
		pb[i] = &shoppinglistv1.Category{Id: c.ID, Name: c.Name}
	}
	return connect.NewResponse(&shoppinglistv1.ListCategoriesResponse{
		Categories: pb,
	}), nil
}

func (h *shoppingConnectHandler) CreateCategory(
	ctx context.Context,
	req *connect.Request[shoppinglistv1.CreateCategoryRequest],
) (*connect.Response[shoppinglistv1.CreateCategoryResponse], error) {
	if req.Msg.Name == "" {
		return nil, errNameRequired()
	}
	ownerID, err := h.resolveOwner(ctx, req.Msg.OwnerUserId, true)
	if err != nil {
		return nil, err
	}

	c, err := h.app.services.Shopping.CreateCategory(ctx, ownerID, req.Msg.Name)
	if err != nil {
		return nil, mapError(err)
	}
	return connect.NewResponse(&shoppinglistv1.CreateCategoryResponse{
		Category: &shoppinglistv1.Category{Id: c.ID, Name: c.Name},
	}), nil
}

//nolint:dupl // parallel to RenameStore but operates on a distinct entity
func (h *shoppingConnectHandler) RenameCategory(
	ctx context.Context,
	req *connect.Request[shoppinglistv1.RenameCategoryRequest],
) (*connect.Response[shoppinglistv1.RenameCategoryResponse], error) {
	if req.Msg.Name == "" {
		return nil, errNameRequired()
	}
	id, err := uuid.Parse(req.Msg.Id)
	if err != nil {
		return nil, errInvalidID()
	}
	ownerID, err := h.resolveOwner(ctx, req.Msg.OwnerUserId, true)
	if err != nil {
		return nil, err
	}

	c, err := h.app.services.Shopping.RenameCategory(ctx, ownerID, id, req.Msg.Name)
	if err != nil {
		return nil, mapError(err)
	}
	return connect.NewResponse(&shoppinglistv1.RenameCategoryResponse{
		Category: &shoppinglistv1.Category{Id: c.ID, Name: c.Name},
	}), nil
}

func (h *shoppingConnectHandler) DeleteCategory(
	ctx context.Context,
	req *connect.Request[shoppinglistv1.DeleteCategoryRequest],
) (*connect.Response[shoppinglistv1.DeleteCategoryResponse], error) {
	id, err := uuid.Parse(req.Msg.Id)
	if err != nil {
		return nil, errInvalidID()
	}
	ownerID, err := h.resolveOwner(ctx, req.Msg.OwnerUserId, true)
	if err != nil {
		return nil, err
	}

	if err = h.app.services.Shopping.DeleteCategory(ctx, ownerID, id); err != nil {
		return nil, mapError(err)
	}
	return connect.NewResponse(&shoppinglistv1.DeleteCategoryResponse{}), nil
}

func (h *shoppingConnectHandler) ListItemNames(
	ctx context.Context,
	req *connect.Request[shoppinglistv1.ListItemNamesRequest],
) (*connect.Response[shoppinglistv1.ListItemNamesResponse], error) {
	ownerID, err := h.resolveOwner(ctx, req.Msg.OwnerUserId, false)
	if err != nil {
		return nil, err
	}

	names, err := h.app.services.Shopping.ListItemNames(ctx, ownerID)
	if err != nil {
		return nil, mapError(err)
	}

	pb := make([]*shoppinglistv1.ItemName, len(names))
	for i, n := range names {
		pb[i] = &shoppinglistv1.ItemName{
			Name:       n.Name,
			CategoryId: n.CategoryID,
			Excluded:   n.Excluded,
		}
	}
	return connect.NewResponse(&shoppinglistv1.ListItemNamesResponse{Names: pb}), nil
}

func (h *shoppingConnectHandler) ListItemCategories(
	ctx context.Context,
	req *connect.Request[shoppinglistv1.ListItemCategoriesRequest],
) (*connect.Response[shoppinglistv1.ListItemCategoriesResponse], error) {
	ownerID, err := h.resolveOwner(ctx, req.Msg.OwnerUserId, false)
	if err != nil {
		return nil, err
	}

	items, err := h.app.services.Shopping.ListItemCategories(ctx, ownerID)
	if err != nil {
		return nil, mapError(err)
	}

	pb := make([]*shoppinglistv1.ItemCategory, len(items))
	for i, ic := range items {
		pb[i] = &shoppinglistv1.ItemCategory{Name: ic.Name, CategoryId: ic.CategoryID}
	}
	return connect.NewResponse(&shoppinglistv1.ListItemCategoriesResponse{
		Items: pb,
	}), nil
}

func (h *shoppingConnectHandler) SetItemCategory(
	ctx context.Context,
	req *connect.Request[shoppinglistv1.SetItemCategoryRequest],
) (*connect.Response[shoppinglistv1.SetItemCategoryResponse], error) {
	if req.Msg.Name == "" {
		return nil, errNameRequired()
	}

	// An empty category id clears the assignment.
	categoryID := uuid.Nil
	if req.Msg.CategoryId != "" {
		parsed, err := uuid.Parse(req.Msg.CategoryId)
		if err != nil {
			return nil, errInvalidID()
		}
		categoryID = parsed
	}

	ownerID, err := h.resolveOwner(ctx, req.Msg.OwnerUserId, true)
	if err != nil {
		return nil, err
	}

	err = h.app.services.Shopping.SetItemCategory(
		ctx, ownerID, req.Msg.Name, categoryID,
	)
	if err != nil {
		return nil, mapError(err)
	}
	return connect.NewResponse(&shoppinglistv1.SetItemCategoryResponse{}), nil
}

func (h *shoppingConnectHandler) SetItemExcluded(
	ctx context.Context,
	req *connect.Request[shoppinglistv1.SetItemExcludedRequest],
) (*connect.Response[shoppinglistv1.SetItemExcludedResponse], error) {
	if req.Msg.Name == "" {
		return nil, errNameRequired()
	}

	ownerID, err := h.resolveOwner(ctx, req.Msg.OwnerUserId, true)
	if err != nil {
		return nil, err
	}

	err = h.app.services.Shopping.SetItemExcluded(
		ctx, ownerID, req.Msg.Name, req.Msg.Excluded,
	)
	if err != nil {
		return nil, mapError(err)
	}
	return connect.NewResponse(&shoppinglistv1.SetItemExcludedResponse{}), nil
}

func errUnauthenticated() error {
	return connect.NewError(
		connect.CodeUnauthenticated,
		fmt.Errorf("user not authenticated"),
	)
}

func errNameRequired() error {
	return connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("name is required"))
}

func errInvalidID() error {
	return connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("invalid ID"))
}
