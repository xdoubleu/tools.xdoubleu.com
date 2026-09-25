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

// loginItemPlistTemplate is a LaunchAgent that runs execPath at login and
// relaunches it after any abnormal exit (panic, SIGABRT); a clean quit exits
// 0 and isn't relaunched.
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
	<key>KeepAlive</key>
	<dict>
		<key>SuccessfulExit</key>
		<false/>
	</dict>
</dict>
</plist>
`

// LoginItemPath returns where the gateway's LaunchAgent plist lives under
// homeDir (normally the real $HOME; a temp dir in tests).
func LoginItemPath(homeDir string) string {
	return filepath.Join(homeDir, "Library", "LaunchAgents", loginItemLabel+".plist")
}

// loginItemPlist renders the plist; execPath is XML-escaped since launchd
// silently rejects malformed XML.
func loginItemPlist(execPath string) string {
	var escaped bytes.Buffer
	_ = xml.EscapeText(&escaped, []byte(execPath))

	return fmt.Sprintf(loginItemPlistTemplate, loginItemLabel, escaped.String())
}

// writeLoginItemPlist renders and writes the LaunchAgent plist for execPath
// to homeDir, creating the LaunchAgents directory if needed.
func writeLoginItemPlist(homeDir, execPath string) error {
	path := LoginItemPath(homeDir)

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create LaunchAgents dir: %w", err)
	}

	if err := os.WriteFile(path, []byte(loginItemPlist(execPath)), loginItemFilePerm); err != nil { //nolint:lll
		return fmt.Errorf("write login item: %w", err)
	}

	return nil
}

// EnableLoginItem writes the plist and best-effort loads it via launchctl.
func EnableLoginItem(homeDir, execPath string) error {
	if err := writeLoginItemPlist(homeDir, execPath); err != nil {
		return err
	}

	bootstrapLoginItem(LoginItemPath(homeDir))

	return nil
}

// SyncLoginItem rewrites an installed plist to the current template without
// launchctl (a bootout would SIGTERM this process); it applies at next load.
// Does nothing when the login item is disabled.
func SyncLoginItem(homeDir, execPath string) error {
	if !LoginItemEnabled(homeDir) {
		return nil
	}

	return writeLoginItemPlist(homeDir, execPath)
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

// LoginItemEnabled reports whether the plist is installed.
func LoginItemEnabled(homeDir string) bool {
	_, err := os.Stat(LoginItemPath(homeDir))

	return err == nil
}

// IsFirstLaunch reports whether the gateway has never run on this machine.
// Check it before EnsureInitialLoginItem, which creates the marker.
func IsFirstLaunch(markerDir string) bool {
	_, err := os.Stat(filepath.Join(markerDir, loginItemMarkerFile))

	return os.IsNotExist(err)
}

// EnsureInitialLoginItem registers the login item only on the very first
// run; after that the user's menu-bar choice stands.
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

// launchctl is a var so tests can stub it (no gui/<uid> session there).
//
//nolint:gochecknoglobals // test seam, see export_test.go
var launchctl = func(args ...string) {
	//nolint:errcheck // best-effort; the plist file is the source of truth
	exec.Command("launchctl", args...).Run()
}

// bootstrapLoginItem/bootoutLoginItem apply the change now instead of at
// next login.
func bootstrapLoginItem(path string) {
	launchctl("bootstrap", loginItemDomain(), path)
}

func bootoutLoginItem() {
	launchctl("bootout", loginItemDomain()+"/"+loginItemLabel)
}

func loginItemDomain() string {
	return fmt.Sprintf("gui/%d", os.Getuid())
}
