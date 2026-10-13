package podcasts

import (
	"context"
	"errors"
	"time"

	"connectrpc.com/connect/v2"
	"github.com/google/uuid"

	"tools.xdoubleu.com/apps/podcasts/internal/models"
	"tools.xdoubleu.com/apps/podcasts/internal/services"
	podcastsv1 "tools.xdoubleu.com/gen/podcasts/v1"
	"tools.xdoubleu.com/gen/podcasts/v1/podcastsv1connect"
	"tools.xdoubleu.com/internal/connecttools"
	"tools.xdoubleu.com/internal/constants"
	"tools.xdoubleu.com/internal/contexttools"
	sharedmodels "tools.xdoubleu.com/internal/models"
)

type podcastsConnectHandler struct {
	app *Podcasts
}

var _ podcastsv1connect.PodcastsServiceHandler = (*podcastsConnectHandler)(nil)

func userID(ctx context.Context) (string, error) {
	user := contexttools.GetValue[sharedmodels.User](ctx, constants.UserContextKey)
	if user == nil {
		return "", connect.NewError(
			connect.CodeUnauthenticated, "user not authenticated",
		)
	}
	return user.ID, nil
}

func mapError(err error) error {
	if errors.Is(err, services.ErrUpstream) {
		return connect.NewError(connect.CodeUnavailable, err.Error()).WithCause(err)
	}
	return connecttools.MapError(err)
}

func protoFavourite(s *models.Show) *podcastsv1.Favourite {
	return &podcastsv1.Favourite{
		Id:         s.ID.String(),
		ItunesId:   s.ITunesID,
		Title:      s.Title,
		Author:     s.Author,
		ArtworkUrl: s.ArtworkURL,
		AppleUrl:   s.AppleURL,
		AddedAt:    s.AddedAt.Format(time.RFC3339),
	}
}

func (h *podcastsConnectHandler) SearchShows(
	ctx context.Context,
	req *podcastsv1.SearchShowsRequest,
) (*podcastsv1.SearchShowsResponse, error) {
	uid, err := userID(ctx)
	if err != nil {
		return nil, err
	}

	results, err := h.app.services.Shows.Search(ctx, uid, req.Query)
	if err != nil {
		return nil, mapError(err)
	}
	out := make([]*podcastsv1.SearchResult, len(results))
	for i, r := range results {
		out[i] = &podcastsv1.SearchResult{
			ItunesId:   r.ID,
			Title:      r.Title,
			Author:     r.Author,
			ArtworkUrl: r.ArtworkURL,
			Favourite:  r.Favourite,
		}
	}
	return &podcastsv1.SearchShowsResponse{Results: out}, nil
}

func (h *podcastsConnectHandler) ListFavourites(
	ctx context.Context,
	_ *podcastsv1.ListFavouritesRequest,
) (*podcastsv1.ListFavouritesResponse, error) {
	uid, err := userID(ctx)
	if err != nil {
		return nil, err
	}

	shows, err := h.app.services.Shows.List(ctx, uid)
	if err != nil {
		return nil, mapError(err)
	}
	out := make([]*podcastsv1.Favourite, len(shows))
	for i := range shows {
		out[i] = protoFavourite(&shows[i])
	}
	return &podcastsv1.ListFavouritesResponse{Favourites: out}, nil
}

func (h *podcastsConnectHandler) AddFavourite(
	ctx context.Context,
	req *podcastsv1.AddFavouriteRequest,
) (*podcastsv1.AddFavouriteResponse, error) {
	uid, err := userID(ctx)
	if err != nil {
		return nil, err
	}

	show, err := h.app.services.Shows.Add(ctx, uid, req.ItunesId)
	if err != nil {
		return nil, mapError(err)
	}
	return &podcastsv1.AddFavouriteResponse{Favourite: protoFavourite(show)}, nil
}

func (h *podcastsConnectHandler) RemoveFavourite(
	ctx context.Context,
	req *podcastsv1.RemoveFavouriteRequest,
) (*podcastsv1.RemoveFavouriteResponse, error) {
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
	if err = h.app.services.Shows.Remove(ctx, uid, id); err != nil {
		return nil, mapError(err)
	}
	return &podcastsv1.RemoveFavouriteResponse{}, nil
}
