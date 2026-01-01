package services

import (
	"context"
	"errors"
	"html/template"
	"net/http"
	"time"

	httptools "github.com/XDoubleU/essentia/pkg/communication/http"
	errortools "github.com/XDoubleU/essentia/pkg/errors"
	tpltools "github.com/XDoubleU/essentia/pkg/tpl"
	"github.com/getsentry/sentry-go"
	"github.com/supabase-community/gotrue-go"
	"github.com/supabase-community/gotrue-go/types"
	"github.com/xhit/go-str2duration/v2"
	"tools.xdoubleu.com/cmd/publish/internal/dtos"
	"tools.xdoubleu.com/internal/constants"
	"tools.xdoubleu.com/internal/models"
)

type AuthService struct {
	supabaseUserID   string
	client           gotrue.Client
	tpl              *template.Template
	useSecureCookies bool
	accessExpiry     string
	refreshExpiry    string
}

func (service *AuthService) GetAllUsers() ([]models.User, error) {
	//nolint:exhaustruct //skip
	return []models.User{
		{
			ID: service.supabaseUserID,
		},
	}, nil
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
		Path:     "/",
	}

	deleteRefreshTokenCookie := &http.Cookie{
		Name:     service.GetCookieName(models.RefreshScope),
		Value:    "",
		MaxAge:   -1,
		SameSite: http.SameSiteStrictMode,
		HttpOnly: true,
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
			tpltools.RenderWithPanic(service.tpl, w, "sign-in.html", nil)
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

	return context.WithValue(ctx, constants.UserContextKey, user)
}
