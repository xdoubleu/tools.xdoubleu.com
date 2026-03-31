package templates

import (
	"embed"
	"html/template"
	"time"

	"tools.xdoubleu.com/internal/config"
)

//go:embed html/shared/*.html
var sharedFS embed.FS

func LoadShared(cfg config.Config) *template.Template {
	return template.Must(template.New("shared").Funcs(template.FuncMap{
		"release": func() string {
			return cfg.Release
		},
		"copyrightYear": func() int {
			return time.Now().Year()
		},
	}).ParseFS(
		sharedFS,
		"html/shared/*.html",
	))
}
