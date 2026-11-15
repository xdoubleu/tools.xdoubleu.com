#!/usr/bin/env bash
# Run the changed-line coverage gate (diff-coverage/) from the current
# module or project directory: scripts/diff_coverage.sh go|lcov|merge ...
# Adds this repo's include/exclude globs; flags given later still apply.
set -euo pipefail

root=$(git rev-parse --show-toplevel)
bin="${TMPDIR:-/tmp}/diff-coverage-$(id -u)"
go build -C "$root/diff-coverage" -o "$bin" .

cmd=${1:?usage: diff_coverage.sh go|lcov|merge ...}
shift
case "$cmd" in
go)
  # AppKit code has no window server under go test; entry.go is process wiring.
  set -- --exclude '**/*_mock.go' --exclude '**/gen/**' \
    --exclude '**/menubar_darwin.go' --exclude 'cmd/factory-runner/entry.go' "$@"
  ;;
lcov)
  # Mirrors web/vite.config.ts test.coverage include/exclude.
  set -- --include 'components/**' --include 'lib/**' --include 'hooks/**' \
    --include 'app/**' --include 'src/**' \
    --exclude 'lib/gen/**' --exclude '**/*.d.ts' --exclude 'src/main.tsx' \
    --exclude '**/*.test.ts' --exclude '**/*.test.tsx' "$@"
  ;;
esac
exec "$bin" "$cmd" "$@"
