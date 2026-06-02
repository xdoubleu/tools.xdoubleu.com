package shoppinglist

import (
	"context"

	"connectrpc.com/connect"
	"github.com/google/uuid"

	shoppinglistv1 "tools.xdoubleu.com/gen/shoppinglist/v1"
)

func (h *shoppingConnectHandler) ListStores(
	ctx context.Context,
	_ *connect.Request[shoppinglistv1.ListStoresRequest],
) (*connect.Response[shoppinglistv1.ListStoresResponse], error) {
	user := getUser(ctx)
	if user == nil {
		return nil, errUnauthenticated()
	}

	stores, err := h.app.services.Shopping.ListStores(ctx, user.ID)
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
	user := getUser(ctx)
	if user == nil {
		return nil, errUnauthenticated()
	}
	if req.Msg.Name == "" {
		return nil, errNameRequired()
	}

	s, err := h.app.services.Shopping.CreateStore(ctx, user.ID, req.Msg.Name)
	if err != nil {
		return nil, mapError(err)
	}
	return connect.NewResponse(&shoppinglistv1.CreateStoreResponse{
		Store: &shoppinglistv1.Store{Id: s.ID, Name: s.Name},
	}), nil
}

func (h *shoppingConnectHandler) RenameStore(
	ctx context.Context,
	req *connect.Request[shoppinglistv1.RenameStoreRequest],
) (*connect.Response[shoppinglistv1.RenameStoreResponse], error) {
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

	s, err := h.app.services.Shopping.RenameStore(ctx, user.ID, id, req.Msg.Name)
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
	user := getUser(ctx)
	if user == nil {
		return nil, errUnauthenticated()
	}
	id, err := uuid.Parse(req.Msg.Id)
	if err != nil {
		return nil, errInvalidID()
	}

	if err = h.app.services.Shopping.DeleteStore(ctx, user.ID, id); err != nil {
		return nil, mapError(err)
	}
	return connect.NewResponse(&shoppinglistv1.DeleteStoreResponse{}), nil
}

func (h *shoppingConnectHandler) GetStoreCategories(
	ctx context.Context,
	req *connect.Request[shoppinglistv1.GetStoreCategoriesRequest],
) (*connect.Response[shoppinglistv1.GetStoreCategoriesResponse], error) {
	user := getUser(ctx)
	if user == nil {
		return nil, errUnauthenticated()
	}
	storeID, err := uuid.Parse(req.Msg.StoreId)
	if err != nil {
		return nil, errInvalidID()
	}

	categories, err := h.app.services.Shopping.GetStoreCategories(
		ctx, user.ID, storeID,
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
	user := getUser(ctx)
	if user == nil {
		return nil, errUnauthenticated()
	}
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

	err = h.app.services.Shopping.SetStoreCategories(
		ctx, user.ID, storeID, categoryIDs,
	)
	if err != nil {
		return nil, mapError(err)
	}
	return connect.NewResponse(&shoppinglistv1.SetStoreCategoriesResponse{}), nil
}
