#!/usr/bin/env bash
# Packages the built kobo-gateway binary into KoboGateway.app + a
# drag-to-Applications kobo-gateway.dmg, plus the raw binary the in-app
# self-updater fetches. macOS only (sips/iconutil/hdiutil). Run via
# `make dist` (gateway/Makefile) after `make build`.
set -euo pipefail

RELEASE="${RELEASE:-dev}"
ROOT="$(cd "$(dirname "$0")/../../.." && pwd)" # gateway/
BIN="$ROOT/bin/kobo-gateway-darwin-arm64"
ASSETS="$ROOT/cmd/kobo-gateway/assets"
DIST="$ROOT/dist/gateway"
APP="$DIST/KoboGateway.app"

if [[ ! -f "$BIN" ]]; then
  echo "error: $BIN not found — run 'make build/kobo-gateway' first" >&2
  exit 1
fi

rm -rf "$DIST"
mkdir -p "$APP/Contents/MacOS" "$APP/Contents/Resources"

cp "$BIN" "$APP/Contents/MacOS/kobo-gateway"
chmod +x "$APP/Contents/MacOS/kobo-gateway"

sed "s/__RELEASE__/$RELEASE/g" "$ASSETS/Info.plist" >"$APP/Contents/Info.plist"

# Build AppIcon.icns from the source PNG (assets/appicon.png is a
# placeholder — swap it for a designed icon and this keeps working).
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

# Ship the raw binary too — the in-app self-updater (POST /update) replaces
# Contents/MacOS/kobo-gateway with this, not the .dmg.
cp "$BIN" "$DIST/kobo-gateway-darwin-arm64"

# Pack the .app into a drag-to-Applications .dmg.
STAGING=$(mktemp -d)
cp -R "$APP" "$STAGING/"
ln -s /Applications "$STAGING/Applications"
hdiutil create -volname "Kobo Gateway" -srcfolder "$STAGING" -ov -format UDZO \
  "$DIST/kobo-gateway.dmg" >/dev/null
rm -rf "$STAGING"

echo "packaged $DIST/kobo-gateway.dmg and $DIST/kobo-gateway-darwin-arm64"
