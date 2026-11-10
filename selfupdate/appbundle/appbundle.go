// Package appbundle is what the menu-bar helpers share about running from a
// macOS .app bundle: finding it, and sealing it after the binary inside changed.
package appbundle

import (
	"context"
	"os/exec"
	"path/filepath"
)

// Path returns the .app bundle directory containing executable, or "" for a bare
// binary.
func Path(executable string) string {
	appDir := filepath.Dir(filepath.Dir(filepath.Dir(executable))) // .../Name.app
	if filepath.Ext(appDir) != ".app" {
		return ""
	}
	return appDir
}

// Resign re-applies package.sh's ad-hoc signature, which covers the binary as
// installed or just replaced. A bare binary has nothing to seal.
func Resign(ctx context.Context, executable string) error {
	appDir := Path(executable)
	if appDir == "" {
		return nil
	}
	return exec.CommandContext(ctx, "codesign", "--force", "--sign", "-", appDir).Run()
}
