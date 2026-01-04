package mocks

import (
	"context"
	"net/http"

	"tools.xdoubleu.com/internal/auth"
	"tools.xdoubleu.com/internal/constants"
	"tools.xdoubleu.com/internal/models"
)

func NewMockedAuthService(userID string) auth.Service {
	return &MockedAuthService{
		userID: userID,
	}
}

type MockedAuthService struct {
	userID string
}

func (m *MockedAuthService) Access(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Inject a mock user into the context
		user := models.User{
			ID:    m.userID,
			Email: "<EMAIL>",
		}

		ctx := context.WithValue(r.Context(), constants.UserContextKey, user)
		r = r.WithContext(ctx)

		next(w, r)
	}
}

func (m *MockedAuthService) TemplateAccess(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Inject a mock user into the context
		user := models.User{
			ID:    m.userID,
			Email: "<EMAIL>",
		}

		ctx := context.WithValue(r.Context(), constants.UserContextKey, user)
		r = r.WithContext(ctx)

		next(w, r)
	}
}

func (m *MockedAuthService) GetAllUsers() ([]models.User, error) {
	return []models.User{}, nil
}

func (m *MockedAuthService) SignOut(
	_ string,
	_ bool,
) (*http.Cookie, *http.Cookie, error) {
	return nil, nil, nil
}
