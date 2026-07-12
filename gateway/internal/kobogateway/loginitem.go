package kobogateway

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

const (
	loginItemLabel      = "com.xdoubleu.tools.kobo-gateway"
	loginItemMarkerFile = ".login-item-initialized"
	loginItemFilePerm   = 0o644
)

// loginItemPlistTemplate is a minimal LaunchAgent: run execPath once at
// login, no KeepAlive (the gateway doesn't need to be relaunched if it
// exits — the user quit it on purpose).
const loginItemPlistTemplate = `<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
	<key>Label</key>
	<string>%s</string>
	<key>ProgramArguments</key>
	<array>
		<string>%s</string>
	</array>
	<key>RunAtLoad</key>
	<true/>
</dict>
</plist>
`

// LoginItemPath returns where the gateway's LaunchAgent plist lives under
// homeDir (normally the real $HOME; a temp dir in tests).
func LoginItemPath(homeDir string) string {
	return filepath.Join(homeDir, "Library", "LaunchAgents", loginItemLabel+".plist")
}

// loginItemPlist renders the LaunchAgent plist that launches execPath at
// login.
func loginItemPlist(execPath string) string {
	return fmt.Sprintf(loginItemPlistTemplate, loginItemLabel, execPath)
}

// EnableLoginItem writes the LaunchAgent plist so execPath launches at every
// login, and asks launchctl to pick it up immediately (best-effort — a
// failure there still leaves the plist in place for the next login).
func EnableLoginItem(homeDir, execPath string) error {
	path := LoginItemPath(homeDir)

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create LaunchAgents dir: %w", err)
	}

	err := os.WriteFile(path, []byte(loginItemPlist(execPath)), loginItemFilePerm)
	if err != nil {
		return fmt.Errorf("write login item: %w", err)
	}

	bootstrapLoginItem(path)

	return nil
}

// DisableLoginItem removes the LaunchAgent plist so the gateway no longer
// launches at login. Not being enabled is not an error.
func DisableLoginItem(homeDir string) error {
	path := LoginItemPath(homeDir)

	bootoutLoginItem()

	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("remove login item: %w", err)
	}

	return nil
}

// LoginItemEnabled reports whether the LaunchAgent plist is currently
// installed.
func LoginItemEnabled(homeDir string) bool {
	_, err := os.Stat(LoginItemPath(homeDir))

	return err == nil
}

// EnsureInitialLoginItem registers the login item exactly once, the first
// time the gateway ever runs (tracked by a marker file in markerDir,
// alongside the TLS cert). After that first run, the user's own choice via
// the menu-bar toggle (Enable/DisableLoginItem) is left alone.
func EnsureInitialLoginItem(markerDir, homeDir, execPath string) error {
	markerPath := filepath.Join(markerDir, loginItemMarkerFile)
	if _, err := os.Stat(markerPath); err == nil {
		return nil
	}

	if err := EnableLoginItem(homeDir, execPath); err != nil {
		return err
	}

	if err := os.MkdirAll(markerDir, 0o700); err != nil {
		return fmt.Errorf("create marker dir: %w", err)
	}

	return os.WriteFile(markerPath, []byte("initialized\n"), loginItemFilePerm)
}

// bootstrapLoginItem/bootoutLoginItem best-effort nudge launchctl so the
// change takes effect immediately instead of at next login. Skipped under
// go test — there's no real gui/<uid> session and shelling out would just
// fail noisily on every CI run.
func bootstrapLoginItem(path string) {
	if testing.Testing() {
		return
	}

	//nolint:errcheck,gosec // best-effort; the plist file is the source of truth
	exec.Command("launchctl", "bootstrap", loginItemDomain(), path).Run()
}

func bootoutLoginItem() {
	if testing.Testing() {
		return
	}

	//nolint:errcheck,gosec // best-effort; removing the plist file is the source of truth
	exec.Command("launchctl", "bootout", loginItemDomain()+"/"+loginItemLabel).Run()
}

func loginItemDomain() string {
	return fmt.Sprintf("gui/%d", os.Getuid())
}
