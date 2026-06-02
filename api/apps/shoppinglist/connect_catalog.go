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
	_ *connect.Request[shoppinglistv1.ListCategoriesRequest],
) (*connect.Response[shoppinglistv1.ListCategoriesResponse], error) {
	user := getUser(ctx)
	if user == nil {
		return nil, errUnauthenticated()
	}

	categories, err := h.app.services.Shopping.ListCategories(ctx, user.ID)
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
	user := getUser(ctx)
	if user == nil {
		return nil, errUnauthenticated()
	}
	if req.Msg.Name == "" {
		return nil, errNameRequired()
	}

	c, err := h.app.services.Shopping.CreateCategory(ctx, user.ID, req.Msg.Name)
	if err != nil {
		return nil, mapError(err)
	}
	return connect.NewResponse(&shoppinglistv1.CreateCategoryResponse{
		Category: &shoppinglistv1.Category{Id: c.ID, Name: c.Name},
	}), nil
}

func (h *shoppingConnectHandler) RenameCategory(
	ctx context.Context,
	req *connect.Request[shoppinglistv1.RenameCategoryRequest],
) (*connect.Response[shoppinglistv1.RenameCategoryResponse], error) {
	user := getUser(ctx)
	if user == nil {
		return nil, errUnauthenticated()
	}
	if req.Msg.Name == "" {
		return nil, errNameRequired()
	}
	id, err := uuid.Parse(req.Msg.Id)
	if err != nil {
		return nil, errInvalidID()
	}

	c, err := h.app.services.Shopping.RenameCategory(ctx, user.ID, id, req.Msg.Name)
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
	user := getUser(ctx)
	if user == nil {
		return nil, errUnauthenticated()
	}
	id, err := uuid.Parse(req.Msg.Id)
	if err != nil {
		return nil, errInvalidID()
	}

	if err = h.app.services.Shopping.DeleteCategory(ctx, user.ID, id); err != nil {
		return nil, mapError(err)
	}
	return connect.NewResponse(&shoppinglistv1.DeleteCategoryResponse{}), nil
}

func (h *shoppingConnectHandler) ListItemNames(
	ctx context.Context,
	_ *connect.Request[shoppinglistv1.ListItemNamesRequest],
) (*connect.Response[shoppinglistv1.ListItemNamesResponse], error) {
	user := getUser(ctx)
	if user == nil {
		return nil, errUnauthenticated()
	}

	names, err := h.app.services.Shopping.ListItemNames(ctx, user.ID)
	if err != nil {
		return nil, mapError(err)
	}

	pb := make([]*shoppinglistv1.ItemName, len(names))
	for i, n := range names {
		pb[i] = &shoppinglistv1.ItemName{Name: n.Name, CategoryId: n.CategoryID}
	}
	return connect.NewResponse(&shoppinglistv1.ListItemNamesResponse{Names: pb}), nil
}

func (h *shoppingConnectHandler) ListItemCategories(
	ctx context.Context,
	_ *connect.Request[shoppinglistv1.ListItemCategoriesRequest],
) (*connect.Response[shoppinglistv1.ListItemCategoriesResponse], error) {
	user := getUser(ctx)
	if user == nil {
		return nil, errUnauthenticated()
	}

	items, err := h.app.services.Shopping.ListItemCategories(ctx, user.ID)
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
	user := getUser(ctx)
	if user == nil {
		return nil, errUnauthenticated()
	}
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

	err := h.app.services.Shopping.SetItemCategory(
		ctx, user.ID, req.Msg.Name, categoryID,
	)
	if err != nil {
		return nil, mapError(err)
	}
	return connect.NewResponse(&shoppinglistv1.SetItemCategoryResponse{}), nil
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
