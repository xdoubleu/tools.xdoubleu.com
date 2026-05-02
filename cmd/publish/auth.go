package main

import (
	"fmt"
	"html/template"
	"net/http"
	"time"

	"github.com/google/uuid"
	httptools "github.com/xdoubleu/essentia/v4/pkg/communication/httptools"
	"github.com/xdoubleu/essentia/v4/pkg/config"
	tpltools "github.com/xdoubleu/essentia/v4/pkg/tpl"
	"tools.xdoubleu.com/cmd/publish/internal/dtos"
	"tools.xdoubleu.com/internal/models"
	"tools.xdoubleu.com/internal/templates"
)

const (
	mfaCookieTTL = 5 * time.Minute
	maxBodyBytes = 1 << 20 // 1 MB
)

func (app *Application) authRoutes(prefix string, mux *http.ServeMux) {
	mux.HandleFunc(fmt.Sprintf("POST /%s/signin", prefix), app.signInHandler)
	mux.HandleFunc(
		fmt.Sprintf("GET /%s/signout", prefix),
		app.services.Auth.Access(app.signOutHandler),
	)
	mux.HandleFunc(fmt.Sprintf("GET /%s/mfa/enroll", prefix), app.mfaEnrollGetHandler)
	mux.HandleFunc(fmt.Sprintf("POST /%s/mfa/enroll", prefix), app.mfaEnrollPostHandler)
	mux.HandleFunc(
		fmt.Sprintf("GET /%s/mfa/challenge", prefix),
		app.mfaChallengeGetHandler,
	)
	mux.HandleFunc(
		fmt.Sprintf("POST /%s/mfa/challenge", prefix),
		app.mfaChallengePostHandler,
	)
}

func (app *Application) signInHandler(w http.ResponseWriter, r *http.Request) {
	var signInDto dtos.SignInDto

	r.Body = http.MaxBytesReader(w, r.Body, maxBodyBytes)
	err := httptools.ReadForm(r, &signInDto)
	if err != nil {
		httptools.RedirectWithError(w, r, "/", err)
		return
	}

	if ok, errs := signInDto.Validate(); !ok {
		httptools.FailedValidationResponse(w, r, errs)
		return
	}

	accessToken, _, err := app.services.Auth.SignInWithEmail(&signInDto)
	if err != nil {
		httptools.RedirectWithError(w, r, "/", err)
		return
	}

	secure := app.config.Env == config.ProdEnv

	// Store the aal1 token in a short-lived cookie for the MFA step.
	app.setMFATokenCookie(
		w,
		*accessToken,
		signInDto.RememberMe,
		signInDto.Redirect,
		secure,
	)

	factorID, hasMFA := app.services.Auth.HasVerifiedTOTP(*accessToken)
	if !hasMFA {
		http.Redirect(w, r, "/auth/mfa/enroll", http.StatusSeeOther)
		return
	}

	http.SetCookie(w, &http.Cookie{
		Name:     "mfaFactorID",
		Value:    factorID.String(),
		MaxAge:   int(mfaCookieTTL.Seconds()),
		SameSite: http.SameSiteStrictMode,
		HttpOnly: true,
		Secure:   secure,
		Path:     "/",
	})

	http.Redirect(w, r, "/auth/mfa/challenge", http.StatusSeeOther)
}

func (app *Application) setMFATokenCookie(
	w http.ResponseWriter,
	accessToken string,
	rememberMe bool,
	redirect string,
	secure bool,
) {
	http.SetCookie(w, &http.Cookie{
		Name:     "mfaToken",
		Value:    accessToken,
		MaxAge:   int(mfaCookieTTL.Seconds()),
		SameSite: http.SameSiteStrictMode,
		HttpOnly: true,
		Secure:   secure,
		Path:     "/",
	})
	rememberVal := "0"
	if rememberMe {
		rememberVal = "1"
	}
	http.SetCookie(w, &http.Cookie{
		Name:     "mfaRememberMe",
		Value:    rememberVal,
		MaxAge:   int(mfaCookieTTL.Seconds()),
		SameSite: http.SameSiteStrictMode,
		HttpOnly: true,
		Secure:   secure,
		Path:     "/",
	})
	http.SetCookie(w, &http.Cookie{
		Name:     "mfaRedirect",
		Value:    redirect,
		MaxAge:   int(mfaCookieTTL.Seconds()),
		SameSite: http.SameSiteStrictMode,
		HttpOnly: true,
		Secure:   secure,
		Path:     "/",
	})
}

func (app *Application) clearMFACookies(w http.ResponseWriter) {
	secure := app.config.Env == config.ProdEnv
	mfaCookieNames := []string{
		"mfaToken",
		"mfaFactorID",
		"mfaRememberMe",
		"mfaRedirect",
	}
	for _, name := range mfaCookieNames {
		http.SetCookie(
			w,
			&http.Cookie{
				Name:     name,
				Value:    "",
				MaxAge:   -1,
				Secure:   secure,
				HttpOnly: true,
				Path:     "/",
			},
		)
	}
}

// completeMFA issues real session cookies after successful MFA verification and
// clears the temporary MFA cookies.
func (app *Application) completeMFA(
	w http.ResponseWriter,
	r *http.Request,
	accessToken, refreshToken string,
	rememberMe bool,
	redirect string,
) {
	secure := app.config.Env == config.ProdEnv

	accessCookie, err := app.services.Auth.CreateCookie(
		models.AccessScope, accessToken, app.config.AccessExpiry, secure,
	)
	if err != nil {
		templates.RenderError(
			app.tpl,
			w,
			http.StatusInternalServerError,
			"Failed to create session",
		)
		return
	}
	http.SetCookie(w, accessCookie)

	if rememberMe {
		refreshCookie, refErr := app.services.Auth.CreateCookie(
			models.RefreshScope, refreshToken, app.config.RefreshExpiry, secure,
		)
		if refErr != nil {
			templates.RenderError(
				app.tpl,
				w,
				http.StatusInternalServerError,
				"Failed to create session",
			)
			return
		}
		http.SetCookie(w, refreshCookie)
	}

	app.clearMFACookies(w)
	http.Redirect(w, r, redirect, http.StatusSeeOther)
}

