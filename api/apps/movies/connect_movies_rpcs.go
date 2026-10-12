package movies

import (
	"context"

	"tools.xdoubleu.com/apps/movies/internal/models"
	moviesv1 "tools.xdoubleu.com/gen/movies/v1"
)

func (h *moviesConnectHandler) SearchTitles(
	ctx context.Context,
	req *moviesv1.SearchTitlesRequest,
) (*moviesv1.SearchTitlesResponse, error) {
	uid, err := userID(ctx)
	if err != nil {
		return nil, err
	}

	results, err := h.app.services.Movies.Search(ctx, uid, req.Query)
	if err != nil {
		return nil, mapError(err)
	}

	out := make([]*moviesv1.SearchResult, len(results))
	for i, r := range results {
		out[i] = protoSearchResult(r)
	}
	return &moviesv1.SearchTitlesResponse{Results: out}, nil
}

func (h *moviesConnectHandler) AddTitle(
	ctx context.Context,
	req *moviesv1.AddTitleRequest,
) (*moviesv1.AddTitleResponse, error) {
	uid, err := userID(ctx)
	if err != nil {
		return nil, err
	}

	entry, err := h.app.services.Movies.Add(
		ctx, uid, req.MediaType, req.TmdbId, req.Status,
		req.UnknownDate,
	)
	if err != nil {
		return nil, mapError(err)
	}
	return &moviesv1.AddTitleResponse{
		Entry: protoEntry(entry),
	}, nil
}

func (h *moviesConnectHandler) SetStatus(
	ctx context.Context,
	req *moviesv1.SetStatusRequest,
) (*moviesv1.SetStatusResponse, error) {
	uid, err := userID(ctx)
	if err != nil {
		return nil, err
	}
	id, err := parseID(req.Id)
	if err != nil {
		return nil, err
	}

	entry, err := h.app.services.Movies.SetStatus(
		ctx, uid, id, req.Status, req.UnknownDate,
	)
	if err != nil {
		return nil, mapError(err)
	}
	return &moviesv1.SetStatusResponse{
		Entry: protoEntry(entry),
	}, nil
}

func (h *moviesConnectHandler) SetRating(
	ctx context.Context,
	req *moviesv1.SetRatingRequest,
) (*moviesv1.SetRatingResponse, error) {
	uid, id, err := target(ctx, req.Id)
	if err != nil {
		return nil, err
	}

	entry, err := h.app.services.Movies.SetRating(
		ctx, uid, id, optionalInt(req.Rating),
	)
	if err != nil {
		return nil, mapError(err)
	}
	return &moviesv1.SetRatingResponse{
		Entry: protoEntry(entry),
	}, nil
}

func (h *moviesConnectHandler) RemoveTitle(
	ctx context.Context,
	req *moviesv1.RemoveTitleRequest,
) (*moviesv1.RemoveTitleResponse, error) {
	uid, err := userID(ctx)
	if err != nil {
		return nil, err
	}
	id, err := parseID(req.Id)
	if err != nil {
		return nil, err
	}

	if err = h.app.services.Movies.Remove(ctx, uid, id); err != nil {
		return nil, mapError(err)
	}
	return &moviesv1.RemoveTitleResponse{}, nil
}

func (h *moviesConnectHandler) ListBacklog(
	ctx context.Context,
	req *moviesv1.ListBacklogRequest,
) (*moviesv1.ListBacklogResponse, error) {
	uid, err := userID(ctx)
	if err != nil {
		return nil, err
	}

	entries, hasMore, err := h.app.services.Movies.List(ctx, uid, models.ListFilter{
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

	out := make([]*moviesv1.BacklogEntry, len(entries))
	for i := range entries {
		out[i] = protoEntry(&entries[i])
	}
	return &moviesv1.ListBacklogResponse{
		Entries: out,
		HasMore: hasMore,
	}, nil
}

func (h *moviesConnectHandler) GetTitle(
	ctx context.Context,
	req *moviesv1.GetTitleRequest,
) (*moviesv1.GetTitleResponse, error) {
	uid, err := userID(ctx)
	if err != nil {
		return nil, err
	}
	id, err := parseID(req.Id)
	if err != nil {
		return nil, err
	}

	entry, err := h.app.services.Movies.Get(ctx, uid, id)
	if err != nil {
		return nil, mapError(err)
	}
	return &moviesv1.GetTitleResponse{
		Entry:     protoEntry(entry),
		Overview:  entry.Title.Overview,
		Seasons:   protoSeasons(entry.Seasons),
		Offers:    protoOffers(entry.Providers),
		WatchLink: entry.Title.WatchLink,
	}, nil
}
