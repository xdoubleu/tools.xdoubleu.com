package shoppinglist

import (
	"context"

	"connectrpc.com/connect/v2"
	"github.com/google/uuid"

	shoppinglistv1 "tools.xdoubleu.com/gen/shoppinglist/v1"
	"tools.xdoubleu.com/internal/connecttools"
)

func (h *shoppingConnectHandler) ListCategories(
	ctx context.Context,
	_ *shoppinglistv1.ListCategoriesRequest,
) (*shoppinglistv1.ListCategoriesResponse, error) {
	userID, err := h.callerID(ctx)
	if err != nil {
		return nil, err
	}

	categories, err := h.app.services.Shopping.ListCategories(ctx, userID)
	if err != nil {
		return nil, mapError(err)
	}

	pb := make([]*shoppinglistv1.Category, len(categories))
	for i, c := range categories {
		pb[i] = &shoppinglistv1.Category{Id: c.ID, Name: c.Name}
	}
	return &shoppinglistv1.ListCategoriesResponse{
		Categories: pb,
	}, nil
}

func (h *shoppingConnectHandler) CreateCategory(
	ctx context.Context,
	req *shoppinglistv1.CreateCategoryRequest,
) (*shoppinglistv1.CreateCategoryResponse, error) {
	if req.Name == "" {
		return nil, errNameRequired()
	}
	id, err := connecttools.ParseClientID(req.Id)
	if err != nil {
		return nil, err
	}
	userID, err := h.callerID(ctx)
	if err != nil {
		return nil, err
	}

	c, err := h.app.services.Shopping.CreateCategory(ctx, userID, id, req.Name)
	if err != nil {
		return nil, mapError(err)
	}
	return &shoppinglistv1.CreateCategoryResponse{
		Category: &shoppinglistv1.Category{Id: c.ID, Name: c.Name},
	}, nil
}

func (h *shoppingConnectHandler) RenameCategory(
	ctx context.Context,
	req *shoppinglistv1.RenameCategoryRequest,
) (*shoppinglistv1.RenameCategoryResponse, error) {
	if req.Name == "" {
		return nil, errNameRequired()
	}
	id, err := uuid.Parse(req.Id)
	if err != nil {
		return nil, errInvalidID()
	}
	userID, err := h.callerID(ctx)
	if err != nil {
		return nil, err
	}

	c, err := h.app.services.Shopping.RenameCategory(ctx, userID, id, req.Name)
	if err != nil {
		return nil, mapError(err)
	}
	return &shoppinglistv1.RenameCategoryResponse{
		Category: &shoppinglistv1.Category{Id: c.ID, Name: c.Name},
	}, nil
}

func (h *shoppingConnectHandler) DeleteCategory(
	ctx context.Context,
	req *shoppinglistv1.DeleteCategoryRequest,
) (*shoppinglistv1.DeleteCategoryResponse, error) {
	id, err := uuid.Parse(req.Id)
	if err != nil {
		return nil, errInvalidID()
	}
	userID, err := h.callerID(ctx)
	if err != nil {
		return nil, err
	}

	if err = h.app.services.Shopping.DeleteCategory(ctx, userID, id); err != nil {
		return nil, mapError(err)
	}
	return &shoppinglistv1.DeleteCategoryResponse{}, nil
}

func (h *shoppingConnectHandler) ListItemNames(
	ctx context.Context,
	_ *shoppinglistv1.ListItemNamesRequest,
) (*shoppinglistv1.ListItemNamesResponse, error) {
	userID, err := h.callerID(ctx)
	if err != nil {
		return nil, err
	}

	names, err := h.app.services.Shopping.ListItemNames(ctx, userID)
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
	return &shoppinglistv1.ListItemNamesResponse{Names: pb}, nil
}

func (h *shoppingConnectHandler) ListItemCategories(
	ctx context.Context,
	_ *shoppinglistv1.ListItemCategoriesRequest,
) (*shoppinglistv1.ListItemCategoriesResponse, error) {
	userID, err := h.callerID(ctx)
	if err != nil {
		return nil, err
	}

	items, err := h.app.services.Shopping.ListItemCategories(ctx, userID)
	if err != nil {
		return nil, mapError(err)
	}

	pb := make([]*shoppinglistv1.ItemCategory, len(items))
	for i, ic := range items {
		pb[i] = &shoppinglistv1.ItemCategory{Name: ic.Name, CategoryId: ic.CategoryID}
	}
	return &shoppinglistv1.ListItemCategoriesResponse{
		Items: pb,
	}, nil
}

func (h *shoppingConnectHandler) SetItemCategory(
	ctx context.Context,
	req *shoppinglistv1.SetItemCategoryRequest,
) (*shoppinglistv1.SetItemCategoryResponse, error) {
	if req.Name == "" {
		return nil, errNameRequired()
	}

	// An empty category id clears the assignment.
	categoryID := uuid.Nil
	if req.CategoryId != "" {
		parsed, err := uuid.Parse(req.CategoryId)
		if err != nil {
			return nil, errInvalidID()
		}
		categoryID = parsed
	}

	userID, err := h.callerID(ctx)
	if err != nil {
		return nil, err
	}

	err = h.app.services.Shopping.SetItemCategory(
		ctx, userID, req.Name, categoryID,
	)
	if err != nil {
		return nil, mapError(err)
	}
	return &shoppinglistv1.SetItemCategoryResponse{}, nil
}

func (h *shoppingConnectHandler) SetItemExcluded(
	ctx context.Context,
	req *shoppinglistv1.SetItemExcludedRequest,
) (*shoppinglistv1.SetItemExcludedResponse, error) {
	if req.Name == "" {
		return nil, errNameRequired()
	}

	userID, err := h.callerID(ctx)
	if err != nil {
		return nil, err
	}

	err = h.app.services.Shopping.SetItemExcluded(
		ctx, userID, req.Name, req.Excluded,
	)
	if err != nil {
		return nil, mapError(err)
	}
	return &shoppinglistv1.SetItemExcludedResponse{}, nil
}

func errUnauthenticated() error {
	return connect.Errorf(
		connect.CodeUnauthenticated,
		"user not authenticated",
	)
}

func errNameRequired() error {
	return connect.Errorf(connect.CodeInvalidArgument, "name is required")
}

func errInvalidID() error {
	return connect.Errorf(connect.CodeInvalidArgument, "invalid ID")
}