// mfaEnrollGetHandler shows the TOTP QR code for first-time enrollment.
func (app *Application) mfaEnrollGetHandler(w http.ResponseWriter, r *http.Request) {
	mfaToken, err := r.Cookie("mfaToken")
	if err != nil {
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}

	enroll, err := app.services.Auth.EnrollTOTP(mfaToken.Value)
	if err != nil {
		httptools.RedirectWithError(w, r, "/", err)
		return
	}

	//nolint:gosec //SVG originates from Supabase, not user input
	qrCode := template.HTML(enroll.TOTP.QRCode)
	tpltools.RenderWithPanic(app.tpl, w, "mfa-enroll.html", map[string]any{
		"HideNav":  true,
		"QRCode":   qrCode,
		"Secret":   enroll.TOTP.Secret,
		"FactorID": enroll.ID.String(),
	})
}

// mfaEnrollPostHandler verifies the first TOTP code and completes enrollment.
func (app *Application) mfaEnrollPostHandler(w http.ResponseWriter, r *http.Request) {
	mfaToken, err := r.Cookie("mfaToken")
	if err != nil {
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, maxBodyBytes)

	if err = r.ParseForm(); err != nil {
		httptools.RedirectWithError(w, r, "/auth/mfa/enroll", err)
		return
	}

	factorIDStr := r.FormValue("factor_id")
	code := r.FormValue("code")

	factorID, err := uuid.Parse(factorIDStr)
	if err != nil {
		http.Redirect(w, r, "/auth/mfa/enroll", http.StatusSeeOther)
		return
	}

	challenge, err := app.services.Auth.ChallengeMFA(mfaToken.Value, factorID)
	if err != nil {
		httptools.RedirectWithError(w, r, "/auth/mfa/enroll", err)
		return
	}

	accessToken, refreshToken, err := app.services.Auth.VerifyMFA(
		mfaToken.Value, factorID, challenge.ID, code,
	)
	if err != nil {
		httptools.RedirectWithError(w, r, "/auth/mfa/enroll", err)
		return
	}

	rememberMe := false
	if c, cErr := r.Cookie("mfaRememberMe"); cErr == nil {
		rememberMe = c.Value == "1"
	}
	redirect := "/"
	if c, cErr := r.Cookie("mfaRedirect"); cErr == nil && c.Value != "" {
		redirect = c.Value
	}

	app.completeMFA(w, r, *accessToken, *refreshToken, rememberMe, redirect)
}

// mfaChallengeGetHandler shows the TOTP code entry form.
func (app *Application) mfaChallengeGetHandler(w http.ResponseWriter, r *http.Request) {
	if _, err := r.Cookie("mfaToken"); err != nil {
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}

	tpltools.RenderWithPanic(app.tpl, w, "mfa-challenge.html", map[string]any{
		"HideNav": true,
	})
}

// mfaChallengePostHandler verifies the TOTP code and issues session cookies.
func (app *Application) mfaChallengePostHandler(
	w http.ResponseWriter,
	r *http.Request,
) {
	mfaToken, err := r.Cookie("mfaToken")
	if err != nil {
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}
	mfaFactorID, err := r.Cookie("mfaFactorID")
	if err != nil {
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, maxBodyBytes)

	if err = r.ParseForm(); err != nil {
		httptools.RedirectWithError(w, r, "/auth/mfa/challenge", err)
		return
	}

	code := r.FormValue("code")

	factorID, err := uuid.Parse(mfaFactorID.Value)
	if err != nil {
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}

	challenge, err := app.services.Auth.ChallengeMFA(mfaToken.Value, factorID)
	if err != nil {
		httptools.RedirectWithError(w, r, "/auth/mfa/challenge", err)
		return
	}

	accessToken, refreshToken, err := app.services.Auth.VerifyMFA(
		mfaToken.Value, factorID, challenge.ID, code,
	)
	if err != nil {
		httptools.RedirectWithError(w, r, "/auth/mfa/challenge", err)
		return
	}

	rememberMe := false
	if c, cErr := r.Cookie("mfaRememberMe"); cErr == nil {
		rememberMe = c.Value == "1"
	}
	redirect := "/"
	if c, cErr := r.Cookie("mfaRedirect"); cErr == nil && c.Value != "" {
		redirect = c.Value
	}

	app.completeMFA(w, r, *accessToken, *refreshToken, rememberMe, redirect)
}

func (app *Application) signOutHandler(w http.ResponseWriter, r *http.Request) {
	accessToken, _ := r.Cookie("accessToken")
	refreshToken, _ := r.Cookie("refreshToken")

	secure := app.config.Env == config.ProdEnv
	deleteAccessTokenCookie, deleteRefreshTokenCookie, err := app.services.Auth.SignOut(
		accessToken.Value,
		secure,
	)
	if err != nil {
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}

	http.SetCookie(w, deleteAccessTokenCookie)

	if refreshToken != nil {
		http.SetCookie(w, deleteRefreshTokenCookie)
	}

	http.Redirect(w, r, "/", http.StatusSeeOther)
}
