package services

import (
	"context"
	"errors"
	"html/template"
	"log/slog"
	"net/http"
	"slices"
	"time"

	"github.com/getsentry/sentry-go"
	"github.com/google/uuid"
	"github.com/supabase-community/gotrue-go"
	"github.com/supabase-community/gotrue-go/types"
	"github.com/xdoubleu/essentia/v4/pkg/communication/httptools"
	"github.com/xdoubleu/essentia/v4/pkg/errortools"
	tpltools "github.com/xdoubleu/essentia/v4/pkg/tpl"
	"github.com/xhit/go-str2duration/v2"
	"tools.xdoubleu.com/cmd/publish/internal/dtos"
	"tools.xdoubleu.com/cmd/publish/internal/logging"
	"tools.xdoubleu.com/internal/constants"
	"tools.xdoubleu.com/internal/models"
	"tools.xdoubleu.com/internal/repositories"
)

type AuthService struct {
	client           gotrue.Client
	tpl              *template.Template
	useSecureCookies bool
	accessExpiry     string
	refreshExpiry    string
	appUsersRepo     *repositories.AppUsersRepository
}

func (service *AuthService) GetAllUsers() ([]models.User, error) {
	if service.appUsersRepo != nil {
		return service.appUsersRepo.GetAll(context.Background())
	}
	return []models.User{}, nil
}

func (service *AuthService) SignInWithEmail(
	signInDto *dtos.SignInDto,
) (*string, *string, error) {
	//nolint:exhaustruct //don't need other fields
	response, err := service.client.Token(types.TokenRequest{
		GrantType: "password",
		Email:     signInDto.Email,
		Password:  signInDto.Password,
	})
	if err != nil {
		return nil, nil, errortools.NewUnauthorizedError(
			errors.New("invalid credentials"),
		)
	}

	return &response.AccessToken, &response.RefreshToken, nil
}

func (service *AuthService) GetUser(accessToken string) (*models.User, error) {
	response, err := service.client.WithToken(accessToken).GetUser()
	if err != nil {
		return nil, err
	}

	user := models.UserFromTypesUser(response.User)

	return &user, nil
}

func (service *AuthService) SignInWithRefreshToken(
	refreshToken string,
) (*string, *string, error) {
	//nolint:exhaustruct //don't need other fields
	response, err := service.client.Token(types.TokenRequest{
		GrantType:    "refresh_token",
		RefreshToken: refreshToken,
	})
	if err != nil {
		return nil, nil, err
	}

	return &response.AccessToken, &response.RefreshToken, nil
}

func (service *AuthService) SignOut(
	accessToken string,
	secure bool,
) (*http.Cookie, *http.Cookie, error) {
	err := service.client.WithToken(accessToken).Logout()
	if err != nil {
		return nil, nil, err
	}

	deleteAccessTokenCookie := &http.Cookie{
		Name:     service.GetCookieName(models.AccessScope),
		Value:    "",
		MaxAge:   -1,
		SameSite: http.SameSiteStrictMode,
		HttpOnly: true,
		Secure:   secure,
		Path:     "/",
	}

	deleteRefreshTokenCookie := &http.Cookie{
		Name:     service.GetCookieName(models.RefreshScope),
		Value:    "",
		MaxAge:   -1,
		SameSite: http.SameSiteStrictMode,
		HttpOnly: true,
		Secure:   secure,
		Path:     "/",
	}

	return deleteAccessTokenCookie, deleteRefreshTokenCookie, nil
}

func (service *AuthService) GetCookieName(scope models.Scope) string {
	switch scope {
	case models.AccessScope:
		return "accessToken"
	case models.RefreshScope:
		return "refreshToken"
	default:
		panic("invalid scope")
	}
}

