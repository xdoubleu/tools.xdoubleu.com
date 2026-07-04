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

	user, err := service.GetUser(accessToken.Value)
	if err != nil {
		return nil
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
		tokenCookie.Value,
	)
	if err != nil {
		return nil
	}

	http.SetCookie(w, accessCookie)
	http.SetCookie(w, refreshCookie)

	return user
}

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
