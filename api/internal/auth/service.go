package auth

import (
	"context"
	"errors"
	"net/http"
	"time"

	gotrue "github.com/supabase-community/auth-go"
	"github.com/supabase-community/auth-go/types"
	essentiaconfig "github.com/xdoubleu/essentia/v4/pkg/config"
	"github.com/xdoubleu/essentia/v4/pkg/errortools"
	"github.com/xhit/go-str2duration/v2"

	"tools.xdoubleu.com/internal/config"
	"tools.xdoubleu.com/internal/models"
	"tools.xdoubleu.com/internal/repositories"
)

// SignInRenderFunc is called by TemplateAccess when the user is not authenticated.
// It receives the redirect URL so the sign-in page can redirect back after login.
type SignInRenderFunc func(w http.ResponseWriter, r *http.Request, redirectURL string)

// GoTrueService is the Supabase GoTrue-backed implementation of Service.
type GoTrueService struct {
	// client's methods do not accept a context.Context
	// (auth-go v1.5.0 has no context support), so request contexts
	// stop propagating at the GoTrue boundary.
	client           gotrue.Client
	useSecureCookies bool
	accessExpiry     string
	refreshExpiry    string
	appUsersRepo     *repositories.AppUsersRepository
	userCache        *userCache
	// SignInRenderer is set by cmd/api after construction to avoid a
	// circular import between this package and package main (which owns the
	// templ-generated SignInPage component).
	SignInRenderer SignInRenderFunc
}

var _ Service = (*GoTrueService)(nil)

func NewService(
	cfg config.Config,
	supabaseClient gotrue.Client,
	appUsersRepo *repositories.AppUsersRepository,
) *GoTrueService {
	return &GoTrueService{
		client:           supabaseClient,
		useSecureCookies: cfg.Env == essentiaconfig.ProdEnv,
		accessExpiry:     cfg.AccessExpiry,
		refreshExpiry:    cfg.RefreshExpiry,
		appUsersRepo:     appUsersRepo,
		userCache: newUserCache(
			time.Duration(cfg.AuthCacheTTL) * time.Second,
		),
		SignInRenderer: nil,
	}
}

// InvalidateUserCache drops every cached user. Call it after mutations that
// change a user's role or app access so other sessions don't serve stale
// permissions for up to the cache TTL.
func (service *GoTrueService) InvalidateUserCache() {
	service.userCache.clear()
}

func (service *GoTrueService) GetAllUsers(
	ctx context.Context,
) ([]models.User, error) {
	if service.appUsersRepo != nil {
		return service.appUsersRepo.GetAll(ctx)
	}
	return []models.User{}, nil
}

func (service *GoTrueService) SignInWithEmail(
	_ context.Context,
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

func (service *GoTrueService) GetUser(
	_ context.Context,
	accessToken string,
) (*models.User, error) {
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

func (service *GoTrueService) SignInWithRefreshToken(
	_ context.Context,
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

// RefreshSession exchanges a refresh token for new tokens and returns the
// signed-in user plus ready-to-set access and refresh cookies. A nil user
// with nil error means the tokens rotated but the user lookup failed;
// callers should still set the cookies and treat the session as absent.
func (service *GoTrueService) RefreshSession(
	ctx context.Context,
	refreshToken string,
) (*models.User, *http.Cookie, *http.Cookie, error) {
	accessToken, newRefreshToken, err := service.SignInWithRefreshToken(
		ctx,
		refreshToken,
	)
	if err != nil {
		return nil, nil, nil, err
	}

	accessCookie, err := service.CreateCookie(
		models.AccessScope,
		*accessToken,
		service.accessExpiry,
		service.useSecureCookies,
	)
	if err != nil {
		return nil, nil, nil, err
	}

	refreshCookie, err := service.CreateCookie(
		models.RefreshScope,
		*newRefreshToken,
		service.refreshExpiry,
		service.useSecureCookies,
	)
	if err != nil {
		return nil, nil, nil, err
	}

	user, _ := service.GetUser(ctx, *accessToken)
	return user, accessCookie, refreshCookie, nil
}

func (service *GoTrueService) SignOut(
	_ context.Context,
	accessToken string,
	secure bool,
) (*http.Cookie, *http.Cookie, error) {
	service.userCache.evict(accessToken)

	err := service.client.WithToken(accessToken).Logout()
	if err != nil {
		return nil, nil, err
	}

	//nolint:gosec // Secure is conditionally set based on environment
	deleteAccessTokenCookie := &http.Cookie{
		Name:     service.GetCookieName(models.AccessScope),
		Value:    "",
		MaxAge:   -1,
		SameSite: http.SameSiteLaxMode,
		HttpOnly: true,
		Secure:   secure,
		Path:     "/",
	}

	//nolint:gosec // Secure is conditionally set based on environment
	deleteRefreshTokenCookie := &http.Cookie{
		Name:     service.GetCookieName(models.RefreshScope),
		Value:    "",
		MaxAge:   -1,
		SameSite: http.SameSiteLaxMode,
		HttpOnly: true,
		Secure:   secure,
		Path:     "/",
	}

	return deleteAccessTokenCookie, deleteRefreshTokenCookie, nil
}

func (service *GoTrueService) GetCookieName(scope models.Scope) string {
	switch scope {
	case models.AccessScope:
		return "accessToken"
	case models.RefreshScope:
		return "refreshToken"
	default:
		panic("invalid scope")
	}
}

func (service *GoTrueService) CreateCookie(
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
		Name:    name,
		Value:   token,
		Expires: time.Now().Add(ttl),
		// Lax (not Strict): Supabase redirects the browser here cross-site
		// during the MCP OAuth consent flow (/oauth/consent), and a Strict
		// cookie would not attach to that top-level GET. These cookies don't
		// gate cross-site CSRF on their own, so Lax is the standard tradeoff.
		SameSite: http.SameSiteLaxMode,
		HttpOnly: true,
		Secure:   secure,
		Path:     "/",
	}

	return &cookie, nil
}

func (service *GoTrueService) ForgotPassword(
	_ context.Context,
	email, redirectTo string,
) error {
	//nolint:exhaustruct //Security is optional
	return service.client.Recover(types.RecoverRequest{
		Email:      email,
		RedirectTo: redirectTo,
	})
}

func (service *GoTrueService) UpdatePassword(
	_ context.Context,
	accessToken, newPassword string,
) error {
	authedClient := service.client.WithToken(accessToken)

	//nolint:exhaustruct //only updating password field
	_, err := authedClient.UpdateUser(
		types.UpdateUserRequest{Password: &newPassword},
	)
	if err != nil {
		return err
	}

	// ponytail: auth-go v1.5.0's Logout has no scope param, so this revokes
	// every refresh token for the user (including the one just used here) —
	// the point of resetting a password is to kick out any other session too.
	// The caller's own JWT access token stays valid until it expires; the
	// cache eviction below stops it being trusted past this request's TTL.
	if err = authedClient.Logout(); err != nil {
		return err
	}

	service.InvalidateUserCache()
	return nil
}
