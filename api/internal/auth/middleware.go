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
// the enrichment queries. Cache misses for the same token are coalesced via
// resolveGroup so concurrent requests (e.g. several tabs opened at once)
// share one GoTrue round trip instead of each firing their own.
func (service *GoTrueService) resolveUser(
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

// enrichUser records the user in global.app_users and overlays the DB role
// and app access. A DB failure is returned rather than swallowed: the
// unenriched GoTrue user always carries Role: RoleUser and no AppAccess (see
// models.UserFromTypesUser), so silently falling back to it would look
// indistinguishable from "this user genuinely has no access" to callers like
// AdminAccess/AppAccess — and resolveUser/refreshTokens would then cache that
// wrong identity for the full TTL instead of retrying on the next request.
func (service *GoTrueService) enrichUser(
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

	enriched, err := service.enrichUser(r.Context(), *user)
	if err != nil {
		return nil
	}
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

// AppAccess only ever guards ConnectRPC service handlers (see every
// apps/*/routes.go call site), never an HTML page, so a denial responds with
// a plain 403 rather than AdminAccess's redirect: a fetch()-based RPC client
// follows a 30x transparently to "/" and fails opaquely instead of surfacing
// a clean error.
func (service *GoTrueService) AppAccess(
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
