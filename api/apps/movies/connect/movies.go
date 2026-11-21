package connect

import (
	"context"

	"tools.xdoubleu.com/apps/movies/internal/models"
	"tools.xdoubleu.com/apps/movies/internal/protomap"
	moviesv1 "tools.xdoubleu.com/gen/movies/v1"
)

func (h *Handler) SearchTitles(
	ctx context.Context,
	req *moviesv1.SearchTitlesRequest,
) (*moviesv1.SearchTitlesResponse, error) {
	uid, err := h.owner(ctx, req.Shared)
	if err != nil {
		return nil, err
	}

	results, err := h.movies.Search(ctx, uid, req.Query)
	if err != nil {
		return nil, mapError(err)
	}

	return &moviesv1.SearchTitlesResponse{Results: protomap.SearchResults(results)}, nil
}

func (h *Handler) AddTitle(
	ctx context.Context,
	req *moviesv1.AddTitleRequest,
) (*moviesv1.AddTitleResponse, error) {
	uid, err := h.owner(ctx, req.Shared)
	if err != nil {
		return nil, err
	}

	entry, err := h.movies.Add(
		ctx, uid, req.MediaType, req.TmdbId, req.Status,
		req.UnknownDate,
	)
	if err != nil {
		return nil, mapError(err)
	}
	return &moviesv1.AddTitleResponse{
		Entry: protomap.Entry(entry),
	}, nil
}

func (h *Handler) SetStatus(
	ctx context.Context,
	req *moviesv1.SetStatusRequest,
) (*moviesv1.SetStatusResponse, error) {
	uid, err := h.owner(ctx, req.Shared)
	if err != nil {
		return nil, err
	}
	id, err := parseID(req.Id)
	if err != nil {
		return nil, err
	}

	entry, err := h.movies.SetStatus(
		ctx, uid, id, req.Status, req.UnknownDate,
	)
	if err != nil {
		return nil, mapError(err)
	}
	return &moviesv1.SetStatusResponse{
		Entry: protomap.Entry(entry),
	}, nil
}

func (h *Handler) SetRating(
	ctx context.Context,
	req *moviesv1.SetRatingRequest,
) (*moviesv1.SetRatingResponse, error) {
	uid, id, err := h.target(ctx, req.Shared, req.Id)
	if err != nil {
		return nil, err
	}

	entry, err := h.movies.SetRating(
		ctx, uid, id, optionalInt(req.Rating),
	)
	if err != nil {
		return nil, mapError(err)
	}
	return &moviesv1.SetRatingResponse{
		Entry: protomap.Entry(entry),
	}, nil
}

func (h *Handler) RemoveTitle(
	ctx context.Context,
	req *moviesv1.RemoveTitleRequest,
) (*moviesv1.RemoveTitleResponse, error) {
	uid, err := h.owner(ctx, req.Shared)
	if err != nil {
		return nil, err
	}
	id, err := parseID(req.Id)
	if err != nil {
		return nil, err
	}

	if err = h.movies.Remove(ctx, uid, id); err != nil {
		return nil, mapError(err)
	}
	return &moviesv1.RemoveTitleResponse{}, nil
}

func (h *Handler) ListBacklog(
	ctx context.Context,
	req *moviesv1.ListBacklogRequest,
) (*moviesv1.ListBacklogResponse, error) {
	uid, err := h.owner(ctx, req.Shared)
	if err != nil {
		return nil, err
	}

	entries, hasMore, err := h.movies.List(ctx, uid, models.ListFilter{
		Status:       req.Status,
		MediaType:    req.MediaType,
		Sort:         req.Sort,
		NewSeason:    req.NewSeason,
		OnMyServices: req.OnMyServices,
		Limit:        req.Limit,
		Offset:       req.Offset,
	})
	if err != nil {
		return nil, mapError(err)
	}

	return &moviesv1.ListBacklogResponse{
		Entries: protomap.Entries(entries),
		HasMore: hasMore,
	}, nil
}

func (h *Handler) GetTitle(
	ctx context.Context,
	req *moviesv1.GetTitleRequest,
) (*moviesv1.GetTitleResponse, error) {
	uid, err := h.owner(ctx, req.Shared)
	if err != nil {
		return nil, err
	}
	id, err := parseID(req.Id)
	if err != nil {
		return nil, err
	}

	entry, err := h.movies.Get(ctx, uid, id)
	if err != nil {
		return nil, mapError(err)
	}
	return &moviesv1.GetTitleResponse{
		Entry:     protomap.Entry(entry),
		Overview:  entry.Title.Overview,
		Seasons:   protomap.Seasons(entry.Seasons),
		Offers:    protomap.Offers(entry.Providers),
		WatchLink: entry.Title.WatchLink,
	}, nil
}
