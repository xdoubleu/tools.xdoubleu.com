package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

var nameRE = regexp.MustCompile(`^[a-z][a-z0-9]+$`)

func main() {
	name := flag.String("name", "", "app name (lowercase, no spaces) [required]")
	withDB := flag.Bool("with-db", false, "generate DB repositories and migrations")
	withJobs := flag.Bool(
		"with-jobs",
		false,
		"generate background job queue wiring (implies --with-db)",
	)
	flag.Parse()

	if *name == "" {
		fmt.Fprintln(os.Stderr, "error: --name is required")
		flag.Usage()
		os.Exit(1)
	}

	if !nameRE.MatchString(*name) {
		fmt.Fprintf(os.Stderr, "error: --name %q must match ^[a-z][a-z0-9]+$\n", *name)
		os.Exit(1)
	}

	if *withJobs {
		*withDB = true
	}

	module, err := readModuleName()
	if err != nil {
		fmt.Fprintf(os.Stderr, "error reading go.mod: %v\n", err)
		os.Exit(1)
	}

	data := scaffoldData{
		Name:      *name,
		NameTitle: strings.ToUpper((*name)[:1]) + (*name)[1:],
		WithDB:    *withDB,
		WithJobs:  *withJobs,
		Module:    module,
	}

	repoRoot, err := findRepoRoot()
	if err != nil {
		fmt.Fprintf(os.Stderr, "error finding repo root: %v\n", err)
		os.Exit(1)
	}

	outDir := filepath.Join(repoRoot, "apps", data.Name)

	if _, statErr := os.Stat(outDir); statErr == nil {
		fmt.Fprintf(os.Stderr, "error: directory %s already exists\n", outDir)
		os.Exit(1)
	}

	if genErr := generateApp(outDir, data); genErr != nil {
		fmt.Fprintf(os.Stderr, "error generating app: %v\n", genErr)
		os.Exit(1)
	}

	appsGoPath := filepath.Join(repoRoot, "cmd", "publish", "apps.go")
	if injErr := injectApp(appsGoPath, data); injErr != nil {
		fmt.Fprintf(os.Stderr, "error injecting into apps.go: %v\n", injErr)
		os.Exit(1)
	}

	printSuccess(data, outDir)
}

func printSuccess(data scaffoldData, outDir string) {
	fmt.Fprintf(os.Stdout, "Scaffolded app %q at %s\n\n", data.Name, outDir)
	fmt.Fprintln(os.Stdout, "Next steps:")
	fmt.Fprintf(os.Stdout, "  1. Implement handlers in apps/%s/\n", data.Name)
	fmt.Fprintf(os.Stdout, "  2. Add domain logic to apps/%s/internal/\n", data.Name)
	if data.WithDB {
		fmt.Fprintf(
			os.Stdout,
			"  3. Edit apps/%s/migrations/00001_init.sql with your schema\n",
			data.Name,
		)
	}
	fmt.Fprintln(os.Stdout, "  Run: go build ./... to verify")
}

// readModuleName extracts the module path from go.mod.
func readModuleName() (string, error) {
	root, err := findRepoRoot()
	if err != nil {
		return "", err
	}

	raw, err := os.ReadFile(filepath.Join(root, "go.mod"))
	if err != nil {
		return "", err
	}

	for _, line := range strings.Split(string(raw), "\n") {
		if strings.HasPrefix(line, "module ") {
			return strings.TrimSpace(strings.TrimPrefix(line, "module ")), nil
		}
	}

	return "", fmt.Errorf("module declaration not found in go.mod")
}

// findRepoRoot walks up from cwd to find the directory containing go.mod.
func findRepoRoot() (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", err
	}

	for {
		if _, statErr := os.Stat(filepath.Join(dir, "go.mod")); statErr == nil {
			return dir, nil
		}

		parent := filepath.Dir(dir)
		if parent == dir {
			return "", fmt.Errorf("go.mod not found")
		}

		dir = parent
	}
}
