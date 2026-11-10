package runner

import (
	"bytes"
	"context"
	"encoding/xml"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// LaunchAgentLabel names the runner's LaunchAgent.
const LaunchAgentLabel = "com.xdoubleu.tools.factory-runner"

const launchAgentTemplate = `<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
	<key>Label</key>
	<string>%[1]s</string>
	<key>ProgramArguments</key>
	<array>
		<string>%[2]s</string>
		<string>run</string>
	</array>
	<key>RunAtLoad</key>
	<true/>
	<key>KeepAlive</key>
	<true/>
	<key>ThrottleInterval</key>
	<integer>10</integer>
	<key>StandardErrorPath</key>
	<string>%[3]s</string>
	<key>StandardOutPath</key>
	<string>%[3]s</string>
</dict>
</plist>
`

// LaunchAgentPath is where the plist lives under homeDir.
func LaunchAgentPath(homeDir string) string {
	return filepath.Join(homeDir, "Library", "LaunchAgents", LaunchAgentLabel+".plist")
}

// LogPath is where the LaunchAgent sends the runner's output.
func LogPath(homeDir string) string {
	return filepath.Join(homeDir, "Library", "Logs", "factory-runner.log")
}

func xmlEscape(s string) string {
	var escaped bytes.Buffer
	_ = xml.EscapeText(&escaped, []byte(s))
	return escaped.String()
}

func launchAgentPlist(homeDir, execPath string) string {
	return fmt.Sprintf(
		launchAgentTemplate,
		LaunchAgentLabel, xmlEscape(execPath), xmlEscape(LogPath(homeDir)),
	)
}

// InstallLaunchAgent writes the plist that keeps `factory-runner run` alive at
// login and returns its path; loading it is left to the caller.
func InstallLaunchAgent(homeDir, execPath string) (string, error) {
	path := LaunchAgentPath(homeDir)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return "", fmt.Errorf("create LaunchAgents dir: %w", err)
	}
	plist := []byte(launchAgentPlist(homeDir, execPath))
	//nolint:gosec // a LaunchAgent plist is world-readable
	if err := os.WriteFile(path, plist, 0o644); err != nil {
		return "", fmt.Errorf("write LaunchAgent: %w", err)
	}
	return path, nil
}

const (
	binaryReleaseTimeout = 5 * time.Second
	installDirPerm       = 0o755
	installFilePerm      = 0o755
)

// InstalledBinaryPath is the stable place the LaunchAgent runs the runner from,
// so a download in ~/Downloads can be deleted and self-update has one file to
// replace.
func InstalledBinaryPath(homeDir string) string {
	return filepath.Join(
		homeDir, "Library", "Application Support", "factory-runner", "factory-runner",
	)
}

// InstallBinary copies the running executable to InstalledBinaryPath and
// returns it. Running the installed copy already is a no-op.
func InstallBinary(homeDir, src string) (string, error) {
	dst := InstalledBinaryPath(homeDir)
	if filepath.Clean(src) == dst {
		return dst, nil
	}
	data, err := os.ReadFile(src)
	if err != nil {
		return "", fmt.Errorf("read executable: %w", err)
	}
	if err = os.MkdirAll(filepath.Dir(dst), installDirPerm); err != nil {
		return "", fmt.Errorf("create install dir: %w", err)
	}
	tmp := dst + ".install"
	//nolint:gosec // must be executable
	if err = os.WriteFile(tmp, data, installFilePerm); err != nil {
		return "", fmt.Errorf("write binary: %w", err)
	}
	if err = os.Rename(tmp, dst); err != nil {
		_ = os.Remove(tmp)
		return "", fmt.Errorf("install binary: %w", err)
	}
	return dst, nil
}

// BinaryRelease asks the binary at path for its stamped release; an empty
// string means it couldn't say (missing, not executable, or too old).
func BinaryRelease(path string) string {
	ctx, cancel := context.WithTimeout(context.Background(), binaryReleaseTimeout)
	defer cancel()
	out, err := exec.CommandContext(ctx, path, "version").Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

// LoginItemEnabled reports whether the LaunchAgent plist exists, which is what
// makes launchd start the runner at login.
func LoginItemEnabled(homeDir string) bool {
	_, err := os.Stat(LaunchAgentPath(homeDir))
	return err == nil
}

// DisableLoginItem removes the plist so the runner isn't started at the next
// login. The running process, and the loaded agent, are left alone.
func DisableLoginItem(homeDir string) error {
	err := os.Remove(LaunchAgentPath(homeDir))
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("remove LaunchAgent: %w", err)
	}
	return nil
}
