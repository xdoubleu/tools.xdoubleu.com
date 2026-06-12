package shoppinglist

import (
	"context"

	"connectrpc.com/connect"
	"github.com/google/uuid"

	shoppinglistv1 "tools.xdoubleu.com/gen/shoppinglist/v1"
)

func (h *shoppingConnectHandler) ListStores(
	ctx context.Context,
	req *connect.Request[shoppinglistv1.ListStoresRequest],
) (*connect.Response[shoppinglistv1.ListStoresResponse], error) {
	ownerID, err := h.resolveOwner(ctx, req.Msg.OwnerUserId, false)
	if err != nil {
		return nil, err
	}

	stores, err := h.app.services.Shopping.ListStores(ctx, ownerID)
	if err != nil {
		return nil, mapError(err)
	}

	pb := make([]*shoppinglistv1.Store, len(stores))
	for i, s := range stores {
		pb[i] = &shoppinglistv1.Store{Id: s.ID, Name: s.Name}
	}
	return connect.NewResponse(&shoppinglistv1.ListStoresResponse{Stores: pb}), nil
}

func (h *shoppingConnectHandler) CreateStore(
	ctx context.Context,
	req *connect.Request[shoppinglistv1.CreateStoreRequest],
) (*connect.Response[shoppinglistv1.CreateStoreResponse], error) {
	if req.Msg.Name == "" {
		return nil, errNameRequired()
	}
	ownerID, err := h.resolveOwner(ctx, req.Msg.OwnerUserId, true)
	if err != nil {
		return nil, err
	}

	s, err := h.app.services.Shopping.CreateStore(ctx, ownerID, req.Msg.Name)
	if err != nil {
		return nil, mapError(err)
	}
	return connect.NewResponse(&shoppinglistv1.CreateStoreResponse{
		Store: &shoppinglistv1.Store{Id: s.ID, Name: s.Name},
	}), nil
}

//nolint:dupl // parallel to RenameCategory but operates on a distinct entity
func (h *shoppingConnectHandler) RenameStore(
	ctx context.Context,
	req *connect.Request[shoppinglistv1.RenameStoreRequest],
) (*connect.Response[shoppinglistv1.RenameStoreResponse], error) {
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

	s, err := h.app.services.Shopping.RenameStore(ctx, ownerID, id, req.Msg.Name)
	if err != nil {
		return nil, mapError(err)
	}
	return connect.NewResponse(&shoppinglistv1.RenameStoreResponse{
		Store: &shoppinglistv1.Store{Id: s.ID, Name: s.Name},
	}), nil
}

func (h *shoppingConnectHandler) DeleteStore(
	ctx context.Context,
	req *connect.Request[shoppinglistv1.DeleteStoreRequest],
) (*connect.Response[shoppinglistv1.DeleteStoreResponse], error) {
	id, err := uuid.Parse(req.Msg.Id)
	if err != nil {
		return nil, errInvalidID()
	}
	ownerID, err := h.resolveOwner(ctx, req.Msg.OwnerUserId, true)
	if err != nil {
		return nil, err
	}

	if err = h.app.services.Shopping.DeleteStore(ctx, ownerID, id); err != nil {
		return nil, mapError(err)
	}
	return connect.NewResponse(&shoppinglistv1.DeleteStoreResponse{}), nil
}

func (h *shoppingConnectHandler) GetStoreCategories(
	ctx context.Context,
	req *connect.Request[shoppinglistv1.GetStoreCategoriesRequest],
) (*connect.Response[shoppinglistv1.GetStoreCategoriesResponse], error) {
	storeID, err := uuid.Parse(req.Msg.StoreId)
	if err != nil {
		return nil, errInvalidID()
	}
	ownerID, err := h.resolveOwner(ctx, req.Msg.OwnerUserId, false)
	if err != nil {
		return nil, err
	}

	categories, err := h.app.services.Shopping.GetStoreCategories(
		ctx, ownerID, storeID,
	)
	if err != nil {
		return nil, mapError(err)
	}

	pb := make([]*shoppinglistv1.Category, len(categories))
	for i, c := range categories {
		pb[i] = &shoppinglistv1.Category{Id: c.ID, Name: c.Name}
	}
	return connect.NewResponse(&shoppinglistv1.GetStoreCategoriesResponse{
		Categories: pb,
	}), nil
}

func (h *shoppingConnectHandler) SetStoreCategories(
	ctx context.Context,
	req *connect.Request[shoppinglistv1.SetStoreCategoriesRequest],
) (*connect.Response[shoppinglistv1.SetStoreCategoriesResponse], error) {
	storeID, err := uuid.Parse(req.Msg.StoreId)
	if err != nil {
		return nil, errInvalidID()
	}

	categoryIDs := make([]uuid.UUID, len(req.Msg.CategoryIds))
	for i, raw := range req.Msg.CategoryIds {
		parsed, parseErr := uuid.Parse(raw)
		if parseErr != nil {
			return nil, errInvalidID()
		}
		categoryIDs[i] = parsed
	}

	ownerID, err := h.resolveOwner(ctx, req.Msg.OwnerUserId, true)
	if err != nil {
		return nil, err
	}

	err = h.app.services.Shopping.SetStoreCategories(
		ctx, ownerID, storeID, categoryIDs,
	)
	if err != nil {
		return nil, mapError(err)
	}
	return connect.NewResponse(&shoppinglistv1.SetStoreCategoriesResponse{}), nil
}
