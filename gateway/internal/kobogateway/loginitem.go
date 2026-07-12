package kobogateway

import (
	"bytes"
	"encoding/xml"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
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
// login. execPath is XML-escaped — an unescaped "&"/"<"/">" in the path
// (e.g. an app installed under a folder with an ampersand in its name) would
// otherwise produce malformed XML that launchd silently rejects.
func loginItemPlist(execPath string) string {
	var escaped bytes.Buffer
	_ = xml.EscapeText(&escaped, []byte(execPath))

	return fmt.Sprintf(loginItemPlistTemplate, loginItemLabel, escaped.String())
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

// launchctl runs the given launchctl args, best-effort. A var (not a plain
// func call) so export_test.go can swap in a no-op under go test — there's
// no real gui/<uid> session there and shelling out would just fail noisily
// on every CI run.
//
//nolint:gochecknoglobals // test seam, see export_test.go
var launchctl = func(args ...string) {
	//nolint:errcheck,gosec // best-effort; the plist file is the source of truth
	exec.Command("launchctl", args...).Run()
}

// bootstrapLoginItem/bootoutLoginItem best-effort nudge launchctl so the
// change takes effect immediately instead of at next login.
func bootstrapLoginItem(path string) {
	launchctl("bootstrap", loginItemDomain(), path)
}

func bootoutLoginItem() {
	launchctl("bootout", loginItemDomain()+"/"+loginItemLabel)
}

func loginItemDomain() string {
	return fmt.Sprintf("gui/%d", os.Getuid())
}