func (service *AuthService) CreateCookie(
	scope models.Scope,
	token string,
	expiry string,
	secure bool,
) (*http.Cookie, error) {
	ttl, err := str2duration.ParseDuration(expiry)
	if err != nil {
		return nil, err
	}

	name := service.GetCookieName(scope)

	cookie := http.Cookie{
		Name:     name,
		Value:    token,
		Expires:  time.Now().Add(ttl),
		SameSite: http.SameSiteStrictMode,
		HttpOnly: true,
		Secure:   secure,
		Path:     "/",
	}

	return &cookie, nil
}

func (service *AuthService) Access(next http.HandlerFunc) http.HandlerFunc {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tokenCookie, err := r.Cookie("accessToken")

		if err != nil {
			httptools.UnauthorizedResponse(w, r,
				errortools.NewUnauthorizedError(errors.New("no token in cookies")))
			return
		}

		user, err := service.GetUser(
			tokenCookie.Value,
		)
		if err != nil {
			httptools.HandleError(w, r, err)
			return
		}

		r = r.WithContext(service.contextSetUser(r.Context(), *user))
		next.ServeHTTP(w, r)
	})
}

func (service *AuthService) TemplateAccess(next http.HandlerFunc) http.HandlerFunc {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		user := service.getCurrentUser(r)

		if user == nil {
			user = service.refreshTokens(w, r)
		}

		if user == nil {
			tpltools.RenderWithPanic(service.tpl, w, "sign-in.html", map[string]any{
				"HideNav":  true,
				"Redirect": r.URL.RequestURI(),
			})
			return
		}

		r = r.WithContext(service.contextSetUser(r.Context(), *user))
		next(w, r)
	})
}

func (service *AuthService) getCurrentUser(r *http.Request) *models.User {
	accessToken, err := r.Cookie("accessToken")
	if err != nil {
		return nil
	}

	user, err := service.GetUser(accessToken.Value)
	if err != nil {
		return nil
	}

	return user
}

func (service *AuthService) refreshTokens(
	w http.ResponseWriter,
	r *http.Request,
) *models.User {
	tokenCookie, err := r.Cookie("refreshToken")

	if err != nil {
		return nil
	}

	accessToken, refreshToken, err := service.SignInWithRefreshToken(
		tokenCookie.Value,
	)
	if err != nil {
		return nil
	}

	accessTokenCookie, err := service.CreateCookie(
		models.AccessScope,
		*accessToken,
		service.accessExpiry,
		service.useSecureCookies,
	)
	if err != nil {
		return nil
	}

	http.SetCookie(w, accessTokenCookie)

	var refreshTokenCookie *http.Cookie
	refreshTokenCookie, err = service.CreateCookie(
		models.RefreshScope,
		*refreshToken,
		service.refreshExpiry,
		service.useSecureCookies,
	)
	if err != nil {
		return nil
	}

	http.SetCookie(w, refreshTokenCookie)

	user, _ := service.GetUser(accessTokenCookie.Value)
	return user
}

func (service *AuthService) contextSetUser(
	ctx context.Context,
	user models.User,
) context.Context {
	if hub := sentry.GetHubFromContext(ctx); hub != nil {
		//nolint:exhaustruct //other fields are optional
		hub.Scope().SetUser(sentry.User{
			ID:    user.ID,
			Email: user.Email,
		})
	}

	if carrier, ok := ctx.Value(logging.CarrierKey).(*logging.UserIDCarrier); ok {
		carrier.ID = user.ID
	}

	ctx = context.WithValue(ctx, logging.UserIDContextKey, user.ID)

	if service.appUsersRepo == nil {
		return context.WithValue(ctx, constants.UserContextKey, user)
	}

	err := service.appUsersRepo.Upsert(ctx, user.ID, user.Email)

	if err != nil {
		slog.Default().ErrorContext(ctx, "failed to upsert app user", "error", err)
		return context.WithValue(ctx, constants.UserContextKey, user)
	}

	var enriched *models.User
	enriched, err = service.appUsersRepo.GetByID(ctx, user.ID)
	if err != nil {
		slog.Default().ErrorContext(ctx, "failed to enrich user from db", "error", err)
		return context.WithValue(ctx, constants.UserContextKey, user)
	}

	if enriched != nil {
		user = *enriched
	}

	return context.WithValue(ctx, constants.UserContextKey, user)
}

