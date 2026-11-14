package auth

import (
	"context"
	"errors"
	"net/http"
	"strings"

	"tools.xdoubleu.com/internal/kit/errortools"
	"tools.xdoubleu.com/internal/models"
)

// SignInWithOIDC issues a session for the user linked to the provider subject.
// An unknown subject is refused: there is no open sign-up. The provider has
// already authenticated the person, including any second factor, so the
// session is aal2.
func (service *LocalService) SignInWithOIDC(
	ctx context.Context,
	subject string,
) (*string, *string, error) {
	if subject == "" {
		return nil, nil, errOIDCSignIn
	}
	user, err := service.usersStore.GetUserByOIDCSubject(ctx, subject)
	if errors.Is(err, errNotFound) {
		return nil, nil, errortools.NewUnauthorizedError(
			errors.New("no account is linked to this sign-in"),
		)
	}
	if err != nil {
		return nil, nil, err
	}

	accessToken, err := service.mintAccessToken(user.ID, aal2)
	if err != nil {
		return nil, nil, err
	}
	refreshToken, err := service.issueRefreshToken(ctx, user.ID, aal2)
	if err != nil {
		return nil, nil, err
	}
	return &accessToken, &refreshToken, nil
}

// isLocalPath reports whether path is a same-origin path, not a protocol-
// relative or absolute URL.
func isLocalPath(path string) bool {
	return strings.HasPrefix(path, "/") &&
		!strings.HasPrefix(path, "//") && !strings.HasPrefix(path, "/\\")
}

// OIDCLoginHandler starts a sign-in at the provider. ?redirect= is a local
// path to return to afterwards; the web's home by default.
func OIDCLoginHandler(provider *OIDCProvider) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		returnTo := r.URL.Query().Get("redirect")
		if !isLocalPath(returnTo) {
			returnTo = "/"
		}
		authURL, cookie, err := provider.Start(r.Context(), returnTo)
		if err != nil {
			http.Error(w, "sign-in is unavailable", http.StatusBadGateway)
			return
		}
		http.SetCookie(w, cookie)
		//nolint:gosec // authURL is the configured issuer's authorization endpoint
		http.Redirect(w, r, authURL, http.StatusFound)
	}
}

// OIDCCallbackHandler completes a sign-in: it sets the session cookies and
// returns the person to where they were headed. authBase is this api's public
// base URL, for returning to its own /oauth2/ endpoints.
func (service *LocalService) OIDCCallbackHandler(
	provider *OIDCProvider,
	authBase string,
) http.HandlerFunc {
	failed := service.webURL + "/auth/sign-in?error=oidc"
	return func(w http.ResponseWriter, r *http.Request) {
		subject, returnTo, err := provider.Complete(r)
		if err != nil {
			http.Redirect(w, r, failed, http.StatusFound)
			return
		}
		accessToken, refreshToken, err := service.SignInWithOIDC(r.Context(), subject)
		if err != nil {
			http.Redirect(w, r, failed, http.StatusFound)
			return
		}
		secure := service.useSecureCookies
		access, err := service.CreateCookie(
			models.AccessScope, *accessToken, service.accessExpiry, secure,
		)
		if err != nil {
			http.Error(w, "sign-in failed", http.StatusInternalServerError)
			return
		}
		refresh, err := service.CreateCookie(
			models.RefreshScope, *refreshToken, service.refreshExpiry, secure,
		)
		if err != nil {
			http.Error(w, "sign-in failed", http.StatusInternalServerError)
			return
		}
		http.SetCookie(w, access)
		http.SetCookie(w, refresh)
		// The state cookie is single-use.
		//nolint:gosec // Secure follows the environment
		http.SetCookie(w, &http.Cookie{
			Name: oidcStateCookie, MaxAge: -1, Path: "/",
			HttpOnly: true, Secure: secure, SameSite: http.SameSiteLaxMode,
		})

		target := service.webURL + returnTo
		if strings.HasPrefix(returnTo, authorizePrefix) {
			target = strings.TrimSuffix(authBase, "/") + returnTo
		}
		http.Redirect(w, r, target, http.StatusFound)
	}
}
