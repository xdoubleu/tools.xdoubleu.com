package connect

import (
	"context"
	"errors"
	"strings"
	"time"

	"connectrpc.com/connect/v2"

	factoryv1 "tools.xdoubleu.com/gen/factory/v1"
	"tools.xdoubleu.com/gen/factory/v1/factoryv1connect"
	"tools.xdoubleu.com/internal/connecttools"
	"tools.xdoubleu.com/internal/kit/database"
	"tools.xdoubleu.com/internal/platform/auth"
	"tools.xdoubleu.com/internal/platform/factory/internal/models"
	"tools.xdoubleu.com/internal/platform/factory/present"
)

// StepUp checks a TOTP or recovery code for the signed-in user's session.
type StepUp interface {
	VerifyCurrentFactor(ctx context.Context, accessToken, code string) error
}

// DefaultHeartbeatHold is how long a Heartbeat is held open before it is
// answered: under the 25-second budget of a long route (adr-0017).
const DefaultHeartbeatHold = 20 * time.Second

const bearerPrefix = "Bearer "

func protoRunner(r *models.Runner, now time.Time) *factoryv1.Runner {
	return &factoryv1.Runner{
		Id:               r.ID.String(),
		Name:             r.Name,
		PairedAt:         formatTime(r.PairedAt),
		LastSeen:         formatTime(r.LastSeen),
		Version:          r.Version,
		RevokedAt:        formatTime(r.RevokedAt),
		PairingExpiresAt: formatTime(r.PairingExpiresAt),
		Online:           r.Online(now),
	}
}

func formatTime(t *time.Time) string {
	if t == nil {
		return ""
	}
	return t.Format(time.RFC3339)
}

// runnerAdminHandler is the admin side of pairing: an admin in a browser
// session, never a bearer token.
type runnerAdminHandler struct {
	app *Deps
}

var _ factoryv1connect.RunnerAdminServiceHandler = (*runnerAdminHandler)(nil)

func (h *runnerAdminHandler) ListRunners(
	ctx context.Context,
	_ *factoryv1.ListRunnersRequest,
) (*factoryv1.ListRunnersResponse, error) {
	actor, err := actorFrom(ctx)
	if err != nil {
		return nil, err
	}
	runners, err := h.app.Services.Runners.List(ctx, actor)
	if err != nil {
		return nil, present.MapError(err)
	}
	now := h.app.Services.Runners.Now()
	out := make([]*factoryv1.Runner, len(runners))
	for i := range runners {
		out[i] = protoRunner(&runners[i], now)
	}
	return &factoryv1.ListRunnersResponse{Runners: out}, nil
}

func (h *runnerAdminHandler) CreateRunner(
	ctx context.Context,
	req *factoryv1.CreateRunnerRequest,
) (*factoryv1.CreateRunnerResponse, error) {
	actor, err := actorFrom(ctx)
	if err != nil {
		return nil, err
	}
	runner, code, err := h.app.Services.Runners.Create(
		ctx, actor, req.GetName(), func(ctx context.Context) error {
			return h.verifyStepUp(ctx, req.GetTotpCode())
		},
	)
	if err != nil {
		return nil, mapRunnerError(err)
	}
	return &factoryv1.CreateRunnerResponse{
		Runner:      protoRunner(runner, h.app.Services.Runners.Now()),
		PairingCode: code,
	}, nil
}

// verifyStepUp checks the code against the signed-in session.
func (h *runnerAdminHandler) verifyStepUp(ctx context.Context, code string) error {
	token, ok := connecttools.RequestCookie(ctx, "accessToken")
	if !ok {
		return connect.NewError(connect.CodeUnauthenticated, "not signed in")
	}
	if h.app.StepUp == nil {
		return connect.NewError(
			connect.CodeFailedPrecondition, "the step-up check isn't available",
		)
	}
	if err := h.app.StepUp.VerifyCurrentFactor(ctx, token, code); err != nil {
		return connect.NewError(stepUpCode(err), err.Error()).WithCause(err)
	}
	return nil
}

