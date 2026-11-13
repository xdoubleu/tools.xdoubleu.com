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

	"tools.xdoubleu.com/selfupdate/appbundle"
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

// InstallDir holds the installed app and the runner's local state.
func InstallDir(homeDir string) string {
	return filepath.Join(homeDir, "Library", "Application Support", "factory-runner")
}

// InstalledBundlePath is the app the LaunchAgent runs; macOS only shows
// notifications from a process inside a bundle.
func InstalledBundlePath(homeDir string) string {
	return filepath.Join(InstallDir(homeDir), "FactoryRunner.app")
}

// LegacyBinaryPath is where installs before the app bundle kept the binary.
func LegacyBinaryPath(homeDir string) string {
	return filepath.Join(InstallDir(homeDir), "factory-runner")
}

// InstalledBinaryPath is the stable place the LaunchAgent runs the runner from,
// so a download in ~/Downloads can be deleted and self-update has one file to
// replace.
func InstalledBinaryPath(homeDir string) string {
	return filepath.Join(
		InstalledBundlePath(homeDir), "Contents", "MacOS", "factory-runner",
	)
}

// InstallBinary installs src's app bundle at InstalledBundlePath (a bare binary
// gets a minimal one) and returns the binary inside it. The caller seals the
// result with ResignBundle. Running the installed copy already is a no-op.
func InstallBinary(homeDir, src string) (string, error) {
	dst := InstalledBinaryPath(homeDir)
	if filepath.Clean(src) == dst {
		return dst, nil
	}
	bundle := InstalledBundlePath(homeDir)
	tmp := bundle + ".install"
	if err := os.RemoveAll(tmp); err != nil {
		return "", fmt.Errorf("clear stale install: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(bundle), installDirPerm); err != nil {
		return "", fmt.Errorf("create install dir: %w", err)
	}
	var err error
	if app := appbundle.Path(src); app != "" {
		err = copyBundle(app, tmp)
	} else {
		err = writeBundle(src, tmp)
	}
	if err == nil {
		err = os.RemoveAll(bundle)
	}
	if err == nil {
		err = os.Rename(tmp, bundle)
	}
	if err != nil {
		_ = os.RemoveAll(tmp)
		return "", fmt.Errorf("install app: %w", err)
	}
	return dst, nil
}

// copyBundle copies an app bundle, keeping modes and symlinks.
//
//nolint:gosec // copies the runner's own bundle into its install dir
func copyBundle(src, dst string) error {
	err := filepath.WalkDir(src, func(p string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, relErr := filepath.Rel(src, p)
		if relErr != nil {
			return relErr
		}
		target := filepath.Join(dst, rel)
		info, infoErr := d.Info()
		if infoErr != nil {
			return infoErr
		}
		switch {
		case d.IsDir():
			return os.MkdirAll(target, info.Mode().Perm()|0o700)
		case info.Mode()&os.ModeSymlink != 0:
			link, linkErr := os.Readlink(p)
			if linkErr != nil {
				return linkErr
			}
			return os.Symlink(link, target)
		default:
			data, readErr := os.ReadFile(p)
			if readErr != nil {
				return readErr
			}
			return os.WriteFile(target, data, info.Mode().Perm())
		}
	})
	if err != nil {
		return fmt.Errorf("copy app: %w", err)
	}
	return nil
}

func writeBundle(binary, bundle string) error {
	data, err := os.ReadFile(binary)
	if err != nil {
		return fmt.Errorf("read executable: %w", err)
	}
	macOS := filepath.Join(bundle, "Contents", "MacOS")
	if err = os.MkdirAll(macOS, installDirPerm); err != nil {
		return fmt.Errorf("create app: %w", err)
	}
	//nolint:gosec // must be executable
	err = os.WriteFile(filepath.Join(macOS, "factory-runner"), data, installFilePerm)
	if err != nil {
		return fmt.Errorf("write binary: %w", err)
	}
	plist := filepath.Join(bundle, "Contents", "Info.plist")
	//nolint:gosec // an Info.plist is world-readable
	if err = os.WriteFile(plist, []byte(bundleInfoPlist), 0o644); err != nil {
		return fmt.Errorf("write Info.plist: %w", err)
	}
	return nil
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
