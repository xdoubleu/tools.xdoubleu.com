package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func testData(withDB, withJobs bool) scaffoldData {
	return scaffoldData{
		Name:      "testapp",
		NameTitle: "Testapp",
		WithDB:    withDB,
		WithJobs:  withJobs,
		Module:    "tools.xdoubleu.com",
	}
}

func TestGenerateApp_NoDBNoJobs(t *testing.T) {
	outDir := t.TempDir()

	require.NoError(t, generateApp(outDir, testData(false, false)))

	assert.FileExists(t, filepath.Join(outDir, "app.go"))
	assert.FileExists(t, filepath.Join(outDir, "routes.go"))
	assert.FileExists(t, filepath.Join(outDir, "internal", "services", "main.go"))
	assert.FileExists(
		t,
		filepath.Join(outDir, "templates", "html", "testapp", "index.html"),
	)

	assert.NoDirExists(t, filepath.Join(outDir, "internal", "repositories"))
	assert.NoDirExists(t, filepath.Join(outDir, "migrations"))
	assert.NoDirExists(t, filepath.Join(outDir, "internal", "jobs"))
}

func TestGenerateApp_WithDB(t *testing.T) {
	outDir := t.TempDir()

	require.NoError(t, generateApp(outDir, testData(true, false)))

	assert.FileExists(t, filepath.Join(outDir, "internal", "repositories", "main.go"))
	assert.FileExists(t, filepath.Join(outDir, "migrations", "00001_init.sql"))
	assert.NoDirExists(t, filepath.Join(outDir, "internal", "jobs"))
}

func TestGenerateApp_WithJobs(t *testing.T) {
	outDir := t.TempDir()

	require.NoError(t, generateApp(outDir, testData(true, true)))

	assert.FileExists(t, filepath.Join(outDir, "internal", "repositories", "main.go"))
	assert.FileExists(t, filepath.Join(outDir, "migrations", "00001_init.sql"))
	assert.FileExists(t, filepath.Join(outDir, "internal", "jobs", "main.go"))
}

func TestGenerateApp_AlreadyExists(t *testing.T) {
	outDir := t.TempDir()
	existingFile := filepath.Join(outDir, "app.go")

	require.NoError(t, os.WriteFile(existingFile, []byte("existing"), 0o644))

	// generateApp itself does not check for existing dirs; main.go does.
	// We test that generating into an existing dir overwrites predictably
	// (no error from generateApp itself — the existence check is in main).
	err := generateApp(outDir, testData(false, false))
	assert.NoError(t, err)
}

func TestGenerateApp_AppGoContainsName(t *testing.T) {
	outDir := t.TempDir()

	require.NoError(t, generateApp(outDir, testData(false, false)))

	content, err := os.ReadFile(filepath.Join(outDir, "app.go"))
	require.NoError(t, err)
	assert.Contains(t, string(content), "package testapp")
	assert.Contains(t, string(content), "type Testapp struct")
	assert.Contains(t, string(content), `"testapp"`)
}

func TestGenerateApp_MigrationContainsName(t *testing.T) {
	outDir := t.TempDir()

	require.NoError(t, generateApp(outDir, testData(true, false)))

	content, err := os.ReadFile(filepath.Join(outDir, "migrations", "00001_init.sql"))
	require.NoError(t, err)
	assert.Contains(t, string(content), "CREATE SCHEMA IF NOT EXISTS testapp")
}
