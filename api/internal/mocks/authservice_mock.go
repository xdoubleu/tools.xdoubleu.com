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
		userID:  userID,
		isAdmin: false,
	}
}

func NewMockedAdminAuthService(userID string) auth.Service {
	return &MockedAuthService{
		userID:  userID,
		isAdmin: true,
	}
}

type MockedAuthService struct {
	userID  string
	isAdmin bool
}

func (m *MockedAuthService) mockUser() models.User {
	role := models.RoleUser
	if m.isAdmin {
		role = models.RoleAdmin
	}
	return models.User{
		ID:        m.userID,
		Email:     "user@example.com",
		Role:      role,
		AppAccess: []string{"backlog", "watchparty", "icsproxy", "recipes"},
	}
}

func (m *MockedAuthService) Access(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := context.WithValue(r.Context(), constants.UserContextKey, m.mockUser())
		next(w, r.WithContext(ctx))
	}
}

func (m *MockedAuthService) TemplateAccess(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := context.WithValue(r.Context(), constants.UserContextKey, m.mockUser())
		next(w, r.WithContext(ctx))
	}
}

func (m *MockedAuthService) AdminAccess(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !m.isAdmin {
			http.Redirect(w, r, "/", http.StatusSeeOther)
			return
		}
		ctx := context.WithValue(r.Context(), constants.UserContextKey, m.mockUser())
		next(w, r.WithContext(ctx))
	}
}

func (m *MockedAuthService) AppAccess(
	_ string,
	next http.HandlerFunc,
) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := context.WithValue(r.Context(), constants.UserContextKey, m.mockUser())
		next(w, r.WithContext(ctx))
	}
}

func (m *MockedAuthService) GetAllUsers() ([]models.User, error) {
	return []models.User{m.mockUser()}, nil
}

func (m *MockedAuthService) SignOut(
	_ string,
	_ bool,
) (*http.Cookie, *http.Cookie, error) {
	return nil, nil, nil
}
