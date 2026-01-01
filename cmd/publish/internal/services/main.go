package services

import (
	"html/template"

	"github.com/XDoubleU/essentia/pkg/config"
	"github.com/supabase-community/gotrue-go"
	cfg "tools.xdoubleu.com/internal/config"
)

type Services struct {
	Auth *AuthService
}

func New(
	cfg cfg.Config,
	supabaseClient gotrue.Client,
	tpl *template.Template,
) *Services {
	return &Services{
		Auth: &AuthService{
			supabaseUserID:   cfg.SupabaseUserID,
			client:           supabaseClient,
			tpl:              tpl,
			useSecureCookies: cfg.Env == config.ProdEnv,
			accessExpiry:     cfg.AccessExpiry,
			refreshExpiry:    cfg.RefreshExpiry,
		},
	}
}
