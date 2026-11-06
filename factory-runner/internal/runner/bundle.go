package runner

import "path/filepath"

// AppBundlePath returns the enclosing .app bundle of an executable, or "" when
// it runs as a bare binary. Launching inside a bundle (double-clicking the app)
// is how the drag-to-Applications install starts.
func AppBundlePath(executable string) string {
	appDir := filepath.Dir(filepath.Dir(filepath.Dir(executable))) // .../FactoryRunner.app
	if filepath.Ext(appDir) != ".app" {
		return ""
	}
	return appDir
}
