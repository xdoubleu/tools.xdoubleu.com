package templates

import (
	"embed"
	"html/template"
	"net/http"
	"time"

	tpltools "github.com/xdoubleu/essentia/v3/pkg/tpl"
	"tools.xdoubleu.com/internal/config"
)

//go:embed html/shared/*.html
var sharedFS embed.FS

// RenderError renders the shared error.html page with the given HTTP status
// and message.
func RenderError(
	tpl *template.Template,
	w http.ResponseWriter,
	status int,
	message string,
) {
	w.WriteHeader(status)
	tpltools.RenderWithPanic(tpl, w, "error.html", map[string]any{
		"Status":  status,
		"Title":   http.StatusText(status),
		"Message": message,
	})
}

func LoadShared(cfg config.Config) *template.Template {
	return template.Must(template.New("shared").Funcs(template.FuncMap{
		"release": func() string {
			return cfg.Release[0:7]
		},
		"copyrightYear": func() int {
			return time.Now().Year()
		},
	}).ParseFS(
		sharedFS,
		"html/shared/*.html",
	))
}
