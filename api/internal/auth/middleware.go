package auth

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"slices"

	"github.com/getsentry/sentry-go"
	"github.com/xdoubleu/essentia/v4/pkg/communication/httptools"
	"github.com/xdoubleu/essentia/v4/pkg/errortools"

	"tools.xdoubleu.com/internal/constants"
	"tools.xdoubleu.com/internal/models"
)

func (service *GoTrueService) Access(next http.HandlerFunc) http.HandlerFunc {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tokenCookie, err := r.Cookie("accessToken")

		if err != nil {
			httptools.UnauthorizedResponse(w, r,
				errortools.NewUnauthorizedError(errors.New("no token in cookies")))
			return
		}

		user, err := service.resolveUser(
			r.Context(),
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

func (service *GoTrueService) TemplateAccess(
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

func (service *GoTrueService) getCurrentUser(r *http.Request) *models.User {
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

// ResolveToken validates a bearer access token and returns the DB-enriched
// user, reusing the same TTL cache and admin-role enrichment as the cookie
// middleware. It is the entry point for the observability MCP server acting as
// an OAuth resource server: an OAuth-issued Supabase access token resolves
// exactly like the cookie token.
func (service *GoTrueService) ResolveToken(
	ctx context.Context,
	accessToken string,
) (*models.User, error) {
	return service.resolveUser(ctx, accessToken)
}

// resolveUser returns the DB-enriched user for an access token, consulting
// the TTL cache first so repeated requests skip the GoTrue round-trip and
// the enrichment queries.
func (service *GoTrueService) resolveUser(
	ctx context.Context,
	accessToken string,
) (*models.User, error) {
	if cached, ok := service.userCache.get(accessToken); ok {
		return &cached, nil
	}

	user, err := service.GetUser(ctx, accessToken)
	if err != nil {
		return nil, err
	}

	enriched := service.enrichUser(ctx, *user)
	service.userCache.set(accessToken, enriched)
	return &enriched, nil
}

// enrichUser records the user in global.app_users and overlays the DB role
// and app access; on any failure it falls back to the GoTrue user unchanged.
func (service *GoTrueService) enrichUser(
	ctx context.Context,
	user models.User,
) models.User {
	if service.appUsersRepo == nil {
		return user
	}

	if err := service.appUsersRepo.Upsert(ctx, user.ID, user.Email); err != nil {
		slog.Default().ErrorContext(ctx, "failed to upsert app user", "error", err)
		return user
	}

	enriched, err := service.appUsersRepo.GetByID(ctx, user.ID)
	if err != nil {
		slog.Default().ErrorContext(ctx, "failed to enrich user from db", "error", err)
		return user
	}

	if enriched != nil {
		return *enriched
	}
	return user
}

func (service *GoTrueService) refreshTokens(
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

	enriched := service.enrichUser(r.Context(), *user)
	service.userCache.set(accessCookie.Value, enriched)
	return &enriched
}

// contextSetUser stores an already-resolved user on the request context and
// tags the Sentry scope; enrichment happens earlier in resolveUser.
func (service *GoTrueService) contextSetUser(
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

func (service *GoTrueService) AdminAccess(next http.HandlerFunc) http.HandlerFunc {
	return service.TemplateAccess(func(w http.ResponseWriter, r *http.Request) {
		user, ok := r.Context().Value(constants.UserContextKey).(models.User)
		if !ok || user.Role != models.RoleAdmin {
			http.Redirect(w, r, "/", http.StatusSeeOther)
			return
		}
		next(w, r)
	})
}

func (service *GoTrueService) AppAccess(
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
