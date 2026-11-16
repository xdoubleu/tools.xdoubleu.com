#!/usr/bin/env bash
# Fail when a workflow or Makefile installs a tool that tools/ci-tools.txt lacks or pins
# differently, or when the job image stops installing from that manifest. The job image
# has no network, so a tool only CI knows about is a sandbox block.
set -euo pipefail

root="${LINT_CI_TOOLS_ROOT:-$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)}"
manifest="$root/tools/ci-tools.txt"
status=0

# Pins as "<name> <version without v>", from every place that installs a tool.
found=$(
	{
		grep -rhoE 'go install [^ ]+@v[0-9][^ ]*' "$root"/.github "$root"/Makefile \
			"$root"/*/Makefile 2>/dev/null |
			sed -E 's/go install ([^@]+)@v?(.*)/\1 \2/'
		grep -rhoE 'ensure_go_tool\.sh [^ ]+ [^ ]+ [^ ]+ [0-9][^ ]*' "$root"/Makefile \
			"$root"/*/Makefile 2>/dev/null | awk '{print $4, $5}'
		grep -rhoE 'pip install [A-Za-z0-9_.-]+==[^ ]+' "$root"/.github "$root"/Makefile \
			"$root"/*/Makefile 2>/dev/null |
			sed -E 's/pip install ([^=]+)==(.*)/\1 \2/'
		# golangci-lint-action pins its version in a `version:` under the renovate marker.
		grep -rhA1 'renovate: datasource=go depName=' "$root"/.github 2>/dev/null |
			awk '/depName=/ {split($0, a, "depName="); n = a[2]; next}
				/version:/ {print n, $2; n = ""}'
	} | sed -E 's/ v/ /' | sort -u
)

while read -r name version; do
	[ -n "$name" ] || continue
	want=$(awk -v n="$name" '$2 == n {sub(/^v/, "", $3); print $3}' "$manifest")
	if [ -z "$want" ]; then
		echo "$name is installed by CI but missing from tools/ci-tools.txt" >&2
		status=1
	elif [ "$want" != "$version" ]; then
		echo "$name: CI pins $version, tools/ci-tools.txt pins $want" >&2
		status=1
	fi
done <<<"$found"

if ! grep -q 'setup-ci-tools.sh' "$root/factory-runner/Containerfile"; then
	echo "factory-runner/Containerfile must install tools via tools/setup-ci-tools.sh" >&2
	status=1
fi

[ "$status" -eq 0 ] || exit 1
echo "ok: every tool CI installs is in tools/ci-tools.txt and the job image"
