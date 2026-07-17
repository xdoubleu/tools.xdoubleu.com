package shoppinglist

import (
	"context"

	"connectrpc.com/connect"
	"github.com/google/uuid"

	shoppinglistv1 "tools.xdoubleu.com/gen/shoppinglist/v1"
)

// callerID returns the authenticated caller's own user ID. Stores are private:
// the store RPCs never resolve a shared owner, so a share recipient can only
// ever touch their own stores.
func (h *shoppingConnectHandler) callerID(ctx context.Context) (string, error) {
	user := getUser(ctx)
	if user == nil {
		return "", errUnauthenticated()
	}
	return user.ID, nil
}

func (h *shoppingConnectHandler) ListStores(
	ctx context.Context,
	_ *connect.Request[shoppinglistv1.ListStoresRequest],
) (*connect.Response[shoppinglistv1.ListStoresResponse], error) {
	userID, err := h.callerID(ctx)
	if err != nil {
		return nil, err
	}

	stores, err := h.app.services.Shopping.ListStores(ctx, userID)
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
	userID, err := h.callerID(ctx)
	if err != nil {
		return nil, err
	}

	s, err := h.app.services.Shopping.CreateStore(ctx, userID, req.Msg.Name)
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
	if req.Msg.Name == "" {
		return nil, errNameRequired()
	}
	id, err := uuid.Parse(req.Msg.Id)
	if err != nil {
		return nil, errInvalidID()
	}
	userID, err := h.callerID(ctx)
	if err != nil {
		return nil, err
	}

	s, err := h.app.services.Shopping.RenameStore(ctx, userID, id, req.Msg.Name)
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
	userID, err := h.callerID(ctx)
	if err != nil {
		return nil, err
	}

	if err = h.app.services.Shopping.DeleteStore(ctx, userID, id); err != nil {
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
	userID, err := h.callerID(ctx)
	if err != nil {
		return nil, err
	}

	categories, err := h.app.services.Shopping.GetStoreCategories(
		ctx, userID, storeID,
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

	userID, err := h.callerID(ctx)
	if err != nil {
		return nil, err
	}

	err = h.app.services.Shopping.SetStoreCategories(
		ctx, userID, storeID, categoryIDs,
	)
	if err != nil {
		return nil, mapError(err)
	}
	return connect.NewResponse(&shoppinglistv1.SetStoreCategoriesResponse{}), nil
}
