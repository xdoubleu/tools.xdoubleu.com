package taste

import (
	"context"
	"errors"
	"time"

	"connectrpc.com/connect"

	"tools.xdoubleu.com/apps/taste/internal/jobs"
	"tools.xdoubleu.com/apps/taste/internal/models"
	"tools.xdoubleu.com/apps/taste/internal/services"
	tastev1 "tools.xdoubleu.com/gen/taste/v1"
	"tools.xdoubleu.com/internal/connecttools"
)

func (h *tasteConnectHandler) GetProfile(
	ctx context.Context,
	_ *connect.Request[tastev1.GetProfileRequest],
) (*connect.Response[tastev1.GetProfileResponse], error) {
	uid, err := userID(ctx)
	if err != nil {
		return nil, err
	}

	out := &tastev1.GetProfileResponse{Configured: h.app.profiles.Configured()}
	if !out.Configured {
		return connect.NewResponse(out), nil
	}
	p, err := h.app.profiles.Get(ctx, uid)
	if err != nil {
		return nil, connecttools.MapError(err)
	}
	if p != nil {
		fillProfile(out, p)
	}
	return connect.NewResponse(out), nil
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
	if p.RequestedAt != nil {
		if next := p.RequestedAt.Add(services.RefreshCooldown); next.After(time.Now()) {
			out.NextRefreshAt = formatTime(next)
		}
	}
}

func (h *tasteConnectHandler) RefreshProfile(
	ctx context.Context,
	_ *connect.Request[tastev1.RefreshProfileRequest],
) (*connect.Response[tastev1.RefreshProfileResponse], error) {
	uid, err := userID(ctx)
	if err != nil {
		return nil, err
	}
	accepted, err := h.app.profiles.Request(ctx, uid)
	if errors.Is(err, services.ErrNotConfigured) {
		return nil, connect.NewError(connect.CodeFailedPrecondition, err)
	}
	if err != nil {
		return nil, connecttools.MapError(err)
	}
	if !accepted {
		return nil, connect.NewError(
			connect.CodeResourceExhausted,
			errors.New("the profile was refreshed in the last 6 hours"),
		)
	}
	h.app.jobQueue.ForceRun(jobs.GenerateProfilesID)
	return connect.NewResponse(&tastev1.RefreshProfileResponse{}), nil
}

func formatTimePtr(t *time.Time) string {
	if t == nil {
		return ""
	}
	return formatTime(*t)
}
