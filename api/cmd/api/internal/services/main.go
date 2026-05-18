package services

import (
	"github.com/supabase-community/gotrue-go"
	"github.com/xdoubleu/essentia/v4/pkg/config"

	cfg "tools.xdoubleu.com/internal/config"
	"tools.xdoubleu.com/internal/repositories"
)

type Services struct {
	Auth *AuthService
}

func New(
	cfg cfg.Config,
	supabaseClient gotrue.Client,
	appUsersRepo *repositories.AppUsersRepository,
) *Services {
	return &Services{
		Auth: &AuthService{
			client:           supabaseClient,
			useSecureCookies: cfg.Env == config.ProdEnv,
			accessExpiry:     cfg.AccessExpiry,
			refreshExpiry:    cfg.RefreshExpiry,
			appUsersRepo:     appUsersRepo,
			SignInRenderer:   nil,
		},
	}
}
