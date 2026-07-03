package services

import (
	"context"
	"errors"
	"net/http"
	"time"

	auth "github.com/supabase-community/auth-go"
	"github.com/supabase-community/auth-go/types"
	"github.com/xdoubleu/essentia/v4/pkg/errortools"
	"github.com/xhit/go-str2duration/v2"

	"tools.xdoubleu.com/internal/models"
	"tools.xdoubleu.com/internal/repositories"
)

// SignInRenderFunc is called by TemplateAccess when the user is not authenticated.
// It receives the redirect URL so the sign-in page can redirect back after login.
type SignInRenderFunc func(w http.ResponseWriter, r *http.Request, redirectURL string)

type AuthService struct {
	client           auth.Client
	useSecureCookies bool
	accessExpiry     string
	refreshExpiry    string
	appUsersRepo     *repositories.AppUsersRepository
	// SignInRenderer is set by cmd/api after construction to avoid a
	// circular import between this package and package main (which owns the
	// templ-generated SignInPage component).
	SignInRenderer SignInRenderFunc
}

func (service *AuthService) GetAllUsers() ([]models.User, error) {
	if service.appUsersRepo != nil {
		return service.appUsersRepo.GetAll(context.Background())
	}
	return []models.User{}, nil
}

func (service *AuthService) SignInWithEmail(
	email, password string,
) (*string, *string, error) {
	//nolint:exhaustruct //don't need other fields
	response, err := service.client.Token(types.TokenRequest{
		GrantType: "password",
		Email:     email,
		Password:  password,
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
	if response == nil {
		return nil, errors.New("user not found")
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

	//nolint:gosec // Secure is conditionally set based on environment
	deleteAccessTokenCookie := &http.Cookie{
		Name:     service.GetCookieName(models.AccessScope),
		Value:    "",
		MaxAge:   -1,
		SameSite: http.SameSiteStrictMode,
		HttpOnly: true,
		Secure:   secure,
		Path:     "/",
	}

	//nolint:gosec // Secure is conditionally set based on environment
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

	//nolint:gosec // Secure is conditionally set based on environment
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

func (service *AuthService) ForgotPassword(email, redirectTo string) error {
	//nolint:exhaustruct //Security is optional
	return service.client.Recover(types.RecoverRequest{
		Email:      email,
		RedirectTo: redirectTo,
	})
}

func (service *AuthService) UpdatePassword(
	accessToken, newPassword string,
) error {
	//nolint:exhaustruct //only updating password field
	_, err := service.client.WithToken(accessToken).UpdateUser(
		types.UpdateUserRequest{Password: &newPassword},
	)
	return err
}
