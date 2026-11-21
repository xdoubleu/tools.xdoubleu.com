package runner

import (
	"bytes"
	_ "embed"
	"encoding/binary"
	"fmt"
	"image/png"
	"os"
	"path/filepath"
	"strings"
)

// bundleInfoPlist is the Info.plist of an installed app made from a bare
// binary, matching assets/Info.plist in the dmg's app; __RELEASE__ is replaced
// by the binary's release.
const bundleInfoPlist = `<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
	<key>CFBundleName</key>
	<string>FactoryRunner</string>
	<key>CFBundleDisplayName</key>
	<string>Factory Runner</string>
	<key>CFBundleIdentifier</key>
	<string>com.xdoubleu.tools.factory-runner</string>
	<key>CFBundleVersion</key>
	<string>__RELEASE__</string>
	<key>CFBundleShortVersionString</key>
	<string>__RELEASE__</string>
	<key>CFBundlePackageType</key>
	<string>APPL</string>
	<key>CFBundleExecutable</key>
	<string>factory-runner</string>
	<key>CFBundleIconFile</key>
	<string>AppIcon</string>
	<key>LSUIElement</key>
	<true/>
	<key>LSMinimumSystemVersion</key>
	<string>13.0</string>
	<key>NSHighResolutionCapable</key>
	<true/>
</dict>
</plist>
`

//go:embed appicon.png
var appIconPNG []byte

// icnsTypes maps a square PNG's edge to its icns PNG-entry type.
var icnsTypes = map[int]string{ //nolint:gochecknoglobals // constant table
	16: "icp4", 32: "icp5", 64: "icp6", 128: "ic07",
	256: "ic08", 512: "ic09", 1024: "ic10",
}

const icnsHeader = 8

// appIconICNS wraps the embedded PNG in an icns container, so a bare-binary
// install needs no macOS tooling.
func appIconICNS() ([]byte, error) {
	cfg, err := png.DecodeConfig(bytes.NewReader(appIconPNG))
	if err != nil {
		return nil, fmt.Errorf("decode app icon: %w", err)
	}
	kind, ok := icnsTypes[cfg.Width]
	if !ok || cfg.Width != cfg.Height {
		return nil, fmt.Errorf("app icon is %dx%d, not an icns size", cfg.Width, cfg.Height)
	}
	entry := uint32(icnsHeader + len(appIconPNG)) //nolint:gosec // small embedded file
	var out bytes.Buffer
	out.WriteString("icns")
	_ = binary.Write(&out, binary.BigEndian, entry+icnsHeader)
	out.WriteString(kind)
	_ = binary.Write(&out, binary.BigEndian, entry)
	out.Write(appIconPNG)
	return out.Bytes(), nil
}

// writeBundleResources writes the Info.plist and icon: everything in the app
// but the binary.
func writeBundleResources(bundle, release string) error {
	icns, err := appIconICNS()
	if err != nil {
		return err
	}
	resources := filepath.Join(bundle, "Contents", "Resources")
	if err = os.MkdirAll(resources, installDirPerm); err != nil {
		return fmt.Errorf("create resources: %w", err)
	}
	//nolint:gosec // app resources are world-readable
	err = os.WriteFile(filepath.Join(resources, "AppIcon.icns"), icns, 0o644)
	if err != nil {
		return fmt.Errorf("write icon: %w", err)
	}
	plist := strings.ReplaceAll(bundleInfoPlist, "__RELEASE__", xmlEscape(release))
	info := filepath.Join(bundle, "Contents", "Info.plist")
	//nolint:gosec // an Info.plist is world-readable
	err = os.WriteFile(info, []byte(plist), 0o644)
	if err != nil {
		return fmt.Errorf("write Info.plist: %w", err)
	}
	return nil
}

// RepairBundle rewrites the Info.plist and icon of the installed app when an
// install from a bare binary left it without them, and reports whether it did.
// The binary is untouched; the caller re-seals the bundle on true.
func RepairBundle(homeDir, release string) (bool, error) {
	bundle := InstalledBundlePath(homeDir)
	icon := filepath.Join(bundle, "Contents", "Resources", "AppIcon.icns")
	if _, err := os.Stat(icon); err == nil {
		return false, nil
	}
	if _, err := os.Stat(InstalledBinaryPath(homeDir)); err != nil {
		return false, nil //nolint:nilerr // nothing installed to repair
	}
	if err := writeBundleResources(bundle, release); err != nil {
		return false, err
	}
	return true, nil
}
