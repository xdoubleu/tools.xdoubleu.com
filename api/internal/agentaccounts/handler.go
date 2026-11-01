package agentaccounts

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"connectrpc.com/connect/v2"
	"github.com/google/uuid"

	factoryv1 "tools.xdoubleu.com/gen/factory/v1"
	"tools.xdoubleu.com/gen/factory/v1/factoryv1connect"
	iapp "tools.xdoubleu.com/internal/app"
	"tools.xdoubleu.com/internal/auth"
	"tools.xdoubleu.com/internal/connecttools"
	"tools.xdoubleu.com/internal/constants"
	"tools.xdoubleu.com/internal/contexttools"
	"tools.xdoubleu.com/internal/models"
)

const maxNameLen = 80

// StepUp checks a TOTP or recovery code for the signed-in user's session.
type StepUp interface {
	VerifyCurrentFactor(ctx context.Context, accessToken, code string) error
}

// Deps is what the agent account RPCs need.
type Deps struct {
	Store  *Store
	StepUp StepUp
}

// Register mounts the AgentAccountService behind gate.
func Register(mux *http.ServeMux, logger *slog.Logger, gate iapp.Gate, d *Deps) {
	iapp.MountConnect(mux, logger, gate, func(s *connect.Server) {
		factoryv1connect.RegisterAgentAccountServiceHandler(
			s, &handler{deps: d},
		)
	})
}

type handler struct {
	deps *Deps
}

var _ factoryv1connect.AgentAccountServiceHandler = (*handler)(nil)

// adminFrom returns the signed-in admin. Agent accounts are created and ended
// only by an admin in a browser session, never through a bearer token.
func adminFrom(ctx context.Context) (*models.User, error) {
	user := contexttools.GetValue[models.User](ctx, constants.UserContextKey)
	if user == nil {
		return nil, connect.NewError(
			connect.CodeUnauthenticated, "user not authenticated",
		)
	}
	if user.Role != models.RoleAdmin {
		return nil, connect.NewError(
			connect.CodePermissionDenied, "admin access required",
		)
	}
	channel := contexttools.GetValue[string](ctx, constants.AuthChannelContextKey)
	if channel == nil || *channel != constants.ChannelBrowser {
		return nil, connect.NewError(
			connect.CodePermissionDenied,
			"agent accounts can only be managed in a browser session",
		)
	}
	return user, nil
}

// validName trims name and rejects an empty, overlong or control-character one.
func validName(name string) (string, bool) {
	name = strings.TrimSpace(name)
	if name == "" || utf8.RuneCountInString(name) > maxNameLen {
		return "", false
	}
	for _, r := range name {
		if unicode.IsControl(r) {
			return "", false
		}
	}
	return name, true
}

func protoAccount(a *Account) *factoryv1.AgentAccount {
	revoked := ""
	if a.RevokedAt != nil {
		revoked = a.RevokedAt.Format(time.RFC3339)
	}
	return &factoryv1.AgentAccount{
		Id:        a.ID.String(),
		Name:      a.Name,
		UserId:    a.UserID.String(),
		ClientId:  a.ClientID,
		CreatedBy: a.CreatedBy,
		CreatedAt: a.CreatedAt.Format(time.RFC3339),
		RevokedAt: revoked,
	}
}

func (h *handler) ListAgentAccounts(
	ctx context.Context,
	_ *factoryv1.ListAgentAccountsRequest,
) (*factoryv1.ListAgentAccountsResponse, error) {
	if _, err := adminFrom(ctx); err != nil {
		return nil, err
	}
	accounts, err := h.deps.Store.List(ctx)
	if err != nil {
		return nil, connecttools.MapError(err)
	}
	out := make([]*factoryv1.AgentAccount, len(accounts))
	for i := range accounts {
		out[i] = protoAccount(&accounts[i])
	}
	return &factoryv1.ListAgentAccountsResponse{Accounts: out}, nil
}

func (h *handler) CreateAgentAccount(
	ctx context.Context,
	req *factoryv1.CreateAgentAccountRequest,
) (*factoryv1.CreateAgentAccountResponse, error) {
	user, err := adminFrom(ctx)
	if err != nil {
		return nil, err
	}
	name, ok := validName(req.GetName())
	if !ok {
		return nil, connect.NewError(
			connect.CodeInvalidArgument, "name must be 1 to 80 printable characters",
		)
	}

	token, ok := connecttools.RequestCookie(ctx, "accessToken")
	if !ok {
		return nil, connect.NewError(connect.CodeUnauthenticated, "not signed in")
	}
	if err = h.deps.StepUp.VerifyCurrentFactor(
		ctx, token, req.GetTotpCode(),
	); err != nil {
		return nil, connect.NewError(stepUpCode(err), err.Error()).WithCause(err)
	}

	account, secret, err := h.deps.Store.Create(ctx, "human:"+user.ID, name)
	if err != nil {
		return nil, connecttools.MapError(err)
	}
	return &factoryv1.CreateAgentAccountResponse{
		Account:      protoAccount(account),
		ClientSecret: secret,
	}, nil
}

func (h *handler) RevokeAgentAccount(
	ctx context.Context,
	req *factoryv1.RevokeAgentAccountRequest,
) (*factoryv1.RevokeAgentAccountResponse, error) {
	if _, err := adminFrom(ctx); err != nil {
		return nil, err
	}
	id, err := uuid.Parse(req.GetId())
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, "invalid id")
	}
	account, err := h.deps.Store.Revoke(ctx, id)
	if err != nil {
		return nil, connecttools.MapError(err)
	}
	return &factoryv1.RevokeAgentAccountResponse{Account: protoAccount(account)}, nil
}

// stepUpCode maps a failed step-up check to the code the account RPCs use.
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
