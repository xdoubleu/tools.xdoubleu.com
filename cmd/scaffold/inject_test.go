package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const sampleAppsGo = `package main

import (
	"context"
	"html/template"
	"log/slog"
	"net/http"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/xdoubleu/essentia/v4/pkg/database/postgres"
	"tools.xdoubleu.com/apps/backlog"
	"tools.xdoubleu.com/apps/icsproxy"
	"tools.xdoubleu.com/apps/watchparty"
	"tools.xdoubleu.com/internal/auth"
	"tools.xdoubleu.com/internal/config"
)

func NewApps(
	_ context.Context,
	authService auth.Service,
	logger *slog.Logger,
	cfg config.Config,
	db postgres.DB,
	sharedTpl *template.Template,
	bl *backlog.Backlog,
) *Apps {
	var apps Apps = []App{}

	apps.addApp(bl)
	apps.addApp(watchparty.New(authService, logger, cfg, sharedTpl))
	apps.addApp(icsproxy.New(authService, logger, cfg, db, sharedTpl))
	// scaffold:app

	return &apps
}
`

func writeAppsGo(t *testing.T, content string) string {
	t.Helper()

	path := filepath.Join(t.TempDir(), "apps.go")

	require.NoError(t, os.WriteFile(path, []byte(content), 0o644))

	return path
}

func TestInjectApp_ImportsInserted(t *testing.T) {
	path := writeAppsGo(t, sampleAppsGo)

	//nolint:exhaustruct //WithDB and WithJobs are zero values (false)
	data := scaffoldData{
		Name:      "myapp",
		NameTitle: "Myapp",
		Module:    "tools.xdoubleu.com",
	}

	require.NoError(t, injectApp(path, data))

	content, err := os.ReadFile(path)
	require.NoError(t, err)
	assert.Contains(t, string(content), `"tools.xdoubleu.com/apps/myapp"`)
	assert.Contains(
		t,
		string(content),
		`apps.addApp(myapp.New(authService, logger, cfg, sharedTpl))`,
	)
}

func TestInjectApp_WithDB(t *testing.T) {
	path := writeAppsGo(t, sampleAppsGo)

	//nolint:exhaustruct //WithJobs is zero value (false)
	data := scaffoldData{
		Name:      "myapp",
		NameTitle: "Myapp",
		WithDB:    true,
		Module:    "tools.xdoubleu.com",
	}

	require.NoError(t, injectApp(path, data))

	content, err := os.ReadFile(path)
	require.NoError(t, err)
	assert.Contains(
		t,
		string(content),
		`apps.addApp(myapp.New(authService, logger, cfg, db, sharedTpl))`,
	)
}

func TestInjectApp_Idempotent(t *testing.T) {
	path := writeAppsGo(t, sampleAppsGo)

	//nolint:exhaustruct //WithDB and WithJobs are zero values (false)
	data := scaffoldData{
		Name:      "myapp",
		NameTitle: "Myapp",
		Module:    "tools.xdoubleu.com",
	}

	require.NoError(t, injectApp(path, data))
	require.NoError(t, injectApp(path, data))

	content, err := os.ReadFile(path)
	require.NoError(t, err)

	count := strings.Count(string(content), `"tools.xdoubleu.com/apps/myapp"`)
	assert.Equal(t, 1, count, "import should appear exactly once")

	count = strings.Count(string(content), `apps.addApp(myapp.New`)
	assert.Equal(t, 1, count, "addApp call should appear exactly once")
}

func TestInjectApp_MissingMarker(t *testing.T) {
	path := writeAppsGo(t, "package main\n\nimport (\n)\n")

	//nolint:exhaustruct //NameTitle, WithDB and WithJobs are zero values
	data := scaffoldData{Name: "myapp", Module: "tools.xdoubleu.com"}

	err := injectApp(path, data)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "scaffold:app")
}
