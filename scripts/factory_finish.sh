#!/usr/bin/env bash
# Factory-job finish step: runs the gates for the modules changed since the base
# ref, offline (no gh, no docker), and prints the outcome's final "Checks:" line.
# Usage: scripts/factory_finish.sh [base-ref]   (default origin/main, else main)
# Exit 0 only when every gate passed or is left to CI.
set -uo pipefail

cd "$(git rev-parse --show-toplevel)"
base=${1:-origin/main}
git rev-parse --verify -q "$base" >/dev/null || base=main
files=$( { git diff --name-only "$base"...HEAD; git diff --name-only HEAD; } 2>/dev/null | sort -u)

touched() { grep -q "^$1/" <<<"$files"; }
declare -A res
run() { # run <gate> <dir> <cmd...>; any failure marks the gate fail
	local gate=$1 dir=$2
	shift 2
	echo "== $gate: $dir: $*" >&2
	if (cd "$dir" && "$@") >&2; then
		[ "${res[$gate]:-pass}" = fail ] || res[$gate]=pass
	else
		res[$gate]=fail
	fi
}

for m in api factory-runner wardcase selfupdate; do
	touched "$m" || continue
	[ "$m" = api ] && scripts/sandbox_postgres.sh >&2
	run lint "$m" make lint
	run test "$m" make test/cov/diff
done
# factory-runner imports wardcase and selfupdate (replace directives): lint, build
# and test it when only a dependency changed.
if (touched wardcase || touched selfupdate) && ! touched factory-runner; then
	run lint factory-runner make lint
	run build factory-runner go build ./...
	run test factory-runner make test
fi
if touched web; then
	run lint web npm run lint
	run test web npm run test:cov
	run build web npm run build
fi
if touched kobo-gateway; then
	res[build-mac]="not run (CI checks it)"
fi
if grep -q '^infra/' <<<"$files"; then
	res[lint-infra]="not run (CI checks it)"
fi
if grep -q '^infra/grafana/' <<<"$files"; then
	res[grafana-verify]="not run (CI checks it)"
fi
if grep -q '\.proto$' <<<"$files"; then
	run proto api make proto/check/local
	touched web && run proto web npm run generate:check:local
fi
run docs . make lint/docs

line="Checks:"
sep=" "
fail=0
for g in lint test build proto docs build-mac lint-infra grafana-verify; do
	[ -n "${res[$g]:-}" ] || continue
	line+="$sep$g=${res[$g]}"
	sep=", "
	[ "${res[$g]}" = fail ] && fail=1
done
echo "$line"
exit $fail
