package oauth2as

import (
	"context"
	"log/slog"
	"net/http"

	"github.com/ory/fosite"
)

// Endpoint labels for the `endpoint` log attribute — the log record's own
// identification of which /oauth2/* leg rejected the request, independent of
// whatever path the mux happened to route.
const (
	endpointAuthorize = "/oauth2/authorize"
	endpointToken     = "/oauth2/token" //nolint:gosec // label, not a credential
	endpointRegister  = "/oauth2/register"
)

// The two grant types this server registers every client for.
// refreshTokenGrant is the one whose rejection always means a client that
// previously held a working session just lost it — see oauthErrorLevel.
const (
	authorizationCodeGrant = "authorization_code"
	refreshTokenGrant      = "refresh_token"
)

// logOAuthError records a rejection from /oauth2/authorize or /oauth2/token.
// fosite writes its errors straight into the HTTP response and never through
// slog, so without this nothing server-side distinguishes a failed token
// exchange from a successful one: issue #1177 (no refresh token issued, every
// client forced through an interactive re-auth once an hour) was invisible in
// production until a user reported it, because `global.usage_daily` counts
// oauth2/token hits regardless of outcome.
//
// requester is whatever fosite managed to parse before failing, and is nil
// when it failed before parsing anything at all.
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

	// DebugField is deliberately not logged: fosite puts raw internals there,
	// which is the one field that could carry a credential out of the request
	// form. HintField is server-authored and is what actually names the cause
	// ("...was not granted scope offline_access"), so it earns its place.
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

// oauthErrorLevel decides whether a rejection is worth a Sentry event.
// Error-level records are what the root sentrytools module's LogHandler
// forwards, so this is the whole alerting policy in one place:
//
//   - 5xx is always a server fault.
//   - A failed refresh_token grant is Error even though it's a 400. Unlike
//     every other 4xx here it can't be caused by a stranger hitting the
//     endpoint — it takes a real refresh token this server itself issued, so
//     invalid_grant/invalid_scope here means a working client just lost its
//     session. That is exactly the #1177 failure mode and the one signal
//     worth alerting on.
//   - Everything else 4xx is routine: an expired authorization code, a
//     mistyped PKCE verifier, a denied consent, or the steady background of
//     internet scanners probing /oauth2/*. Alerting on those would bury the
//     case above in noise.
func oauthErrorLevel(grantType string, rfcErr *fosite.RFC6749Error) slog.Level {
	if rfcErr.CodeField >= http.StatusInternalServerError {
		return slog.LevelError
	}
	if grantType == refreshTokenGrant {
		return slog.LevelError
	}
	return slog.LevelWarn
}

// requesterIdentity pulls the two non-secret identifying fields — grant type
// and client id — off a requester that may be nil, or may have failed before
// its client was resolved. Nothing else from the request is safe to log: the
// token endpoint's form carries the authorization code, the PKCE verifier and
// the refresh token itself.
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

// logRegisterError records a rejected RFC 7591 dynamic client registration.
// These never reach fosite, so they carry a plain error and an explicit
// status rather than an RFC6749Error — always a 4xx (malformed body, or a
// redirect_uri validateRedirectURI refused), so always Warn.
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
