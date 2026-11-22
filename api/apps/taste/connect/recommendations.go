package connect

import (
	"context"

	"connectrpc.com/connect/v2"

	"tools.xdoubleu.com/apps/taste/internal/jobs"
	"tools.xdoubleu.com/apps/taste/internal/models"
	"tools.xdoubleu.com/apps/taste/internal/protomap"
	"tools.xdoubleu.com/apps/taste/internal/services"
	tastev1 "tools.xdoubleu.com/gen/taste/v1"
)

func (h *Handler) ListRecommendations(
	ctx context.Context,
	req *tastev1.ListRecommendationsRequest,
) (*tastev1.ListRecommendationsResponse, error) {
	sc, err := h.scopeOf(ctx, req.Shared)
	if err != nil {
		return nil, err
	}
	recs, st, err := h.svc.Picks.Visible(
		ctx, sc, req.Domain, req.Track, req.SavedOnly,
	)
	if err != nil {
		return nil, mapError(err)
	}
	out := make([]*tastev1.Recommendation, 0, len(recs))
	for _, r := range recs {
		out = append(out, protomap.Recommendation(r))
	}
	emailNewPicks, err := h.svc.Notify.EmailNewPicks(ctx, sc.User)
	if err != nil {
		return nil, mapError(err)
	}
	return &tastev1.ListRecommendationsResponse{
		Recommendations: out,
		GeneratedAt:     protomap.TimePtr(st.GeneratedAt),
		LastError:       st.Error,
		EmailNewPicks:   emailNewPicks,
		Pending:         st.Pending,
		NextRefreshAt:   protomap.NextRefreshAt(st.RequestedAt),
	}, nil
}

func (h *Handler) RefreshRecommendations(
	ctx context.Context,
	req *tastev1.RefreshRecommendationsRequest,
) (*tastev1.RefreshRecommendationsResponse, error) {
	sc, err := h.scopeOf(ctx, req.Shared)
	if err != nil {
		return nil, err
	}
	if !h.svc.Profiles.Configured() {
		return nil, mapError(services.ErrNotConfigured)
	}
	accepted, err := h.svc.Picks.Request(ctx, sc.Subject)
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
	h.jobs.ForceRun(jobs.GenerateProfilesID)
	return &tastev1.RefreshRecommendationsResponse{}, nil
}

func (h *Handler) SetEmailNewPicks(
	ctx context.Context,
	req *tastev1.SetEmailNewPicksRequest,
) (*tastev1.SetEmailNewPicksResponse, error) {
	uid, err := userID(ctx)
	if err != nil {
		return nil, err
	}
	if err = h.svc.Notify.SetEmailNewPicks(ctx, uid, req.Enabled); err != nil {
		return nil, mapError(err)
	}
	return &tastev1.SetEmailNewPicksResponse{}, nil
}

func (h *Handler) SetFeedback(
	ctx context.Context,
	req *tastev1.SetFeedbackRequest,
) (*tastev1.SetFeedbackResponse, error) {
	uid, rec, err := h.ownPick(ctx, req.Shared, req.Id)
	if err != nil {
		return nil, err
	}
	rec, err = h.svc.Picks.SetFeedback(ctx, uid, rec.ID, req.Feedback)
	if err != nil {
		return nil, mapError(err)
	}
	return &tastev1.SetFeedbackResponse{
		Recommendation: protomap.Recommendation(*rec),
	}, nil
}

func (h *Handler) SetSaved(
	ctx context.Context,
	req *tastev1.SetSavedRequest,
) (*tastev1.SetSavedResponse, error) {
	uid, rec, err := h.ownPick(ctx, req.Shared, req.Id)
	if err != nil {
		return nil, err
	}
	rec, err = h.svc.Picks.SetSaved(ctx, uid, rec.ID, req.Saved)
	if err != nil {
		return nil, mapError(err)
	}
	return &tastev1.SetSavedResponse{
		Recommendation: protomap.Recommendation(*rec),
	}, nil
}

func (h *Handler) AcceptRecommendation(
	ctx context.Context,
	req *tastev1.AcceptRecommendationRequest,
) (*tastev1.AcceptRecommendationResponse, error) {
	uid, rec, err := h.ownPick(ctx, req.Shared, req.Id)
	if err != nil {
		return nil, err
	}
	rec, err = h.svc.Picks.Accept(ctx, uid, *rec)
	if err != nil {
		return nil, mapError(err)
	}
	return &tastev1.AcceptRecommendationResponse{
		Recommendation: protomap.Recommendation(*rec),
	}, nil
}

// ownPick returns the scope's subject and its pick id.
func (h *Handler) ownPick(
	ctx context.Context,
	shared bool,
	id string,
) (string, *models.Recommendation, error) {
	sc, err := h.scopeOf(ctx, shared)
	if err != nil {
		return "", nil, err
	}
	rec, err := h.svc.Picks.Own(ctx, sc, id)
	if err != nil {
		return "", nil, mapError(err)
	}
	return sc.Subject, rec, nil
}
