package main

import (
	"fmt"
	"os"
	"strings"
)

const appMarker = "// scaffold:app"

// injectApp inserts the import and app registration for the new app into apps.go.
// The function is idempotent: it will not insert a line that already appears.
func injectApp(appsGoPath string, data scaffoldData) error {
	raw, err := os.ReadFile(appsGoPath)
	if err != nil {
		return err
	}

	lines := strings.Split(string(raw), "\n")

	importLine := fmt.Sprintf("\t%q", data.Module+"/apps/"+data.Name)

	dbArg := ""
	if data.WithDB {
		dbArg = "db, "
	}
	appLine := fmt.Sprintf(
		"\tapps.addApp(%s.New(authService, logger, cfg, %ssharedTpl))",
		data.Name,
		dbArg,
	)

	lines, err = insertImport(lines, importLine)
	if err != nil {
		return fmt.Errorf("inserting import: %w", err)
	}

	lines, err = insertBefore(lines, appMarker, appLine)
	if err != nil {
		return fmt.Errorf("app marker: %w", err)
	}

	//nolint:gosec // generated Go source files are intentionally world-readable
	return os.WriteFile(appsGoPath, []byte(strings.Join(lines, "\n")), 0o644)
}

// insertImport finds the closing ")" of the import block and inserts newLine
// immediately before it. Idempotent: does nothing if newLine already appears.
func insertImport(lines []string, newLine string) ([]string, error) {
	for _, l := range lines {
		if strings.TrimSpace(l) == strings.TrimSpace(newLine) {
			return lines, nil
		}
	}

	inImport := false
	for i, l := range lines {
		trimmed := strings.TrimSpace(l)
		if trimmed == "import (" {
			inImport = true
			continue
		}

		if inImport && trimmed == ")" {
			result := make([]string, 0, len(lines)+1)
			result = append(result, lines[:i]...)
			result = append(result, newLine)
			result = append(result, lines[i:]...)
			return result, nil
		}
	}

	return nil, fmt.Errorf("import block not found in file")
}

// insertBefore finds the line containing marker and inserts newLine immediately
// above it. Returns an error if the marker is not found. Idempotent.
func insertBefore(lines []string, marker, newLine string) ([]string, error) {
	for _, l := range lines {
		if strings.TrimSpace(l) == strings.TrimSpace(newLine) {
			return lines, nil
		}
	}

	for i, l := range lines {
		if strings.Contains(l, marker) {
			result := make([]string, 0, len(lines)+1)
			result = append(result, lines[:i]...)
			result = append(result, newLine)
			result = append(result, lines[i:]...)
			return result, nil
		}
	}

	return nil, fmt.Errorf("marker %q not found in file", marker)
}
