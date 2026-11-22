#!/usr/bin/env bash
# Hexagonal apps (docs/convention-hexagonal-apps.md): adapters
# (apps/<app>/{connect,mcp,http}) call services and never import the app's
# internal/repositories, and an app's root package imports it only in app.go,
# where the wiring lives. depguard cannot glob an import path, so this covers
# every app, including a new one. mcp/ packages also never import a connect
# package: tools call services and map to DTOs themselves. Test files are exempt.
set -euo pipefail

# Paths below are relative to api/.
cd "$(dirname "$0")/.."

status=0
for app in apps/*/; do
  app=${app%/}
  repos="tools.xdoubleu.com/$app/internal/repositories"
  files=()
  for dir in connect mcp http; do
    [ -d "$app/$dir" ] || continue
    while IFS= read -r f; do files+=("$f"); done < <(find "$app/$dir" -name '*.go' ! -name '*_test.go')
  done
  while IFS= read -r f; do
    [ "$(basename "$f")" = app.go ] || files+=("$f")
  done < <(find "$app" -maxdepth 1 -name '*.go' ! -name '*_test.go')
  for f in "${files[@]+"${files[@]}"}"; do
    if grep -qE "\"$repos(/[^\"]*)?\"" "$f"; then
      echo "$f: imports $app/internal/repositories; go through a service (app.go may wire repositories)"
      status=1
    fi
  done
done

# MCP adapters (apps and platform modules) never import a connect package.
while IFS= read -r f; do
  if grep -qE '"tools\.xdoubleu\.com/[^"]*/connect"' "$f"; then
    echo "$f: imports a connect package; call a service and map to the DTO"
    status=1
  fi
done < <(find apps internal/platform -path '*/mcp/*' -name '*.go' ! -name '*_test.go')
exit $status
