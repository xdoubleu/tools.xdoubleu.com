package taste

import (
	"context"
	"time"

	"connectrpc.com/connect/v2"
	"github.com/google/uuid"

	"tools.xdoubleu.com/apps/taste/internal/jobs"
	"tools.xdoubleu.com/apps/taste/internal/models"
	"tools.xdoubleu.com/apps/taste/internal/services"
	tastev1 "tools.xdoubleu.com/gen/taste/v1"
	"tools.xdoubleu.com/internal/consumption"
)

func (h *tasteConnectHandler) ListRecommendations(
	ctx context.Context,
	req *tastev1.ListRecommendationsRequest,
) (*tastev1.ListRecommendationsResponse, error) {
	sc, err := h.scopeOf(ctx, req.Shared)
	if err != nil {
		return nil, err
	}
	domain, track := req.Domain, req.Track
	if domain != "" && !services.IsDomain(consumption.Domain(domain)) {
		return nil, connect.NewError(
			connect.CodeInvalidArgument, "unknown domain",
		)
	}
	if track != "" && !models.IsTrack(track) {
		return nil, connect.NewError(
			connect.CodeInvalidArgument, "track must be more or broaden",
		)
	}
	recs, st, err := h.app.picks.List(ctx, sc.subject, domain, track, req.SavedOnly)
	if err != nil {
		return nil, mapError(err)
	}
	out := make([]*tastev1.Recommendation, 0, len(recs))
	for _, r := range recs {
		if sc.access(services.AppOf(consumption.Domain(r.Domain))) {
			out = append(out, protoRecommendation(r))
		}
	}
	emailNewPicks, err := h.app.notify.EmailNewPicks(ctx, sc.user)
	if err != nil {
		return nil, mapError(err)
	}
	return &tastev1.ListRecommendationsResponse{
		Recommendations: out,
		GeneratedAt:     formatTimePtr(st.GeneratedAt),
		LastError:       st.Error,
		EmailNewPicks:   emailNewPicks,
		Pending:         st.Pending,
		NextRefreshAt:   nextRefreshAt(st.RequestedAt),
	}, nil
}

func (h *tasteConnectHandler) RefreshRecommendations(
	ctx context.Context,
	req *tastev1.RefreshRecommendationsRequest,
) (*tastev1.RefreshRecommendationsResponse, error) {
	sc, err := h.scopeOf(ctx, req.Shared)
	if err != nil {
		return nil, err
	}
	if !h.app.profiles.Configured() {
		return nil, mapError(services.ErrNotConfigured)
	}
	accepted, err := h.app.picks.Request(ctx, sc.subject)
	if err != nil {
		return nil, mapError(err)
	}
	if !accepted {
		return nil, connect.Errorf(
			connect.CodeResourceExhausted,
			"the picks were refreshed in the last %s",
			services.RefreshCooldown,
		)
	}
	h.app.jobQueue.ForceRun(jobs.GenerateProfilesID)
	return &tastev1.RefreshRecommendationsResponse{}, nil
}

func (h *tasteConnectHandler) SetEmailNewPicks(
	ctx context.Context,
	req *tastev1.SetEmailNewPicksRequest,
) (*tastev1.SetEmailNewPicksResponse, error) {
	uid, err := userID(ctx)
	if err != nil {
		return nil, err
	}
	if err = h.app.notify.SetEmailNewPicks(ctx, uid, req.Enabled); err != nil {
		return nil, mapError(err)
	}
	return &tastev1.SetEmailNewPicksResponse{}, nil
}

func (h *tasteConnectHandler) SetFeedback(
	ctx context.Context,
	req *tastev1.SetFeedbackRequest,
) (*tastev1.SetFeedbackResponse, error) {
	if !models.IsFeedback(req.Feedback) {
		return nil, connect.NewError(
			connect.CodeInvalidArgument,
			"feedback must be hit, miss, not_interested, known or empty",
		)
	}
	uid, rec, err := h.ownPick(ctx, req.Shared, req.Id)
	if err != nil {
		return nil, err
	}
	rec, err = h.app.picks.SetFeedback(ctx, uid, rec.ID, req.Feedback)
	if err != nil {
		return nil, mapError(err)
	}
	return &tastev1.SetFeedbackResponse{
		Recommendation: protoRecommendation(*rec),
	}, nil
}

func (h *tasteConnectHandler) SetSaved(
	ctx context.Context,
	req *tastev1.SetSavedRequest,
) (*tastev1.SetSavedResponse, error) {
	uid, rec, err := h.ownPick(ctx, req.Shared, req.Id)
	if err != nil {
		return nil, err
	}
	rec, err = h.app.picks.SetSaved(ctx, uid, rec.ID, req.Saved)
	if err != nil {
		return nil, mapError(err)
	}
	return &tastev1.SetSavedResponse{
		Recommendation: protoRecommendation(*rec),
	}, nil
}

func (h *tasteConnectHandler) AcceptRecommendation(
	ctx context.Context,
	req *tastev1.AcceptRecommendationRequest,
) (*tastev1.AcceptRecommendationResponse, error) {
	uid, rec, err := h.ownPick(ctx, req.Shared, req.Id)
	if err != nil {
		return nil, err
	}
	rec, err = h.app.picks.Accept(ctx, uid, *rec)
	if err != nil {
		return nil, mapError(err)
	}
	return &tastev1.AcceptRecommendationResponse{
		Recommendation: protoRecommendation(*rec),
	}, nil
}

// ownPick returns the caller's pick id, as not found when they can't open
// its app.
func (h *tasteConnectHandler) ownPick(
	ctx context.Context,
	shared bool,
	id string,
) (string, *models.Recommendation, error) {
	sc, err := h.scopeOf(ctx, shared)
	if err != nil {
		return "", nil, err
	}
	if _, err = uuid.Parse(id); err != nil {
		return "", nil, connect.NewError(
			connect.CodeInvalidArgument, "invalid id",
		)
	}
	rec, err := h.app.picks.Get(ctx, sc.subject, id)
	if err != nil {
		return "", nil, mapError(err)
	}
	if !sc.access(services.AppOf(consumption.Domain(rec.Domain))) {
		return "", nil, connect.NewError(
			connect.CodeNotFound, "recommendation not found",
		)
	}
	return sc.subject, rec, nil
}

func protoRecommendation(r models.Recommendation) *tastev1.Recommendation {
	sources := make([]*tastev1.Source, len(r.Sources))
	for i, s := range r.Sources {
		sources[i] = &tastev1.Source{Url: s.URL, Title: s.Title}
	}
	return &tastev1.Recommendation{
		Id:           r.ID,
		Domain:       r.Domain,
		Track:        r.Track,
		Title:        r.Title,
		Creator:      r.Creator,
		Year:         count32(r.Year),
		Url:          r.URL,
		ImageUrl:     r.ImageURL,
		Why:          r.Why,
		Sources:      sources,
		Score:        count32(r.Score),
		Feedback:     r.Feedback,
		Saved:        r.Saved,
		Accepted:     r.Accepted,
		OnMyServices: r.OnMyServices,
		CanAccept:    r.CanAccept(),
		GeneratedAt:  r.GeneratedAt.UTC().Format(time.RFC3339),
	}
}
