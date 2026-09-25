package auth

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"slices"

	"github.com/getsentry/sentry-go"

	"tools.xdoubleu.com/internal/communication/httptools"
	"tools.xdoubleu.com/internal/constants"
	"tools.xdoubleu.com/internal/errortools"
	"tools.xdoubleu.com/internal/models"
)

// Access resolves the user like TemplateAccess, refreshing via the refresh
// cookie when the access token has expired.
func (service *LocalService) Access(next http.HandlerFunc) http.HandlerFunc {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		user := service.getCurrentUser(r)
		if user == nil {
			user = service.refreshTokens(w, r)
		}
		if user == nil {
			httptools.UnauthorizedResponse(w, r,
				errortools.NewUnauthorizedError(errors.New("no token in cookies")))
			return
		}

		r = r.WithContext(service.contextSetUser(r.Context(), *user))
		next.ServeHTTP(w, r)
	})
}

func (service *LocalService) TemplateAccess(
	next http.HandlerFunc,
) http.HandlerFunc {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		user := service.getCurrentUser(r)

		if user == nil {
			user = service.refreshTokens(w, r)
		}

		if user == nil {
			if service.SignInRenderer != nil {
				service.SignInRenderer(w, r, r.URL.RequestURI())
			}
			return
		}

		r = r.WithContext(service.contextSetUser(r.Context(), *user))
		next(w, r)
	})
}

func (service *LocalService) getCurrentUser(r *http.Request) *models.User {
	accessToken, err := r.Cookie("accessToken")
	if err != nil {
		return nil
	}

	user, err := service.resolveUser(r.Context(), accessToken.Value)
	if err != nil {
		return nil
	}

	return user
}

// ResolveToken resolves a bearer token to the enriched user for the MCP
// resource server: a local session JWT first, then a fosite opaque token.
func (service *LocalService) ResolveToken(
	ctx context.Context,
	accessToken string,
) (*models.User, error) {
	user, err := service.resolveUser(ctx, accessToken)
	if err == nil {
		return user, nil
	}
	if service.OAuth2TokenResolver == nil {
		return nil, err
	}

	userID, resolveErr := service.OAuth2TokenResolver.ResolveAccessToken(
		ctx,
		accessToken,
	)
	if resolveErr != nil {
		return nil, err
	}

	row, err := service.usersStore.GetUserByID(ctx, userID)
	if err != nil {
		return nil, err
	}
	_, hasMFA := service.HasVerifiedTOTP(ctx, accessToken)
	oauthUser := models.User{
		ID:          row.ID,
		Email:       row.Email,
		Role:        models.RoleUser,
		AppAccess:   []string{},
		HasMFA:      hasMFA,
		DisplayName: "",
	}
	enriched, err := service.enrichUser(ctx, oauthUser)
	if err != nil {
		return nil, err
	}
	return &enriched, nil
}

// resolveUser returns the enriched user for a token, via the TTL cache, with
// concurrent misses for the same token coalesced by resolveGroup.
func (service *LocalService) resolveUser(
	ctx context.Context,
	accessToken string,
) (*models.User, error) {
	if cached, ok := service.userCache.get(accessToken); ok {
		return &cached, nil
	}

	result, err, _ := service.resolveGroup.Do(accessToken, func() (any, error) {
		user, err := service.GetUser(ctx, accessToken)
		if err != nil {
			return nil, err
		}

		enriched, err := service.enrichUser(ctx, *user)
		if err != nil {
			return nil, err
		}
		service.userCache.set(accessToken, enriched)
		return &enriched, nil
	})
	if err != nil {
		return nil, err
	}
	user, ok := result.(*models.User)
	if !ok {
		return nil, errors.New("resolveGroup returned unexpected type")
	}
	return user, nil
}

// enrichUser upserts global.app_users and overlays role and app access. DB
// errors are returned, not swallowed: the unenriched user looks like "no
// access" and would be cached for the full TTL.
func (service *LocalService) enrichUser(
	ctx context.Context,
	user models.User,
) (models.User, error) {
	if service.appUsersRepo == nil {
		return user, nil
	}

	if err := service.appUsersRepo.Upsert(ctx, user.ID, user.Email); err != nil {
		slog.Default().ErrorContext(ctx, "failed to upsert app user", "error", err)
		return user, err
	}

	enriched, err := service.appUsersRepo.GetByID(ctx, user.ID)
	if err != nil {
		slog.Default().ErrorContext(ctx, "failed to enrich user from db", "error", err)
		return user, err
	}

	return *enriched, nil
}

func (service *LocalService) refreshTokens(
	w http.ResponseWriter,
	r *http.Request,
) *models.User {
	tokenCookie, err := r.Cookie("refreshToken")

	if err != nil {
		return nil
	}

	user, accessCookie, refreshCookie, err := service.RefreshSession(
		r.Context(),
		tokenCookie.Value,
	)
	if err != nil {
		return nil
	}

	http.SetCookie(w, accessCookie)
	http.SetCookie(w, refreshCookie)

	if user == nil {
		return nil
	}

	enriched, err := service.enrichUser(r.Context(), *user)
	if err != nil {
		return nil
	}
	service.userCache.set(accessCookie.Value, enriched)
	return &enriched
}

func (service *LocalService) contextSetUser(
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

func (service *LocalService) AdminAccess(next http.HandlerFunc) http.HandlerFunc {
	return service.TemplateAccess(func(w http.ResponseWriter, r *http.Request) {
		user, ok := r.Context().Value(constants.UserContextKey).(models.User)
		if !ok || user.Role != models.RoleAdmin {
			http.Redirect(w, r, "/", http.StatusSeeOther)
			return
		}
		next(w, r)
	})
}

// AppAccess guards only ConnectRPC handlers, so it denies with a plain 403;
// a redirect would be followed silently by fetch().
func (service *LocalService) AppAccess(
	appName string,
	next http.HandlerFunc,
) http.HandlerFunc {
	return service.TemplateAccess(func(w http.ResponseWriter, r *http.Request) {
		user, ok := r.Context().Value(constants.UserContextKey).(models.User)
		if !ok {
			http.Error(w, "forbidden", http.StatusForbidden)
			return
		}
		if user.Role == models.RoleAdmin || slices.Contains(user.AppAccess, appName) {
			next(w, r)
			return
		}
		http.Error(w, "forbidden", http.StatusForbidden)
	})
}
