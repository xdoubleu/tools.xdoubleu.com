package podcasts

import (
	"context"
	"errors"
	"time"

	"connectrpc.com/connect"
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
			connect.CodeUnauthenticated, errors.New("user not authenticated"),
		)
	}
	return user.ID, nil
}

func mapError(err error) error {
	if errors.Is(err, services.ErrUpstream) {
		return connect.NewError(connect.CodeUnavailable, err)
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
		FetchError: s.FetchError,
	}
}

func (h *podcastsConnectHandler) SearchShows(
	ctx context.Context,
	req *connect.Request[podcastsv1.SearchShowsRequest],
) (*connect.Response[podcastsv1.SearchShowsResponse], error) {
	uid, err := userID(ctx)
	if err != nil {
		return nil, err
	}

	results, err := h.app.services.Shows.Search(ctx, uid, req.Msg.Query)
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
	return connect.NewResponse(&podcastsv1.SearchShowsResponse{Results: out}), nil
}

func (h *podcastsConnectHandler) ListFavourites(
	ctx context.Context,
	_ *connect.Request[podcastsv1.ListFavouritesRequest],
) (*connect.Response[podcastsv1.ListFavouritesResponse], error) {
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
	return connect.NewResponse(
		&podcastsv1.ListFavouritesResponse{Favourites: out},
	), nil
}

func (h *podcastsConnectHandler) AddFavourite(
	ctx context.Context,
	req *connect.Request[podcastsv1.AddFavouriteRequest],
) (*connect.Response[podcastsv1.AddFavouriteResponse], error) {
	uid, err := userID(ctx)
	if err != nil {
		return nil, err
	}

	show, err := h.app.services.Shows.Add(ctx, uid, req.Msg.ItunesId)
	if err != nil {
		return nil, mapError(err)
	}
	return connect.NewResponse(
		&podcastsv1.AddFavouriteResponse{Favourite: protoFavourite(show)},
	), nil
}

func (h *podcastsConnectHandler) RemoveFavourite(
	ctx context.Context,
	req *connect.Request[podcastsv1.RemoveFavouriteRequest],
) (*connect.Response[podcastsv1.RemoveFavouriteResponse], error) {
	uid, err := userID(ctx)
	if err != nil {
		return nil, err
	}

	id, err := uuid.Parse(req.Msg.Id)
	if err != nil {
		return nil, connect.NewError(
			connect.CodeInvalidArgument, errors.New("invalid id"),
		)
	}
	if err = h.app.services.Shows.Remove(ctx, uid, id); err != nil {
		return nil, mapError(err)
	}
	return connect.NewResponse(&podcastsv1.RemoveFavouriteResponse{}), nil
}

func protoEpisode(e *models.ListedEpisode) *podcastsv1.Episode {
	out := &podcastsv1.Episode{
		Id:         e.ID.String(),
		ShowId:     e.ShowID.String(),
		ShowTitle:  e.ShowTitle,
		ArtworkUrl: e.ArtworkURL,
		AppleUrl:   e.AppleURL,
		Title:      e.Title,
		Summary:    e.Summary,
		Link:       e.Link,
		AudioUrl:   e.AudioURL,
	}
	if e.DurationSeconds != nil {
		d := int32(*e.DurationSeconds) //nolint:gosec // an episode is far under 68 years
		out.DurationSeconds = &d
	}
	if e.PublishedAt != nil {
		out.PublishedAt = e.PublishedAt.Format(time.RFC3339)
	}
	return out
}

func (h *podcastsConnectHandler) ListEpisodes(
	ctx context.Context,
	req *connect.Request[podcastsv1.ListEpisodesRequest],
) (*connect.Response[podcastsv1.ListEpisodesResponse], error) {
	uid, err := userID(ctx)
	if err != nil {
		return nil, err
	}

	episodes, more, err := h.app.services.Episodes.List(
		ctx, uid, req.Msg.ShowId, req.Msg.Limit, req.Msg.Offset,
	)
	if err != nil {
		return nil, mapError(err)
	}
	out := make([]*podcastsv1.Episode, len(episodes))
	for i := range episodes {
		out[i] = protoEpisode(&episodes[i])
	}
	return connect.NewResponse(
		&podcastsv1.ListEpisodesResponse{Episodes: out, HasMore: more},
	), nil
}
