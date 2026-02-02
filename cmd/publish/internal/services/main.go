package services

import (
	"html/template"

	"github.com/supabase-community/gotrue-go"
	"github.com/xdoubleu/essentia/v2/pkg/config"
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
