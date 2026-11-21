#!/usr/bin/env bash
# Fails if a plugin version pinned for `buf generate` (buf.gen.yaml's
# `remote:` line, resolved from buf.build) drifts from the version installed
# locally for `buf generate --template buf.gen.local.yaml` (the offline path
# for environments that can't reach buf.build). The pins necessarily live in
# different files, so nothing else catches them drifting.
#
# Checked pairs:
#   1. api/buf.gen.yaml   protoc-gen-connect-go pin <-> flake.nix's
#      protoc-gen-connect-go `version`
#   2. web/buf.gen.yaml   protoc-gen-es pin         <-> web/package.json's
#      "@bufbuild/protoc-gen-es" devDependency version
#
# Run via `make lint/proto-local-versions`; paths resolve from the repo root.
# Kept POSIX-bash-3.2 clean (macOS).
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
flake_nix="$root/flake.nix"
web_package_json="$root/web/package.json"

connect_want="$(remote_version connectrpc/go "$api_gen_yaml")"
connect_got="$(grep -A1 'pname = "protoc-gen-connect-go"' "$flake_nix" | grep -oE 'version = "[0-9.]+"' | grep -oE '[0-9.]+' | sed 's/^/v/' || true)"
check "protoc-gen-connect-go" "$connect_want" "$connect_got" "flake.nix"

es_want="$(remote_version bufbuild/es "$web_gen_yaml" | sed 's/^v//')"
es_got="$(grep -oE '"@bufbuild/protoc-gen-es": *"[0-9.]+"' "$web_package_json" | grep -oE '[0-9.]+"$' | tr -d '"' || true)"
check "protoc-gen-es" "$es_want" "$es_got" "web/package.json devDependencies"

if [ "$status" -ne 0 ]; then
	echo "" >&2
	echo "Proto local-generation plugin versions are inconsistent — see errors above." >&2
fi
exit "$status"
