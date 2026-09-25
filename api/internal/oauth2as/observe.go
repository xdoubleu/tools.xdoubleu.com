package oauth2as

import (
	"context"
	"errors"
	"log/slog"
	"net/http"

	"github.com/ory/fosite"
)

// Endpoint labels for the `endpoint` log attribute.
const (
	endpointAuthorize = "/oauth2/authorize"
	endpointToken     = "/oauth2/token" //nolint:gosec // label, not a credential
	endpointRegister  = "/oauth2/register"
)

// Grant types every client is registered for.
const (
	authorizationCodeGrant = "authorization_code"
	refreshTokenGrant      = "refresh_token"
)

// logOAuthError records an /oauth2/authorize or /oauth2/token rejection;
// fosite writes errors only to the response, never to slog. requester is nil
// when fosite failed before parsing anything.
func logOAuthError(
	ctx context.Context,
	logger *slog.Logger,
	endpoint string,
	requester fosite.Requester,
	err error,
) {
	if logger == nil {
		return
	}

	rfcErr := fosite.ErrorToRFC6749Error(err)
	grantType, clientID := requesterIdentity(requester)

	// DebugField may carry raw request internals (credentials), so never log it.
	// HintField is server-authored and names the cause.
	logger.Log(ctx, oauthErrorLevel(grantType, rfcErr),
		"oauth2 request rejected",
		slog.String("endpoint", endpoint),
		slog.String("grant_type", grantType),
		slog.String("client_id", clientID),
		slog.String("oauth_error", rfcErr.ErrorField),
		slog.String("oauth_error_description", rfcErr.DescriptionField),
		slog.String("oauth_error_hint", rfcErr.HintField),
		slog.Int("status", rfcErr.CodeField),
		slog.Any("error", err),
	)
}

// oauthErrorLevel is the alerting policy (Error is forwarded to Sentry):
// - 5xx: Error.
// - refresh_token reuse past the grace period: Error, since fosite revokes
// the whole token family.
// - every other 4xx (expired/unknown tokens, PKCE mistakes, denied consent,
// scanners): Warn.
func oauthErrorLevel(grantType string, rfcErr *fosite.RFC6749Error) slog.Level {
	if rfcErr.CodeField >= http.StatusInternalServerError {
		return slog.LevelError
	}
	if grantType == refreshTokenGrant && refreshTokenReuseDetected(rfcErr) {
		return slog.LevelError
	}
	return slog.LevelWarn
}

// refreshTokenReuseDetected reports fosite's theft response to a rotated-out
// refresh token replayed past refreshTokenReuseGracePeriod, which wraps
// fosite.ErrInactiveToken as the cause (flow_refresh.go). errors.Is walks
// fosite's wrapping.
func refreshTokenReuseDetected(rfcErr *fosite.RFC6749Error) bool {
	return errors.Is(rfcErr, fosite.ErrInactiveToken)
}

// requesterIdentity returns grant type and client id from a possibly nil
// requester. Nothing else is safe to log: the form carries codes, PKCE
// verifiers and refresh tokens.
func requesterIdentity(requester fosite.Requester) (string, string) {
	if requester == nil {
		return "", ""
	}

	clientID := ""
	if client := requester.GetClient(); client != nil {
		clientID = client.GetID()
	}
	return requester.GetRequestForm().Get("grant_type"), clientID
}

// logRegisterError logs a rejected RFC 7591 registration; always a 4xx, so
// always Warn.
func logRegisterError(
	ctx context.Context,
	logger *slog.Logger,
	err error,
) {
	if logger == nil {
		return
	}
	logger.WarnContext(ctx, "oauth2 client registration rejected",
		slog.String("endpoint", endpointRegister),
		slog.Int("status", http.StatusBadRequest),
		slog.Any("error", err),
	)
}
