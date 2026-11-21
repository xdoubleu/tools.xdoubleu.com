package connect

import (
	"context"

	"github.com/google/uuid"

	shoppinglistv1 "tools.xdoubleu.com/gen/shoppinglist/v1"
	"tools.xdoubleu.com/internal/connecttools"
)

func (h *Handler) ListStores(
	ctx context.Context,
	_ *shoppinglistv1.ListStoresRequest,
) (*shoppinglistv1.ListStoresResponse, error) {
	userID, err := h.callerID(ctx)
	if err != nil {
		return nil, err
	}

	stores, err := h.shopping.ListStores(ctx, userID)
	if err != nil {
		return nil, mapError(err)
	}

	pb := make([]*shoppinglistv1.Store, len(stores))
	for i, s := range stores {
		pb[i] = &shoppinglistv1.Store{Id: s.ID, Name: s.Name}
	}
	return &shoppinglistv1.ListStoresResponse{Stores: pb}, nil
}

func (h *Handler) CreateStore(
	ctx context.Context,
	req *shoppinglistv1.CreateStoreRequest,
) (*shoppinglistv1.CreateStoreResponse, error) {
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

	s, err := h.shopping.CreateStore(ctx, userID, id, req.Name)
	if err != nil {
		return nil, mapError(err)
	}
	return &shoppinglistv1.CreateStoreResponse{
		Store: &shoppinglistv1.Store{Id: s.ID, Name: s.Name},
	}, nil
}

func (h *Handler) RenameStore(
	ctx context.Context,
	req *shoppinglistv1.RenameStoreRequest,
) (*shoppinglistv1.RenameStoreResponse, error) {
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

	s, err := h.shopping.RenameStore(ctx, userID, id, req.Name)
	if err != nil {
		return nil, mapError(err)
	}
	return &shoppinglistv1.RenameStoreResponse{
		Store: &shoppinglistv1.Store{Id: s.ID, Name: s.Name},
	}, nil
}

func (h *Handler) DeleteStore(
	ctx context.Context,
	req *shoppinglistv1.DeleteStoreRequest,
) (*shoppinglistv1.DeleteStoreResponse, error) {
	id, err := uuid.Parse(req.Id)
	if err != nil {
		return nil, errInvalidID()
	}
	userID, err := h.callerID(ctx)
	if err != nil {
		return nil, err
	}

	if err = h.shopping.DeleteStore(ctx, userID, id); err != nil {
		return nil, mapError(err)
	}
	return &shoppinglistv1.DeleteStoreResponse{}, nil
}

func (h *Handler) GetStoreCategories(
	ctx context.Context,
	req *shoppinglistv1.GetStoreCategoriesRequest,
) (*shoppinglistv1.GetStoreCategoriesResponse, error) {
	storeID, err := uuid.Parse(req.StoreId)
	if err != nil {
		return nil, errInvalidID()
	}
	userID, err := h.callerID(ctx)
	if err != nil {
		return nil, err
	}

	categories, err := h.shopping.GetStoreCategories(
		ctx, userID, storeID,
	)
	if err != nil {
		return nil, mapError(err)
	}

	pb := make([]*shoppinglistv1.Category, len(categories))
	for i, c := range categories {
		pb[i] = &shoppinglistv1.Category{Id: c.ID, Name: c.Name}
	}
	return &shoppinglistv1.GetStoreCategoriesResponse{
		Categories: pb,
	}, nil
}

func (h *Handler) SetStoreCategories(
	ctx context.Context,
	req *shoppinglistv1.SetStoreCategoriesRequest,
) (*shoppinglistv1.SetStoreCategoriesResponse, error) {
	storeID, err := uuid.Parse(req.StoreId)
	if err != nil {
		return nil, errInvalidID()
	}

	categoryIDs := make([]uuid.UUID, len(req.CategoryIds))
	for i, raw := range req.CategoryIds {
		parsed, parseErr := uuid.Parse(raw)
		if parseErr != nil {
			return nil, errInvalidID()
		}
		categoryIDs[i] = parsed
	}

	userID, err := h.callerID(ctx)
	if err != nil {
		return nil, err
	}

	err = h.shopping.SetStoreCategories(
		ctx, userID, storeID, categoryIDs,
	)
	if err != nil {
		return nil, mapError(err)
	}
	return &shoppinglistv1.SetStoreCategoriesResponse{}, nil
}
