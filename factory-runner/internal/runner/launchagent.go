package runner

import (
	"bytes"
	"encoding/xml"
	"fmt"
	"os"
	"path/filepath"
)

// LaunchAgentLabel names the runner's LaunchAgent.
const LaunchAgentLabel = "com.xdoubleu.tools.factory-runner"

const launchAgentTemplate = `<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
	<key>Label</key>
	<string>%s</string>
	<key>ProgramArguments</key>
	<array>
		<string>%s</string>
		<string>run</string>
	</array>
	<key>RunAtLoad</key>
	<true/>
	<key>KeepAlive</key>
	<true/>
	<key>ThrottleInterval</key>
	<integer>10</integer>
</dict>
</plist>
`

// LaunchAgentPath is where the plist lives under homeDir.
func LaunchAgentPath(homeDir string) string {
	return filepath.Join(homeDir, "Library", "LaunchAgents", LaunchAgentLabel+".plist")
}

func launchAgentPlist(execPath string) string {
	var escaped bytes.Buffer
	_ = xml.EscapeText(&escaped, []byte(execPath))
	return fmt.Sprintf(launchAgentTemplate, LaunchAgentLabel, escaped.String())
}

// InstallLaunchAgent writes the plist that keeps `factory-runner run` alive at
// login and returns its path; loading it is left to the caller.
func InstallLaunchAgent(homeDir, execPath string) (string, error) {
	path := LaunchAgentPath(homeDir)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return "", fmt.Errorf("create LaunchAgents dir: %w", err)
	}
	plist := []byte(launchAgentPlist(execPath))
	//nolint:gosec // a LaunchAgent plist is world-readable
	if err := os.WriteFile(path, plist, 0o644); err != nil {
		return "", fmt.Errorf("write LaunchAgent: %w", err)
	}
	return path, nil
}
