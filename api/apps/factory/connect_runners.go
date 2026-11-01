package factory

import (
	"context"
	"errors"
	"strings"
	"time"

	"connectrpc.com/connect/v2"

	"tools.xdoubleu.com/apps/factory/internal/models"
	"tools.xdoubleu.com/apps/factory/internal/services"
	factoryv1 "tools.xdoubleu.com/gen/factory/v1"
	"tools.xdoubleu.com/gen/factory/v1/factoryv1connect"
	"tools.xdoubleu.com/internal/auth"
	"tools.xdoubleu.com/internal/connecttools"
	"tools.xdoubleu.com/internal/database"
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
	app *Factory
}

var _ factoryv1connect.RunnerAdminServiceHandler = (*runnerAdminHandler)(nil)

func browserAdmin(ctx context.Context) (models.Actor, error) {
	actor, err := actorFrom(ctx)
	if err != nil {
		return models.Actor{}, err
	}
	if !actor.Browser {
		return models.Actor{}, connect.NewError(
			connect.CodePermissionDenied,
			"runners can only be managed in a browser session",
		)
	}
	return actor, nil
}

func (h *runnerAdminHandler) ListRunners(
	ctx context.Context,
	_ *factoryv1.ListRunnersRequest,
) (*factoryv1.ListRunnersResponse, error) {
	if _, err := browserAdmin(ctx); err != nil {
		return nil, err
	}
	runners, err := h.app.services.Runners.List(ctx)
	if err != nil {
		return nil, mapError(err)
	}
	now := h.app.services.Runners.Now()
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
	actor, err := browserAdmin(ctx)
	if err != nil {
		return nil, err
	}
	if _, valid := services.ValidRunnerName(req.GetName()); !valid {
		return nil, connect.NewError(
			connect.CodeInvalidArgument, "name must be 1 to 80 printable characters",
		)
	}
	token, ok := connecttools.RequestCookie(ctx, "accessToken")
	if !ok {
		return nil, connect.NewError(connect.CodeUnauthenticated, "not signed in")
	}
	if h.app.stepUp == nil {
		return nil, connect.NewError(
			connect.CodeFailedPrecondition, "the step-up check isn't available",
		)
	}
	if err = h.app.stepUp.VerifyCurrentFactor(
		ctx, token, req.GetTotpCode(),
	); err != nil {
		return nil, connect.NewError(stepUpCode(err), err.Error()).WithCause(err)
	}

	runner, code, err := h.app.services.Runners.Create(ctx, actor.String(), req.GetName())
	if err != nil {
		return nil, mapRunnerError(err)
	}
	return &factoryv1.CreateRunnerResponse{
		Runner:      protoRunner(runner, h.app.services.Runners.Now()),
		PairingCode: code,
	}, nil
}

func (h *runnerAdminHandler) RevokeRunner(
	ctx context.Context,
	req *factoryv1.RevokeRunnerRequest,
) (*factoryv1.RevokeRunnerResponse, error) {
	if _, err := browserAdmin(ctx); err != nil {
		return nil, err
	}
	id, err := parseID(req.GetId())
	if err != nil {
		return nil, err
	}
	runner, err := h.app.services.Runners.Revoke(ctx, id)
	if err != nil {
		return nil, mapError(err)
	}
	return &factoryv1.RevokeRunnerResponse{
		Runner: protoRunner(runner, h.app.services.Runners.Now()),
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
	return mapError(err)
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
	app *Factory
}

var _ factoryv1connect.RunnerServiceHandler = (*runnerHandler)(nil)

func (h *runnerHandler) Pair(
	ctx context.Context,
	req *factoryv1.PairRequest,
) (*factoryv1.PairResponse, error) {
	token, runner, err := h.app.services.Runners.Pair(
		ctx, req.GetCode(), req.GetVersion(),
	)
	if errors.Is(err, database.ErrResourceNotFound) {
		return nil, connect.NewError(
			connect.CodePermissionDenied, "invalid or expired pairing code",
		)
	}
	if err != nil {
		return nil, mapError(err)
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
	runner, err := h.app.services.Runners.Authenticate(ctx, token)
	if errors.Is(err, database.ErrResourceNotFound) {
		return nil, connect.NewError(connect.CodeUnauthenticated, "unknown device token")
	}
	if err != nil {
		return nil, mapError(err)
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
	runners := h.app.services.Runners
	if err = runners.Heartbeat(ctx, runner.ID, req.GetVersion()); err != nil {
		return nil, mapError(err)
	}

	// Held open so the runner needs no inbound port and learns of work the moment
	// there is some (jobs come later); then answered, and it calls again.
	timer := time.NewTimer(h.app.heartbeatHold)
	defer timer.Stop()
	select {
	case <-timer.C:
	case <-ctx.Done():
		return &factoryv1.HeartbeatResponse{}, nil
	}
	if err = runners.Heartbeat(ctx, runner.ID, req.GetVersion()); err != nil {
		return nil, mapError(err)
	}
	return &factoryv1.HeartbeatResponse{}, nil
}
