package taste

import (
	"context"
	"time"

	"connectrpc.com/connect/v2"
	"github.com/google/uuid"

	"tools.xdoubleu.com/apps/taste/internal/models"
	tastev1 "tools.xdoubleu.com/gen/taste/v1"
)

func protoFavouriteShow(s *models.Show) *tastev1.FavouriteShow {
	return &tastev1.FavouriteShow{
		Id:         s.ID.String(),
		ItunesId:   s.ITunesID,
		Title:      s.Title,
		Author:     s.Author,
		ArtworkUrl: s.ArtworkURL,
		AppleUrl:   s.AppleURL,
		AddedAt:    s.AddedAt.Format(time.RFC3339),
	}
}

func (h *tasteConnectHandler) SearchShows(
	ctx context.Context,
	req *tastev1.SearchShowsRequest,
) (*tastev1.SearchShowsResponse, error) {
	uid, err := userID(ctx)
	if err != nil {
		return nil, err
	}

	results, err := h.app.shows.Search(ctx, uid, req.Query)
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

func (h *tasteConnectHandler) ListFavouriteShows(
	ctx context.Context,
	_ *tastev1.ListFavouriteShowsRequest,
) (*tastev1.ListFavouriteShowsResponse, error) {
	uid, err := userID(ctx)
	if err != nil {
		return nil, err
	}

	shows, err := h.app.shows.List(ctx, uid)
	if err != nil {
		return nil, mapError(err)
	}
	out := make([]*tastev1.FavouriteShow, len(shows))
	for i := range shows {
		out[i] = protoFavouriteShow(&shows[i])
	}
	return &tastev1.ListFavouriteShowsResponse{Favourites: out}, nil
}

func (h *tasteConnectHandler) AddFavouriteShow(
	ctx context.Context,
	req *tastev1.AddFavouriteShowRequest,
) (*tastev1.AddFavouriteShowResponse, error) {
	uid, err := userID(ctx)
	if err != nil {
		return nil, err
	}

	show, err := h.app.shows.Add(ctx, uid, req.ItunesId)
	if err != nil {
		return nil, mapError(err)
	}
	return &tastev1.AddFavouriteShowResponse{Favourite: protoFavouriteShow(show)}, nil
}

func (h *tasteConnectHandler) RemoveFavouriteShow(
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
	if err = h.app.shows.Remove(ctx, uid, id); err != nil {
		return nil, mapError(err)
	}
	return &tastev1.RemoveFavouriteShowResponse{}, nil
}
