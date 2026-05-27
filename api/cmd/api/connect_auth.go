package main

import (
	"net/http"
	"time"

	"connectrpc.com/connect"
	"github.com/xdoubleu/essentia/v4/pkg/config"

	"tools.xdoubleu.com/gen/auth/v1/authv1connect"
	"tools.xdoubleu.com/internal/models"
)

const mfaCookieTTL = 5 * time.Minute

const (
	mfaTokenCookieName        = "mfaToken"
	mfaRefreshTokenCookieName = "mfaRefreshToken"
	mfaRememberMeCookieName   = "mfaRememberMe"
	mfaRedirectCookieName     = "mfaRedirect"
	mfaFactorIDCookieName     = "mfaFactorID"
)

type authConnectHandler struct {
	app *Application
}

var _ authv1connect.AuthServiceHandler = (*authConnectHandler)(nil)

func (h *authConnectHandler) parseCookie(
	header http.Header,
	name string,
) (*http.Cookie, error) {
	return (&http.Request{
		Header: http.Header{"Cookie": {header.Get("Cookie")}},
	}).Cookie(name)
}

func (h *authConnectHandler) secure() bool {
	return h.app.config.Env == config.ProdEnv
}

func (h *authConnectHandler) setMFACookies(
	header http.Header,
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
			Name:     mfaTokenCookieName,
			Value:    accessToken,
			MaxAge:   ttl,
			SameSite: http.SameSiteStrictMode,
			HttpOnly: true,
			Secure:   secure,
			Path:     "/",
		},
		//nolint:gosec // Secure is conditionally set based on environment
		{
			Name:     mfaRefreshTokenCookieName,
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

func (h *authConnectHandler) clearMFACookies(header http.Header) {
	secure := h.secure()
	mfaCookieNames := []string{
		mfaTokenCookieName,
		mfaRefreshTokenCookieName,
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
	header http.Header,
	accessToken, refreshToken string,
	rememberMe bool,
) error {
	secure := h.secure()

	accessCookie, err := h.app.services.Auth.CreateCookie(
		models.AccessScope, accessToken, h.app.config.AccessExpiry, secure,
	)
	if err != nil {
		return connect.NewError(connect.CodeInternal, err)
	}
	header.Add("Set-Cookie", accessCookie.String())

	if rememberMe {
		var refreshCookie *http.Cookie
		refreshCookie, err = h.app.services.Auth.CreateCookie(
			models.RefreshScope, refreshToken, h.app.config.RefreshExpiry, secure,
		)
		if err != nil {
			return connect.NewError(connect.CodeInternal, err)
		}
		header.Add("Set-Cookie", refreshCookie.String())
	}

	h.clearMFACookies(header)
	return nil
}
