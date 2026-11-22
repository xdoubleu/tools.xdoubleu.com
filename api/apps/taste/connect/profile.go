package connect

import (
	"context"

	"connectrpc.com/connect/v2"

	"tools.xdoubleu.com/apps/taste/internal/jobs"
	"tools.xdoubleu.com/apps/taste/internal/protomap"
	"tools.xdoubleu.com/apps/taste/internal/services"
	tastev1 "tools.xdoubleu.com/gen/taste/v1"
)

func (h *Handler) GetProfile(
	ctx context.Context,
	req *tastev1.GetProfileRequest,
) (*tastev1.GetProfileResponse, error) {
	sc, err := h.scopeOf(ctx, req.Shared)
	if err != nil {
		return nil, err
	}

	out := &tastev1.GetProfileResponse{Configured: h.svc.Profiles.Configured()}
	if !out.Configured {
		return out, nil
	}
	p, err := h.svc.Profiles.Get(ctx, sc.Subject)
	if err != nil {
		return nil, mapError(err)
	}
	if p != nil {
		protomap.FillProfile(out, p)
	}
	return out, nil
}

func (h *Handler) RefreshProfile(
	ctx context.Context,
	req *tastev1.RefreshProfileRequest,
) (*tastev1.RefreshProfileResponse, error) {
	sc, err := h.scopeOf(ctx, req.Shared)
	if err != nil {
		return nil, err
	}
	accepted, err := h.svc.Profiles.Request(ctx, sc.Subject)
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
	h.jobs.ForceRun(jobs.GenerateProfilesID)
	return &tastev1.RefreshProfileResponse{}, nil
}
