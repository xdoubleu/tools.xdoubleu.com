package connect

import (
	"context"

	"tools.xdoubleu.com/apps/movies/internal/protomap"
	moviesv1 "tools.xdoubleu.com/gen/movies/v1"
)

func (h *Handler) GetSettings(
	ctx context.Context,
	_ *moviesv1.GetSettingsRequest,
) (*moviesv1.GetSettingsResponse, error) {
	uid, err := userID(ctx)
	if err != nil {
		return nil, err
	}

	ids, err := h.movies.Settings(ctx, uid)
	if err != nil {
		return nil, mapError(err)
	}
	return &moviesv1.GetSettingsResponse{ProviderIds: ids}, nil
}

func (h *Handler) SetSettings(
	ctx context.Context,
	req *moviesv1.SetSettingsRequest,
) (*moviesv1.SetSettingsResponse, error) {
	uid, err := userID(ctx)
	if err != nil {
		return nil, err
	}

	ids, err := h.movies.SetSettings(ctx, uid, req.ProviderIds)
	if err != nil {
		return nil, mapError(err)
	}
	return &moviesv1.SetSettingsResponse{ProviderIds: ids}, nil
}

func (h *Handler) ListAvailableProviders(
	ctx context.Context,
	_ *moviesv1.ListAvailableProvidersRequest,
) (*moviesv1.ListAvailableProvidersResponse, error) {
	if _, err := userID(ctx); err != nil {
		return nil, err
	}

	list, err := h.movies.AvailableProviders(ctx)
	if err != nil {
		return nil, mapError(err)
	}
	return &moviesv1.ListAvailableProvidersResponse{
		Providers: protomap.Providers(list),
	}, nil
}
