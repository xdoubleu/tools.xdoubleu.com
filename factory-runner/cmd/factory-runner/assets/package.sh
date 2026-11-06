#!/usr/bin/env bash
# Packages the built factory-runner binary into FactoryRunner.app + a
# drag-to-Applications factory-runner.dmg, plus the raw binary the in-app
# self-updater fetches. macOS only (sips/iconutil/hdiutil). Run via
# `make dist` (factory-runner/Makefile) after `make build`.
set -euo pipefail

RELEASE="${RELEASE:-dev}"
ROOT="$(cd "$(dirname "$0")/../../.." && pwd)" # factory-runner/
BIN="$ROOT/bin/factory-runner-darwin-arm64"
ASSETS="$ROOT/cmd/factory-runner/assets"
DIST="$ROOT/dist"
APP="$DIST/FactoryRunner.app"

if [[ ! -f "$BIN" ]]; then
  echo "error: $BIN not found — run 'make build' first" >&2
  exit 1
fi

rm -rf "$DIST"
mkdir -p "$APP/Contents/MacOS" "$APP/Contents/Resources"

cp "$BIN" "$APP/Contents/MacOS/factory-runner"
chmod +x "$APP/Contents/MacOS/factory-runner"

sed "s/__RELEASE__/$RELEASE/g" "$ASSETS/Info.plist" >"$APP/Contents/Info.plist"

# Build AppIcon.icns from the source PNG (assets/appicon.png is a placeholder
# derived from the site icon — swap it for a designed one and this keeps working).
ICONSET_PARENT=$(mktemp -d)
ICONSET="$ICONSET_PARENT/AppIcon.iconset"
mkdir -p "$ICONSET"
for size in 16 32 128 256 512; do
  sips -z "$size" "$size" "$ASSETS/appicon.png" --out "$ICONSET/icon_${size}x${size}.png" >/dev/null
  double=$((size * 2))
  sips -z "$double" "$double" "$ASSETS/appicon.png" --out "$ICONSET/icon_${size}x${size}@2x.png" >/dev/null
done
iconutil -c icns "$ICONSET" -o "$APP/Contents/Resources/AppIcon.icns"
rm -rf "$ICONSET_PARENT"

# Ad-hoc sign the bundle so a downloaded factory-runner.dmg opens without the
# app being damaged. The runner's self-update signature is separate (ed25519,
# keyed in advance by the CI build); this seal is only for Gatekeeper.
codesign --force --sign - "$APP"
codesign --verify --strict "$APP"

# Ship the raw binary too — the runner's self-updater replaces the installed
# copy with this, not the .dmg.
cp "$BIN" "$DIST/factory-runner-darwin-arm64"

# Pack the .app into a drag-to-Applications .dmg.
STAGING=$(mktemp -d)
cp -R "$APP" "$STAGING/"
ln -s /Applications "$STAGING/Applications"
hdiutil create -volname "Factory Runner" -srcfolder "$STAGING" -ov -format UDZO \
  "$DIST/factory-runner.dmg" >/dev/null
rm -rf "$STAGING"

echo "packaged $DIST/factory-runner.dmg and $DIST/factory-runner-darwin-arm64"
