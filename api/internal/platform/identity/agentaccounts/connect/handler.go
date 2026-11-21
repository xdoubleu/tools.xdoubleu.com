// Package connect serves the AgentAccountService over ConnectRPC. It maps
// requests and errors; the rules live in agentaccounts.Service.
package connect

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"time"

	rpc "connectrpc.com/connect/v2"

	factoryv1 "tools.xdoubleu.com/gen/factory/v1"
	"tools.xdoubleu.com/gen/factory/v1/factoryv1connect"
	iapp "tools.xdoubleu.com/internal/app"
	"tools.xdoubleu.com/internal/connecttools"
	"tools.xdoubleu.com/internal/kit/constants"
	"tools.xdoubleu.com/internal/kit/contexttools"
	"tools.xdoubleu.com/internal/models"
	"tools.xdoubleu.com/internal/platform/identity/agentaccounts"
)

// Register mounts the AgentAccountService behind gate.
func Register(
	mux *http.ServeMux,
	logger *slog.Logger,
	gate iapp.Gate,
	svc *agentaccounts.Service,
) {
	iapp.MountConnect(mux, logger, gate, func(s *rpc.Server) {
		factoryv1connect.RegisterAgentAccountServiceHandler(s, &handler{svc: svc})
	})
}

type handler struct {
	svc *agentaccounts.Service
}

var _ factoryv1connect.AgentAccountServiceHandler = (*handler)(nil)

// callerFrom reads who the request comes from out of its context.
func callerFrom(ctx context.Context) agentaccounts.Caller {
	user := contexttools.GetValue[models.User](ctx, constants.UserContextKey)
	if user == nil {
		return agentaccounts.Caller{} //nolint:exhaustruct // unauthenticated
	}
	channel := contexttools.GetValue[string](ctx, constants.AuthChannelContextKey)
	token, _ := connecttools.RequestCookie(ctx, "accessToken")
	return agentaccounts.Caller{
		UserID:      user.ID,
		Admin:       user.Role == models.RoleAdmin,
		Browser:     channel != nil && *channel == constants.ChannelBrowser,
		AccessToken: token,
	}
}

// mapError translates a service error into the Connect error the RPCs return.
func mapError(err error) error {
	switch {
	case errors.Is(err, agentaccounts.ErrUnauthenticated),
		errors.Is(err, agentaccounts.ErrNotSignedIn):
		return rpc.NewError(rpc.CodeUnauthenticated, err.Error())
	case errors.Is(err, agentaccounts.ErrNotAdmin),
		errors.Is(err, agentaccounts.ErrNotBrowser):
		return rpc.NewError(rpc.CodePermissionDenied, err.Error())
	case errors.Is(err, agentaccounts.ErrInvalidName),
		errors.Is(err, agentaccounts.ErrInvalidID):
		return rpc.NewError(rpc.CodeInvalidArgument, err.Error())
	case errors.Is(err, agentaccounts.ErrTooManyAttempts):
		return rpc.NewError(rpc.CodeResourceExhausted, err.Error()).WithCause(err)
	case errors.Is(err, agentaccounts.ErrWrongCredential):
		return rpc.NewError(rpc.CodePermissionDenied, err.Error()).WithCause(err)
	}
	return connecttools.MapError(err)
}

func protoAccount(a *agentaccounts.Account) *factoryv1.AgentAccount {
	revoked := ""
	if a.RevokedAt != nil {
		revoked = a.RevokedAt.Format(time.RFC3339)
	}
	return &factoryv1.AgentAccount{
		Id:          a.ID.String(),
		Name:        a.Name,
		UserId:      a.UserID.String(),
		ClientId:    a.ClientID,
		CreatedBy:   a.CreatedBy,
		CreatedAt:   a.CreatedAt.Format(time.RFC3339),
		RevokedAt:   revoked,
		AppAccess:   a.AppAccess,
		WriteAccess: a.WriteAccess,
	}
}

func (h *handler) ListAgentAccounts(
	ctx context.Context,
	_ *factoryv1.ListAgentAccountsRequest,
) (*factoryv1.ListAgentAccountsResponse, error) {
	accounts, err := h.svc.List(ctx, callerFrom(ctx))
	if err != nil {
		return nil, mapError(err)
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
	account, secret, err := h.svc.Create(ctx, callerFrom(ctx), req.GetName())
	if err != nil {
		return nil, mapError(err)
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
	account, err := h.svc.Revoke(ctx, callerFrom(ctx), req.GetId())
	if err != nil {
		return nil, mapError(err)
	}
	return &factoryv1.RevokeAgentAccountResponse{Account: protoAccount(account)}, nil
}

func (h *handler) RotateAgentAccountSecret(
	ctx context.Context,
	req *factoryv1.RotateAgentAccountSecretRequest,
) (*factoryv1.RotateAgentAccountSecretResponse, error) {
	account, secret, err := h.svc.RotateSecret(
		ctx, callerFrom(ctx), req.GetId(), req.GetTotpCode(),
	)
	if err != nil {
		return nil, mapError(err)
	}
	return &factoryv1.RotateAgentAccountSecretResponse{
		Account:      protoAccount(account),
		ClientSecret: secret,
	}, nil
}

func (h *handler) AddAgentAccountToFamily(
	ctx context.Context,
	req *factoryv1.AddAgentAccountToFamilyRequest,
) (*factoryv1.AddAgentAccountToFamilyResponse, error) {
	account, err := h.svc.AddToFamily(
		ctx, callerFrom(ctx), req.GetId(), req.GetTotpCode(),
	)
	if err != nil {
		return nil, mapError(err)
	}
	return &factoryv1.AddAgentAccountToFamilyResponse{
		Account: protoAccount(account),
	}, nil
}

func (h *handler) RemoveAgentAccountFromFamily(
	ctx context.Context,
	req *factoryv1.RemoveAgentAccountFromFamilyRequest,
) (*factoryv1.RemoveAgentAccountFromFamilyResponse, error) {
	account, err := h.svc.RemoveFromFamily(ctx, callerFrom(ctx), req.GetId())
	if err != nil {
		return nil, mapError(err)
	}
	return &factoryv1.RemoveAgentAccountFromFamilyResponse{
		Account: protoAccount(account),
	}, nil
}
