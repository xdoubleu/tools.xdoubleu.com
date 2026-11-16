#!/usr/bin/env bash
# Install the tools pinned in tools/ci-tools.txt: `setup-ci-tools.sh [go|pip|all]`.
# CI workflows and factory-runner/Containerfile both call it, so they can't drift.
# PIP overrides the pip command (the job image installs into a venv).
set -euo pipefail

here="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
manifest="${CI_TOOLS_MANIFEST:-$here/ci-tools.txt}"
want="${1:-all}"

case "$want" in
go | pip | all) ;;
*)
	echo "usage: $0 [go|pip|all]" >&2
	exit 2
	;;
esac

while read -r kind name version _; do
	case "$kind" in '' | '#'*) continue ;; esac
	[ "$want" = all ] || [ "$want" = "$kind" ] || continue
	case "$kind" in
	go) go install "$name@$version" ;;
	pip) ${PIP:-pip} install --no-cache-dir "$name==$version" ;;
	*)
		echo "unknown kind '$kind' in $manifest" >&2
		exit 1
		;;
	esac
done <"$manifest"
