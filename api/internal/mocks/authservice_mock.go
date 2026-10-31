package mocks

import (
	"context"
	"net/http"

	"tools.xdoubleu.com/internal/auth"
	"tools.xdoubleu.com/internal/constants"
	"tools.xdoubleu.com/internal/models"
)

const mockUserEmail = "user@example.com"

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

// NewMockedChannelAuthService acts as a user authenticated through channel
// (a constants.Channel* value); the other constructors act as a browser.
func NewMockedChannelAuthService(
	userID string,
	isAdmin bool,
	channel string,
) auth.Service {
	return &MockedAuthService{userID: userID, isAdmin: isAdmin, channel: channel}
}

type MockedAuthService struct {
	userID  string
	isAdmin bool
	// channel is how requests authenticate; empty means a browser.
	channel string
}

// withUser puts the mock user and auth channel on the request context.
func (m *MockedAuthService) withUser(r *http.Request) context.Context {
	channel := m.channel
	if channel == "" {
		channel = constants.ChannelBrowser
	}
	ctx := context.WithValue(r.Context(), constants.AuthChannelContextKey, channel)
	return context.WithValue(ctx, constants.UserContextKey, m.mockUser())
}

func (m *MockedAuthService) mockUser() models.User {
	role := models.RoleUser
	if m.isAdmin {
		role = models.RoleAdmin
	}
	return models.User{
		ID:          m.userID,
		Email:       mockUserEmail,
		Role:        role,
		AppAccess:   []string{"backlog", "watchparty", "recipes"},
		HasMFA:      false,
		DisplayName: "",
	}
}

func (m *MockedAuthService) Access(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := m.withUser(r)
		next(w, r.WithContext(ctx))
	}
}

func (m *MockedAuthService) TemplateAccess(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := m.withUser(r)
		next(w, r.WithContext(ctx))
	}
}

func (m *MockedAuthService) AdminAccess(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !m.isAdmin {
			http.Redirect(w, r, "/", http.StatusSeeOther)
			return
		}
		ctx := m.withUser(r)
		next(w, r.WithContext(ctx))
	}
}

func (m *MockedAuthService) AppAccess(
	_ string,
	next http.HandlerFunc,
) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := m.withUser(r)
		next(w, r.WithContext(ctx))
	}
}

func (m *MockedAuthService) GetAllUsers(
	_ context.Context,
) ([]models.User, error) {
	return []models.User{m.mockUser()}, nil
}

func (m *MockedAuthService) SignOut(
	_ context.Context,
	_ string,
	_ bool,
) (*http.Cookie, *http.Cookie, error) {
	return nil, nil, nil
}
