package movies

import (
	"context"

	"connectrpc.com/connect"

	"tools.xdoubleu.com/apps/movies/internal/models"
	moviesv1 "tools.xdoubleu.com/gen/movies/v1"
)

func (h *moviesConnectHandler) SearchTitles(
	ctx context.Context,
	req *connect.Request[moviesv1.SearchTitlesRequest],
) (*connect.Response[moviesv1.SearchTitlesResponse], error) {
	uid, err := userID(ctx)
	if err != nil {
		return nil, err
	}

	results, err := h.app.services.Movies.Search(ctx, uid, req.Msg.Query)
	if err != nil {
		return nil, mapError(err)
	}

	out := make([]*moviesv1.SearchResult, len(results))
	for i, r := range results {
		out[i] = protoSearchResult(r)
	}
	return connect.NewResponse(&moviesv1.SearchTitlesResponse{Results: out}), nil
}

func (h *moviesConnectHandler) AddTitle(
	ctx context.Context,
	req *connect.Request[moviesv1.AddTitleRequest],
) (*connect.Response[moviesv1.AddTitleResponse], error) {
	uid, err := userID(ctx)
	if err != nil {
		return nil, err
	}

	entry, err := h.app.services.Movies.Add(
		ctx, uid, req.Msg.MediaType, req.Msg.TmdbId, req.Msg.Status,
		req.Msg.UnknownDate,
	)
	if err != nil {
		return nil, mapError(err)
	}
	return connect.NewResponse(&moviesv1.AddTitleResponse{
		Entry: protoEntry(entry),
	}), nil
}

func (h *moviesConnectHandler) SetStatus(
	ctx context.Context,
	req *connect.Request[moviesv1.SetStatusRequest],
) (*connect.Response[moviesv1.SetStatusResponse], error) {
	uid, err := userID(ctx)
	if err != nil {
		return nil, err
	}
	id, err := parseID(req.Msg.Id)
	if err != nil {
		return nil, err
	}

	entry, err := h.app.services.Movies.SetStatus(
		ctx, uid, id, req.Msg.Status, req.Msg.UnknownDate,
	)
	if err != nil {
		return nil, mapError(err)
	}
	return connect.NewResponse(&moviesv1.SetStatusResponse{
		Entry: protoEntry(entry),
	}), nil
}

func (h *moviesConnectHandler) RemoveTitle(
	ctx context.Context,
	req *connect.Request[moviesv1.RemoveTitleRequest],
) (*connect.Response[moviesv1.RemoveTitleResponse], error) {
	uid, err := userID(ctx)
	if err != nil {
		return nil, err
	}
	id, err := parseID(req.Msg.Id)
	if err != nil {
		return nil, err
	}

	if err = h.app.services.Movies.Remove(ctx, uid, id); err != nil {
		return nil, mapError(err)
	}
	return connect.NewResponse(&moviesv1.RemoveTitleResponse{}), nil
}

func (h *moviesConnectHandler) ListBacklog(
	ctx context.Context,
	req *connect.Request[moviesv1.ListBacklogRequest],
) (*connect.Response[moviesv1.ListBacklogResponse], error) {
	uid, err := userID(ctx)
	if err != nil {
		return nil, err
	}

	entries, hasMore, err := h.app.services.Movies.List(ctx, uid, models.ListFilter{
		Status:    req.Msg.Status,
		MediaType: req.Msg.MediaType,
		Sort:      req.Msg.Sort,
		Limit:     req.Msg.Limit,
		Offset:    req.Msg.Offset,
	})
	if err != nil {
		return nil, mapError(err)
	}

	out := make([]*moviesv1.BacklogEntry, len(entries))
	for i := range entries {
		out[i] = protoEntry(&entries[i])
	}
	return connect.NewResponse(&moviesv1.ListBacklogResponse{
		Entries: out,
		HasMore: hasMore,
	}), nil
}

func (h *moviesConnectHandler) GetTitle(
	ctx context.Context,
	req *connect.Request[moviesv1.GetTitleRequest],
) (*connect.Response[moviesv1.GetTitleResponse], error) {
	uid, err := userID(ctx)
	if err != nil {
		return nil, err
	}
	id, err := parseID(req.Msg.Id)
	if err != nil {
		return nil, err
	}

	entry, err := h.app.services.Movies.Get(ctx, uid, id)
	if err != nil {
		return nil, mapError(err)
	}
	return connect.NewResponse(&moviesv1.GetTitleResponse{
		Entry:    protoEntry(entry),
		Overview: entry.Title.Overview,
		Seasons:  protoSeasons(entry.Seasons),
	}), nil
}
