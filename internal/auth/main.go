package auth

import (
	"net/http"

	"tools.xdoubleu.com/internal/models"
)

type Service interface {
	Access(next http.HandlerFunc) http.HandlerFunc
	TemplateAccess(next http.HandlerFunc) http.HandlerFunc
	GetAllUsers() ([]models.User, error)
	SignOut(accessToken string, secure bool) (*http.Cookie, *http.Cookie, error)
}