func (service *AuthService) AdminAccess(next http.HandlerFunc) http.HandlerFunc {
	return service.TemplateAccess(func(w http.ResponseWriter, r *http.Request) {
		user, ok := r.Context().Value(constants.UserContextKey).(models.User)
		if !ok || user.Role != models.RoleAdmin {
			http.Redirect(w, r, "/", http.StatusSeeOther)
			return
		}
		next(w, r)
	})
}

func (service *AuthService) AppAccess(
	appName string,
	next http.HandlerFunc,
) http.HandlerFunc {
	return service.TemplateAccess(func(w http.ResponseWriter, r *http.Request) {
		user, ok := r.Context().Value(constants.UserContextKey).(models.User)
		if !ok {
			http.Redirect(w, r, "/", http.StatusSeeOther)
			return
		}
		if user.Role == models.RoleAdmin || slices.Contains(user.AppAccess, appName) {
			next(w, r)
			return
		}
		http.Redirect(w, r, "/", http.StatusSeeOther)
	})
}

// HasVerifiedTOTP returns the factor ID of the first verified TOTP factor, or
// (zero, false) when the user has not enrolled MFA yet.
func (service *AuthService) HasVerifiedTOTP(
	accessToken string,
) (uuid.UUID, bool) {
	resp, err := service.client.WithToken(accessToken).GetUser()
	if err != nil {
		return uuid.UUID{}, false
	}
	for _, f := range resp.Factors {
		if f.FactorType == "totp" && f.Status == "verified" {
			return f.ID, true
		}
	}
	return uuid.UUID{}, false
}

// EnrollTOTP begins TOTP enrollment for the given access token and returns the
// QR code SVG, fallback secret, and factor ID.
// Any pre-existing unverified TOTP factor is unenrolled first to avoid the
// friendly-name conflict error Supabase returns on repeated enrollment attempts.
func (service *AuthService) EnrollTOTP(
	accessToken string,
) (*types.EnrollFactorResponse, error) {
	authedClient := service.client.WithToken(accessToken)

	// Clean up any leftover unverified factor from a previous partial enrollment.
	if resp, err := authedClient.GetUser(); err == nil {
		for _, f := range resp.Factors {
			if f.FactorType == "totp" && f.Status == "unverified" {
				_, _ = authedClient.UnenrollFactor(
					types.UnenrollFactorRequest{FactorID: f.ID},
				)
			}
		}
	}

	return authedClient.EnrollFactor(
		//nolint:exhaustruct //issuer and friendlyName are optional
		types.EnrollFactorRequest{
			FactorType: types.FactorTypeTOTP,
		},
	)
}

// ChallengeMFA creates a challenge for the given factor and returns its ID.
func (service *AuthService) ChallengeMFA(
	accessToken string,
	factorID uuid.UUID,
) (*types.ChallengeFactorResponse, error) {
	return service.client.WithToken(accessToken).ChallengeFactor(

		types.ChallengeFactorRequest{FactorID: factorID},
	)
}

// VerifyMFA completes the MFA challenge and returns new aal2 access and refresh tokens.
func (service *AuthService) VerifyMFA(
	accessToken string,
	factorID uuid.UUID,
	challengeID uuid.UUID,
	code string,
) (*string, *string, error) {
	resp, err := service.client.WithToken(accessToken).VerifyFactor(
		types.VerifyFactorRequest{
			FactorID:    factorID,
			ChallengeID: challengeID,
			Code:        code,
		},
	)
	if err != nil {
		return nil, nil, errortools.NewUnauthorizedError(errors.New("invalid MFA code"))
	}
	return &resp.AccessToken, &resp.RefreshToken, nil
}
