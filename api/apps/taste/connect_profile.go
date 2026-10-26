package taste

import (
	"context"
	"time"

	"connectrpc.com/connect/v2"

	"tools.xdoubleu.com/apps/taste/internal/jobs"
	"tools.xdoubleu.com/apps/taste/internal/models"
	"tools.xdoubleu.com/apps/taste/internal/services"
	tastev1 "tools.xdoubleu.com/gen/taste/v1"
)

func (h *tasteConnectHandler) GetProfile(
	ctx context.Context,
	req *tastev1.GetProfileRequest,
) (*tastev1.GetProfileResponse, error) {
	sc, err := h.scopeOf(ctx, req.Shared)
	if err != nil {
		return nil, err
	}

	out := &tastev1.GetProfileResponse{Configured: h.app.profiles.Configured()}
	if !out.Configured {
		return out, nil
	}
	p, err := h.app.profiles.Get(ctx, sc.subject)
	if err != nil {
		return nil, mapError(err)
	}
	if p != nil {
		fillProfile(out, p)
	}
	return out, nil
}

func fillProfile(out *tastev1.GetProfileResponse, p *models.Profile) {
	if p.Sections != nil {
		out.Sections = &tastev1.ProfileSections{
			Summary:  p.Sections.Summary,
			Know:     p.Sections.Know,
			Like:     p.Sections.Like,
			Drifting: p.Sections.Drifting,
			Bounce:   p.Sections.Bounce,
		}
	}
	out.Model = p.Model
	out.GeneratedAt = formatTimePtr(p.GeneratedAt)
	out.Pending = p.Pending
	out.LastError = p.LastError
	out.NextRefreshAt = nextRefreshAt(p.RequestedAt)
}

// nextRefreshAt is when a refresh requested at requested is accepted again;
// empty when now.
func nextRefreshAt(requested *time.Time) string {
	if requested == nil {
		return ""
	}
	if next := requested.Add(services.RefreshCooldown); next.After(time.Now()) {
		return formatTime(next)
	}
	return ""
}

func (h *tasteConnectHandler) RefreshProfile(
	ctx context.Context,
	req *tastev1.RefreshProfileRequest,
) (*tastev1.RefreshProfileResponse, error) {
	sc, err := h.scopeOf(ctx, req.Shared)
	if err != nil {
		return nil, err
	}
	accepted, err := h.app.profiles.Request(ctx, sc.subject)
	if err != nil {
		return nil, mapError(err)
	}
	if !accepted {
		return nil, connect.Errorf(
			connect.CodeResourceExhausted,
			"the profile was refreshed in the last %s",
			services.RefreshCooldown,
		)
	}
	h.app.jobQueue.ForceRun(jobs.GenerateProfilesID)
	return &tastev1.RefreshProfileResponse{}, nil
}

func formatTimePtr(t *time.Time) string {
	if t == nil {
		return ""
	}
	return formatTime(*t)
}
