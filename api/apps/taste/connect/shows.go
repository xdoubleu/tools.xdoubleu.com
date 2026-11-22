package connect

import (
	"context"

	"connectrpc.com/connect/v2"
	"github.com/google/uuid"

	"tools.xdoubleu.com/apps/taste/internal/protomap"
	tastev1 "tools.xdoubleu.com/gen/taste/v1"
)

func (h *Handler) SearchShows(
	ctx context.Context,
	req *tastev1.SearchShowsRequest,
) (*tastev1.SearchShowsResponse, error) {
	uid, err := userID(ctx)
	if err != nil {
		return nil, err
	}

	results, err := h.svc.Shows.Search(ctx, uid, req.Query)
	if err != nil {
		return nil, mapError(err)
	}
	out := make([]*tastev1.SearchResult, len(results))
	for i, r := range results {
		out[i] = &tastev1.SearchResult{
			ItunesId:   r.ID,
			Title:      r.Title,
			Author:     r.Author,
			ArtworkUrl: r.ArtworkURL,
			Favourite:  r.Favourite,
		}
	}
	return &tastev1.SearchShowsResponse{Results: out}, nil
}

func (h *Handler) ListFavouriteShows(
	ctx context.Context,
	_ *tastev1.ListFavouriteShowsRequest,
) (*tastev1.ListFavouriteShowsResponse, error) {
	uid, err := userID(ctx)
	if err != nil {
		return nil, err
	}

	shows, err := h.svc.Shows.List(ctx, uid)
	if err != nil {
		return nil, mapError(err)
	}
	out := make([]*tastev1.FavouriteShow, len(shows))
	for i := range shows {
		out[i] = protomap.FavouriteShow(&shows[i])
	}
	return &tastev1.ListFavouriteShowsResponse{Favourites: out}, nil
}

func (h *Handler) AddFavouriteShow(
	ctx context.Context,
	req *tastev1.AddFavouriteShowRequest,
) (*tastev1.AddFavouriteShowResponse, error) {
	uid, err := userID(ctx)
	if err != nil {
		return nil, err
	}

	show, err := h.svc.Shows.Add(ctx, uid, req.ItunesId)
	if err != nil {
		return nil, mapError(err)
	}
	return &tastev1.AddFavouriteShowResponse{Favourite: protomap.FavouriteShow(show)}, nil
}

func (h *Handler) RemoveFavouriteShow(
	ctx context.Context,
	req *tastev1.RemoveFavouriteShowRequest,
) (*tastev1.RemoveFavouriteShowResponse, error) {
	uid, err := userID(ctx)
	if err != nil {
		return nil, err
	}

	id, err := uuid.Parse(req.Id)
	if err != nil {
		return nil, connect.NewError(
			connect.CodeInvalidArgument, "invalid id",
		)
	}
	if err = h.svc.Shows.Remove(ctx, uid, id); err != nil {
		return nil, mapError(err)
	}
	return &tastev1.RemoveFavouriteShowResponse{}, nil
}
