#!/usr/bin/env bash
# Fails if a plugin version pinned for `buf generate` (buf.gen.yaml's
# `remote:` line, resolved from buf.build) drifts from the version installed
# locally for `buf generate --template buf.gen.local.yaml` (issue #1426) —
# the offline path Claude Code on the web uses since it can't reach
# buf.build. The two pins live in different files by necessity (a `remote:`
# plugin carries its version in the pin itself; a `local:` plugin just names
# a binary already on PATH), so nothing else catches them going out of sync.
#
# Checked pairs:
#   1. api/buf.gen.yaml   protoc-gen-go pin        <-> api/Makefile's
#      `go install google.golang.org/protobuf/cmd/protoc-gen-go@vX`
#   2. api/buf.gen.yaml   protoc-gen-connect-go pin <-> api/Makefile's
#      `go install connectrpc.com/connect/cmd/protoc-gen-connect-go@vX`
#   3. web/buf.gen.yaml   protoc-gen-es pin         <-> web/package.json's
#      "@bufbuild/protoc-gen-es" devDependency version (protoc-gen-es itself
#      is already run from node_modules/.bin, so no separate install pin
#      exists to drift — package.json's pin *is* the local plugin's version)
#
# Run from api/ (via `make lint/proto-local-versions`). Paths resolve
# relative to the repo root so it works from anywhere. Kept POSIX-bash-3.2
# clean (macOS).
set -euo pipefail

root="$(cd "$(dirname "$0")/../.." && pwd)"

status=0

# Pulls the vX.Y.Z out of a `remote: buf.build/<owner>/<name>:vX.Y.Z` line.
remote_version() {
	grep -oE "remote: buf\.build/$1:v[0-9.]+" "$2" | grep -oE 'v[0-9.]+$'
}

check() {
	label="$1"; want="$2"; got="$3"; hint="$4"
	if [ -z "$got" ]; then
		echo "ERROR: could not determine the local pin for $label ($hint)" >&2
		status=1; return
	fi
	if [ "$want" != "$got" ]; then
		echo "ERROR: $label is pinned to $want in buf.gen.yaml but $got locally ($hint) — keep both in step." >&2
		status=1
	fi
}

api_gen_yaml="$root/api/buf.gen.yaml"
web_gen_yaml="$root/web/buf.gen.yaml"
api_makefile="$root/api/Makefile"
web_package_json="$root/web/package.json"

go_want="$(remote_version protocolbuffers/go "$api_gen_yaml")"
go_got="$(grep -oE 'google\.golang\.org/protobuf/cmd/protoc-gen-go@v[0-9.]+' "$api_makefile" | grep -oE 'v[0-9.]+$' || true)"
check "protoc-gen-go" "$go_want" "$go_got" "api/Makefile tools/proto/local"

connect_want="$(remote_version connectrpc/go "$api_gen_yaml")"
connect_got="$(grep -oE 'connectrpc\.com/connect/cmd/protoc-gen-connect-go@v[0-9.]+' "$api_makefile" | grep -oE 'v[0-9.]+$' || true)"
check "protoc-gen-connect-go" "$connect_want" "$connect_got" "api/Makefile tools/proto/local"

es_want="$(remote_version bufbuild/es "$web_gen_yaml" | sed 's/^v//')"
es_got="$(grep -oE '"@bufbuild/protoc-gen-es": *"[0-9.]+"' "$web_package_json" | grep -oE '[0-9.]+"$' | tr -d '"' || true)"
check "protoc-gen-es" "$es_want" "$es_got" "web/package.json devDependencies"

if [ "$status" -ne 0 ]; then
	echo "" >&2
	echo "Proto local-generation plugin versions are inconsistent — see errors above." >&2
fi
exit "$status"