func (h *runnerAdminHandler) RevokeRunner(
	ctx context.Context,
	req *factoryv1.RevokeRunnerRequest,
) (*factoryv1.RevokeRunnerResponse, error) {
	actor, err := actorFrom(ctx)
	if err != nil {
		return nil, err
	}
	id, err := present.ParseID(req.GetId())
	if err != nil {
		return nil, err
	}
	runner, err := h.app.Services.Runners.Revoke(ctx, actor, id)
	if err != nil {
		return nil, present.MapError(err)
	}
	return &factoryv1.RevokeRunnerResponse{
		Runner: protoRunner(runner, h.app.Services.Runners.Now()),
	}, nil
}

// mapRunnerError adds the one runner-specific answer to mapError: a second
// active runner is a conflict the admin resolves by revoking the first.
func mapRunnerError(err error) error {
	if errors.Is(err, database.ErrResourceConflict) {
		return connect.NewError(
			connect.CodeAlreadyExists,
			"a runner is already active: revoke it before pairing another",
		).WithCause(err)
	}
	return present.MapError(err)
}

// stepUpCode maps a failed step-up check to a Connect code.
func stepUpCode(err error) connect.Code {
	switch {
	case errors.Is(err, auth.ErrTooManyAttempts):
		return connect.CodeResourceExhausted
	case errors.Is(err, auth.ErrWrongCredential):
		return connect.CodePermissionDenied
	default:
		return connect.CodeInternal
	}
}

// runnerHandler is the runner's own side. It takes no cookie or MCP token, and
// reaches nothing but the runner's table.
type runnerHandler struct {
	app *Deps
}

var _ factoryv1connect.RunnerServiceHandler = (*runnerHandler)(nil)

func (h *runnerHandler) Pair(
	ctx context.Context,
	req *factoryv1.PairRequest,
) (*factoryv1.PairResponse, error) {
	token, runner, err := h.app.Services.Runners.Pair(
		ctx, req.GetCode(), req.GetVersion(),
	)
	if errors.Is(err, database.ErrResourceNotFound) {
		return nil, connect.NewError(
			connect.CodePermissionDenied, "invalid or expired pairing code",
		)
	}
	if err != nil {
		return nil, present.MapError(err)
	}
	return &factoryv1.PairResponse{DeviceToken: token, RunnerName: runner.Name}, nil
}

// runnerFrom authenticates a call by its bearer device token.
func (h *runnerHandler) runnerFrom(ctx context.Context) (*models.Runner, error) {
	header := connecttools.RequestHeader(ctx, "Authorization")
	token, found := strings.CutPrefix(header, bearerPrefix)
	if !found {
		return nil, connect.NewError(connect.CodeUnauthenticated, "no device token")
	}
	runner, err := h.app.Services.Runners.Authenticate(ctx, token)
	if errors.Is(err, database.ErrResourceNotFound) {
		return nil, connect.NewError(connect.CodeUnauthenticated, "unknown device token")
	}
	if err != nil {
		return nil, present.MapError(err)
	}
	return runner, nil
}

func (h *runnerHandler) Heartbeat(
	ctx context.Context,
	req *factoryv1.HeartbeatRequest,
) (*factoryv1.HeartbeatResponse, error) {
	runner, err := h.runnerFrom(ctx)
	if err != nil {
		return nil, err
	}
	queued, err := h.app.Services.Jobs.HoldHeartbeat(
		ctx, runner.ID, req.GetVersion(), req.GetRepos(), req.GetReposReported(),
		h.app.HeartbeatHold,
	)
	if err != nil {
		return nil, present.MapError(err)
	}
	//nolint:gosec // a count of jobs fits an int32
	return &factoryv1.HeartbeatResponse{QueuedJobs: int32(queued)}, nil
}
