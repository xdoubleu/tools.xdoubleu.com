package runner

// bundleInfoPlist is the minimal Info.plist of an installed app made from a bare
// binary; package.sh's full one ships in the dmg's app.
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
	<key>CFBundlePackageType</key>
	<string>APPL</string>
	<key>CFBundleExecutable</key>
	<string>factory-runner</string>
	<key>LSUIElement</key>
	<true/>
</dict>
</plist>
`
