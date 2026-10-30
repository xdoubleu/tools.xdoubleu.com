package account

import (
	"errors"
	"net/http"
	"time"

	"connectrpc.com/connect/v2"
	"github.com/google/uuid"

	"tools.xdoubleu.com/gen/auth/v1/authv1connect"
	"tools.xdoubleu.com/internal/auth"
	"tools.xdoubleu.com/internal/config"
	"tools.xdoubleu.com/internal/errortools"
	"tools.xdoubleu.com/internal/models"
)

const mfaCookieTTL = 5 * time.Minute

const (
	MfaTokenCookieName        = "mfaToken"
	MfaRefreshTokenCookieName = "mfaRefreshToken"
	mfaRememberMeCookieName   = "mfaRememberMe"
	mfaRedirectCookieName     = "mfaRedirect"
	mfaFactorIDCookieName     = "mfaFactorID"
)

type authConnectHandler struct {
	app *Deps
}

var _ authv1connect.AuthServiceHandler = (*authConnectHandler)(nil)

func (h *authConnectHandler) parseCookie(
	header *connect.Header,
	name string,
) (*http.Cookie, error) {
	return (&http.Request{
		Header: http.Header{"Cookie": {header.Get("Cookie")}},
	}).Cookie(name)
}

// AuthErrorCode maps auth rejections to their Connect codes and anything
// else to Internal.
func AuthErrorCode(err error) connect.Code {
	var unauthorized errortools.UnauthorizedError
	switch {
	case errors.As(err, &unauthorized):
		return connect.CodeUnauthenticated
	case errors.Is(err, auth.ErrWrongCredential):
		return connect.CodePermissionDenied
	case errors.Is(err, auth.ErrPasswordTooShort):
		return connect.CodeInvalidArgument
	case errors.Is(err, auth.ErrTooManyAttempts):
		return connect.CodeResourceExhausted
	default:
		return connect.CodeInternal
	}
}

// SignInErrorCode maps a failed sign-in or MFA check: Unauthenticated,
// except ResourceExhausted during a lockout.
func SignInErrorCode(err error) connect.Code {
	if errors.Is(err, auth.ErrTooManyAttempts) {
		return connect.CodeResourceExhausted
	}
	return connect.CodeUnauthenticated
}

func (h *authConnectHandler) secure() bool {
	return h.app.Config.Env == config.ProdEnv
}

func (h *authConnectHandler) setMFACookies(
	header *connect.Header,
	accessToken string,
	refreshToken string,
	rememberMe bool,
	redirect string,
) {
	secure := h.secure()
	rememberVal := "0"
	if rememberMe {
		rememberVal = "1"
	}
	ttl := int(mfaCookieTTL.Seconds())
	for _, c := range []*http.Cookie{
		//nolint:gosec // Secure is conditionally set based on environment
		{
			Name:     MfaTokenCookieName,
			Value:    accessToken,
			MaxAge:   ttl,
			SameSite: http.SameSiteStrictMode,
			HttpOnly: true,
			Secure:   secure,
			Path:     "/",
		},
		//nolint:gosec // Secure is conditionally set based on environment
		{
			Name:     MfaRefreshTokenCookieName,
			Value:    refreshToken,
			MaxAge:   ttl,
			SameSite: http.SameSiteStrictMode,
			HttpOnly: true,
			Secure:   secure,
			Path:     "/",
		},
		//nolint:gosec // Secure is conditionally set based on environment
		{
			Name:     mfaRememberMeCookieName,
			Value:    rememberVal,
			MaxAge:   ttl,
			SameSite: http.SameSiteStrictMode,
			HttpOnly: true,
			Secure:   secure,
			Path:     "/",
		},
		//nolint:gosec // Secure is conditionally set based on environment
		{
			Name:     mfaRedirectCookieName,
			Value:    redirect,
			MaxAge:   ttl,
			SameSite: http.SameSiteStrictMode,
			HttpOnly: true,
			Secure:   secure,
			Path:     "/",
		},
	} {
		header.Add("Set-Cookie", c.String())
	}
}

func (h *authConnectHandler) setMFAFactorIDCookie(
	header *connect.Header,
	factorID uuid.UUID,
) {
	//nolint:gosec // Secure is conditionally set based on environment
	header.Add("Set-Cookie", (&http.Cookie{
		Name:     mfaFactorIDCookieName,
		Value:    factorID.String(),
		MaxAge:   int(mfaCookieTTL.Seconds()),
		SameSite: http.SameSiteStrictMode,
		HttpOnly: true,
		Secure:   h.secure(),
		Path:     "/",
	}).String())
}

func (h *authConnectHandler) clearMFACookies(header *connect.Header) {
	secure := h.secure()
	mfaCookieNames := []string{
		MfaTokenCookieName,
		MfaRefreshTokenCookieName,
		mfaFactorIDCookieName,
		mfaRememberMeCookieName,
		mfaRedirectCookieName,
	}
	for _, name := range mfaCookieNames {
		//nolint:gosec // Secure is conditionally set based on environment
		c := &http.Cookie{
			Name:     name,
			Value:    "",
			MaxAge:   -1,
			SameSite: http.SameSiteStrictMode,
			HttpOnly: true,
			Secure:   secure,
			Path:     "/",
		}
		header.Add("Set-Cookie", c.String())
	}
}

func (h *authConnectHandler) completeMFA(
	header *connect.Header,
	accessToken, refreshToken string,
	rememberMe bool,
) error {
	secure := h.secure()

	accessCookie, err := h.app.Auth.CreateCookie(
		models.AccessScope, accessToken, h.app.Config.AccessExpiry, secure,
	)
	if err != nil {
		return connect.NewError(connect.CodeInternal, err.Error()).WithCause(err)
	}
	header.Add("Set-Cookie", accessCookie.String())

	if rememberMe {
		var refreshCookie *http.Cookie
		refreshCookie, err = h.app.Auth.CreateCookie(
			models.RefreshScope, refreshToken, h.app.Config.RefreshExpiry, secure,
		)
		if err != nil {
			return connect.NewError(connect.CodeInternal, err.Error()).WithCause(err)
		}
		header.Add("Set-Cookie", refreshCookie.String())
	}

	h.clearMFACookies(header)
	return nil
}
