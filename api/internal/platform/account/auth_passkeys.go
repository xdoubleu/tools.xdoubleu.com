package account

import (
	"context"
	"errors"
	"time"

	"connectrpc.com/connect/v2"
	"github.com/google/uuid"

	authv1 "tools.xdoubleu.com/gen/auth/v1"
	"tools.xdoubleu.com/internal/platform/auth"
)

// passkeyErrorCode adds the passkey rejections to AuthErrorCode's mapping. A
// stale sign-in is FailedPrecondition so the client can send the user to
// reauthenticate.
func passkeyErrorCode(err error) connect.Code {
	switch {
	case errors.Is(err, auth.ErrRecentSignInRequired):
		return connect.CodeFailedPrecondition
	case errors.Is(err, auth.ErrPasskeyInvalid):
		return connect.CodeInvalidArgument
	case errors.Is(err, auth.ErrPasskeyNotFound):
		return connect.CodeNotFound
	case errors.Is(err, auth.ErrPasskeysUnavailable):
		return connect.CodeUnavailable
	default:
		return AuthErrorCode(err)
	}
}

func passkeyError(err error) error {
	return connect.NewError(passkeyErrorCode(err), err.Error()).WithCause(err)
}

// accessTokenOf reads the signed-in session's access token; passkeys are for
// people already signed in, never the MFA step-up cookies.
func (h *authConnectHandler) accessTokenOf(ctx context.Context) (string, error) {
	info, _ := connect.CallInfoForServerContext(ctx)
	cookie, err := h.parseCookie(info.RequestHeader(), "accessToken")
	if err != nil {
		return "", connect.NewError(connect.CodeUnauthenticated, "not signed in")
	}
	return cookie.Value, nil
}

func passkeyProto(p auth.Passkey) *authv1.Passkey {
	out := &authv1.Passkey{
		Id:        p.ID.String(),
		Name:      p.Name,
		CreatedAt: p.CreatedAt.UTC().Format(time.RFC3339),
	}
	if p.LastUsedAt != nil {
		out.LastUsedAt = p.LastUsedAt.UTC().Format(time.RFC3339)
	}
	return out
}

func parsePasskeyID(id string) (uuid.UUID, error) {
	parsed, err := uuid.Parse(id)
	if err != nil {
		return uuid.UUID{}, connect.NewError(
			connect.CodeInvalidArgument, "invalid passkey id",
		)
	}
	return parsed, nil
}

func (h *authConnectHandler) PasskeyRegisterBegin(
	ctx context.Context,
	_ *authv1.PasskeyRegisterBeginRequest,
) (*authv1.PasskeyRegisterBeginResponse, error) {
	token, err := h.accessTokenOf(ctx)
	if err != nil {
		return nil, err
	}
	options, err := h.app.Auth.BeginPasskeyRegistration(ctx, token)
	if err != nil {
		return nil, passkeyError(err)
	}
	return &authv1.PasskeyRegisterBeginResponse{OptionsJson: string(options)}, nil
}

func (h *authConnectHandler) PasskeyRegisterFinish(
	ctx context.Context,
	req *authv1.PasskeyRegisterFinishRequest,
) (*authv1.PasskeyRegisterFinishResponse, error) {
	token, err := h.accessTokenOf(ctx)
	if err != nil {
		return nil, err
	}
	passkey, err := h.app.Auth.FinishPasskeyRegistration(
		ctx, token, req.Name, []byte(req.CredentialJson),
	)
	if err != nil {
		return nil, passkeyError(err)
	}
	return &authv1.PasskeyRegisterFinishResponse{Passkey: passkeyProto(*passkey)}, nil
}

func (h *authConnectHandler) ListPasskeys(
	ctx context.Context,
	_ *authv1.ListPasskeysRequest,
) (*authv1.ListPasskeysResponse, error) {
	token, err := h.accessTokenOf(ctx)
	if err != nil {
		return nil, err
	}
	passkeys, err := h.app.Auth.ListPasskeys(ctx, token)
	if err != nil {
		return nil, passkeyError(err)
	}
	resp := &authv1.ListPasskeysResponse{
		Passkeys: make([]*authv1.Passkey, 0, len(passkeys)),
	}
	for _, p := range passkeys {
		resp.Passkeys = append(resp.Passkeys, passkeyProto(p))
	}
	return resp, nil
}

func (h *authConnectHandler) RenamePasskey(
	ctx context.Context,
	req *authv1.RenamePasskeyRequest,
) (*authv1.RenamePasskeyResponse, error) {
	token, err := h.accessTokenOf(ctx)
	if err != nil {
		return nil, err
	}
	id, err := parsePasskeyID(req.Id)
	if err != nil {
		return nil, err
	}
	if err = h.app.Auth.RenamePasskey(ctx, token, id, req.Name); err != nil {
		return nil, passkeyError(err)
	}
	return &authv1.RenamePasskeyResponse{}, nil
}

func (h *authConnectHandler) RemovePasskey(
	ctx context.Context,
	req *authv1.RemovePasskeyRequest,
) (*authv1.RemovePasskeyResponse, error) {
	token, err := h.accessTokenOf(ctx)
	if err != nil {
		return nil, err
	}
	id, err := parsePasskeyID(req.Id)
	if err != nil {
		return nil, err
	}
	if err = h.app.Auth.RemovePasskey(ctx, token, id); err != nil {
		return nil, passkeyError(err)
	}
	return &authv1.RemovePasskeyResponse{}, nil
}
