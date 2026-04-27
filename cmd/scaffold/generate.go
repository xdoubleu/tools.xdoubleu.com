package main

import (
	"bytes"
	"embed"
	"fmt"
	"os"
	"path/filepath"
	"text/template"
)

//go:embed templates
var scaffoldTemplates embed.FS

type scaffoldData struct {
	Name      string
	NameTitle string
	WithDB    bool
	WithJobs  bool
	Module    string
}

// generateApp writes all scaffold files for a new app into outDir.
func generateApp(outDir string, data scaffoldData) error {
	files := []struct {
		tmpl    string
		outPath string
		cond    bool
	}{
		{"templates/app.go.tmpl", "app.go", true},
		{"templates/routes.go.tmpl", "routes.go", true},
		{
			"templates/services_main.go.tmpl",
			filepath.Join("internal", "services", "main.go"),
			true,
		},
		{
			"templates/index.html.tmpl",
			filepath.Join("templates", "html", data.Name, "index.html"),
			true,
		},
		{
			"templates/repos_main.go.tmpl",
			filepath.Join("internal", "repositories", "main.go"),
			data.WithDB,
		},
		{
			"templates/migration_init.sql.tmpl",
			filepath.Join("migrations", "00001_init.sql"),
			data.WithDB,
		},
		{
			"templates/jobs_main.go.tmpl",
			filepath.Join("internal", "jobs", "main.go"),
			data.WithJobs,
		},
	}

	for _, f := range files {
		if !f.cond {
			continue
		}

		dest := filepath.Join(outDir, f.outPath)
		if renderErr := renderTemplate(f.tmpl, dest, data); renderErr != nil {
			return fmt.Errorf("rendering %s: %w", f.tmpl, renderErr)
		}
	}

	return nil
}

func renderTemplate(tmplPath, destPath string, data scaffoldData) error {
	src, err := scaffoldTemplates.ReadFile(tmplPath)
	if err != nil {
		return err
	}

	tpl, parseErr := template.New(filepath.Base(tmplPath)).Parse(string(src))
	if parseErr != nil {
		return parseErr
	}

	var buf bytes.Buffer
	if execErr := tpl.Execute(&buf, data); execErr != nil {
		return execErr
	}

	if mkdirErr := os.MkdirAll(filepath.Dir(destPath), 0o755); mkdirErr != nil {
		return mkdirErr
	}

	//nolint:gosec // generated source files are intentionally world-readable (0o644)
	return os.WriteFile(destPath, buf.Bytes(), 0o644)
}
