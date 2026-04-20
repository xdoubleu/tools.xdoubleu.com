package services

import (
	"html/template"

	"github.com/supabase-community/gotrue-go"
	"github.com/xdoubleu/essentia/v3/pkg/config"
	cfg "tools.xdoubleu.com/internal/config"
	"tools.xdoubleu.com/internal/repositories"
)

type Services struct {
	Auth *AuthService
}

func New(
	cfg cfg.Config,
	supabaseClient gotrue.Client,
	tpl *template.Template,
	appUsersRepo *repositories.AppUsersRepository,
) *Services {
	return &Services{
		Auth: &AuthService{
			client:           supabaseClient,
			tpl:              tpl,
			useSecureCookies: cfg.Env == config.ProdEnv,
			accessExpiry:     cfg.AccessExpiry,
			refreshExpiry:    cfg.RefreshExpiry,
			appUsersRepo:     appUsersRepo,
		},
	}
}
