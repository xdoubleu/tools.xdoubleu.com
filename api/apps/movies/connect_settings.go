package movies

import (
	"context"

	"connectrpc.com/connect"

	moviesv1 "tools.xdoubleu.com/gen/movies/v1"
)

func (h *moviesConnectHandler) GetSettings(
	ctx context.Context,
	_ *connect.Request[moviesv1.GetSettingsRequest],
) (*connect.Response[moviesv1.GetSettingsResponse], error) {
	uid, err := userID(ctx)
	if err != nil {
		return nil, err
	}

	ids, err := h.app.services.Movies.Settings(ctx, uid)
	if err != nil {
		return nil, mapError(err)
	}
	return connect.NewResponse(&moviesv1.GetSettingsResponse{ProviderIds: ids}), nil
}

func (h *moviesConnectHandler) SetSettings(
	ctx context.Context,
	req *connect.Request[moviesv1.SetSettingsRequest],
) (*connect.Response[moviesv1.SetSettingsResponse], error) {
	uid, err := userID(ctx)
	if err != nil {
		return nil, err
	}

	ids, err := h.app.services.Movies.SetSettings(ctx, uid, req.Msg.ProviderIds)
	if err != nil {
		return nil, mapError(err)
	}
	return connect.NewResponse(&moviesv1.SetSettingsResponse{ProviderIds: ids}), nil
}

func (h *moviesConnectHandler) ListAvailableProviders(
	ctx context.Context,
	_ *connect.Request[moviesv1.ListAvailableProvidersRequest],
) (*connect.Response[moviesv1.ListAvailableProvidersResponse], error) {
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
	return connect.NewResponse(&moviesv1.ListAvailableProvidersResponse{
		Providers: out,
	}), nil
}
