package templates

import (
	"embed"
	"html/template"
)

//go:embed html/shared/*.html
var sharedFS embed.FS

func LoadShared() *template.Template {
	return template.Must(template.New("shared").ParseFS(
		sharedFS,
		"html/shared/*.html",
	))
}