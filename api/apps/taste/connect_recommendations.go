package taste

import (
	"context"
	"errors"

	"connectrpc.com/connect"

	"tools.xdoubleu.com/apps/taste/internal/models"
	"tools.xdoubleu.com/apps/taste/internal/services"
	tastev1 "tools.xdoubleu.com/gen/taste/v1"
	"tools.xdoubleu.com/internal/connecttools"
	"tools.xdoubleu.com/internal/consumption"
)

func (h *tasteConnectHandler) ListRecommendations(
	ctx context.Context,
	req *connect.Request[tastev1.ListRecommendationsRequest],
) (*connect.Response[tastev1.ListRecommendationsResponse], error) {
	uid, err := userID(ctx)
	if err != nil {
		return nil, err
	}
	domain, track := req.Msg.Domain, req.Msg.Track
	if domain != "" && !services.IsDomain(consumption.Domain(domain)) {
		return nil, connect.NewError(
			connect.CodeInvalidArgument, errors.New("unknown domain"),
		)
	}
	if track != "" && !models.IsTrack(track) {
		return nil, connect.NewError(
			connect.CodeInvalidArgument, errors.New("track must be more or broaden"),
		)
	}
	recs, at, note, err := h.app.picks.List(ctx, uid, domain, track)
	if err != nil {
		return nil, connecttools.MapError(err)
	}
	out := make([]*tastev1.Recommendation, 0, len(recs))
	canAccess := canAccess(ctx)
	for _, r := range recs {
		if canAccess(services.AppOf(consumption.Domain(r.Domain))) {
			out = append(out, protoRecommendation(r))
		}
	}
	return connect.NewResponse(&tastev1.ListRecommendationsResponse{
		Recommendations: out,
		GeneratedAt:     formatTimePtr(at),
		LastError:       note,
	}), nil
}

func protoRecommendation(r models.Recommendation) *tastev1.Recommendation {
	sources := make([]*tastev1.Source, len(r.Sources))
	for i, s := range r.Sources {
		sources[i] = &tastev1.Source{Url: s.URL, Title: s.Title}
	}
	return &tastev1.Recommendation{
		Id:       r.ID,
		Domain:   r.Domain,
		Track:    r.Track,
		Title:    r.Title,
		Creator:  r.Creator,
		Year:     count32(r.Year),
		Url:      r.URL,
		ImageUrl: r.ImageURL,
		Why:      r.Why,
		Sources:  sources,
		Score:    count32(r.Score),
	}
}
