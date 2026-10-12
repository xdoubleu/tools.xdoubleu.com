package movies

import (
	"context"

	moviesv1 "tools.xdoubleu.com/gen/movies/v1"
)

func (h *moviesConnectHandler) GetSettings(
	ctx context.Context,
	_ *moviesv1.GetSettingsRequest,
) (*moviesv1.GetSettingsResponse, error) {
	uid, err := userID(ctx)
	if err != nil {
		return nil, err
	}

	ids, err := h.app.services.Movies.Settings(ctx, uid)
	if err != nil {
		return nil, mapError(err)
	}
	return &moviesv1.GetSettingsResponse{ProviderIds: ids}, nil
}

func (h *moviesConnectHandler) SetSettings(
	ctx context.Context,
	req *moviesv1.SetSettingsRequest,
) (*moviesv1.SetSettingsResponse, error) {
	uid, err := userID(ctx)
	if err != nil {
		return nil, err
	}

	ids, err := h.app.services.Movies.SetSettings(ctx, uid, req.ProviderIds)
	if err != nil {
		return nil, mapError(err)
	}
	return &moviesv1.SetSettingsResponse{ProviderIds: ids}, nil
}

func (h *moviesConnectHandler) ListAvailableProviders(
	ctx context.Context,
	_ *moviesv1.ListAvailableProvidersRequest,
) (*moviesv1.ListAvailableProvidersResponse, error) {
	if _, err := userID(ctx); err != nil {
		return nil, err
	}

	list, err := h.app.services.Movies.AvailableProviders(ctx)
	if err != nil {
		return nil, mapError(err)
	}
	out := make([]*moviesv1.Provider, len(list))
	for i, p := range list {
		out[i] = &moviesv1.Provider{Id: p.ID, Name: p.Name, LogoPath: p.LogoPath}
	}
	return &moviesv1.ListAvailableProvidersResponse{
		Providers: out,
	}, nil
}
