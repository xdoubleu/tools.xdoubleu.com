#!/usr/bin/env bash
# Fails if the Kamal deploy-secret list disagrees across the three places it
# is declared:
#
#   1. config/deploy.{api,web}.yml   -> env.secret:      (what Kamal injects)
#   2. .kamal/secrets                                    (shell script Kamal resolves each name from)
#   3. .github/workflows/main.yml     -> deploy-kamal job -> the per-service
#      "Deploy <svc> via Kamal" step's env: block        (maps a repo Secret into that script's env)
#
# A name added to (1) but not (2)/(3) passes every PR check and only blows up
# when `kamal deploy` runs post-merge on main (which is never re-tested):
#   ERROR (Kamal::ConfigurationError): Secret 'X' not found in .kamal/secrets
# That is exactly how #1390's BMC_PARTNER_KEY reached production (fixed in
# #1404); issue #1405 added this check so the next one turns a PR red instead.
#
# Run from api/ (via `make lint/kamal-secrets`). Paths resolve relative to the
# repo root so it works from anywhere. Kept POSIX-bash-3.2 clean (macOS).
set -euo pipefail

root="$(cd "$(dirname "$0")/../.." && pwd)"
secrets_file="$root/.kamal/secrets"
workflow="$root/.github/workflows/main.yml"

status=0

# Newline-delimited list of names defined in .kamal/secrets (`NAME=$NAME`).
defined="$(grep -oE '^[A-Z0-9_]+=' "$secrets_file" | sed 's/=$//' | sort -u)"

# env.secret: list from a deploy config. The block is `  secret:` nested
# under `env:`, followed by `    - NAME` items until the indentation drops.
extract_config_secrets() {
	awk '
		/^  secret:[[:space:]]*$/ { inblock = 1; next }
		inblock && /^    - [A-Z0-9_]+[[:space:]]*$/ { gsub(/[ \t-]/, ""); print; next }
		inblock && /^  [^ ]/ { inblock = 0 }
		inblock && /^[^ ]/   { inblock = 0 }
	' "$1" | sort -u
}

# env: keys of one `- name: Deploy <svc> via Kamal` step in the deploy-kamal
# job. Keys are indented 10 spaces; the block ends at the step's `run:`.
extract_workflow_env_keys() {
	awk -v svc="$1" '
		$0 ~ "- name: Deploy " svc " via Kamal" { instep = 1; inenv = 0; next }
		instep && /^        env:[[:space:]]*$/ { inenv = 1; next }
		instep && /^        run:/ { exit }
		inenv && /^          [A-Z0-9_]+:/ { k = $1; sub(/:$/, "", k); print k }
	' "$workflow" | sort -u
}

in_list() { grep -qxF "$1" <<<"$2"; }

check_service() {
	svc="$1"; config="$2"
	cfg_secrets="$(extract_config_secrets "$config")"
	env_keys="$(extract_workflow_env_keys "$svc")"

	if [ -z "$cfg_secrets" ]; then
		echo "ERROR: no env.secret entries parsed from $config — parser or file layout changed" >&2
		status=1; return
	fi
	if [ -z "$env_keys" ]; then
		echo "ERROR: no env: keys parsed from the 'Deploy $svc via Kamal' step in $workflow — parser or file layout changed" >&2
		status=1; return
	fi

	while IFS= read -r name; do
		[ -n "$name" ] || continue
		if ! in_list "$name" "$defined"; then
			echo "ERROR: $config lists secret '$name' in env.secret: but .kamal/secrets never defines it (add '$name=\$$name')" >&2
			status=1
		fi
		if ! in_list "$name" "$env_keys"; then
			echo "ERROR: $config lists secret '$name' in env.secret: but the 'Deploy $svc via Kamal' step in main.yml never puts it in env: (add '$name: \${{ secrets.$name }}')" >&2
			status=1
		fi
	done <<<"$cfg_secrets"
}

check_service api "$root/config/deploy.api.yml"
check_service web "$root/config/deploy.web.yml"

# Informational only: names sitting in .kamal/secrets that no deploy config
# references. KAMAL_* are consumed by Kamal's own config schema (registry
# auth), not via env.secret, so they are expected orphans.
all_cfg_secrets="$(
	{ extract_config_secrets "$root/config/deploy.api.yml"
	  extract_config_secrets "$root/config/deploy.web.yml"; } | sort -u
)"
while IFS= read -r name; do
	[ -n "$name" ] || continue
	case "$name" in KAMAL_*) continue ;; esac
	in_list "$name" "$all_cfg_secrets" || echo "note: .kamal/secrets defines '$name' but no deploy config references it (dead entry?)" >&2
done <<<"$defined"

if [ "$status" -ne 0 ]; then
	echo "" >&2
	echo "Kamal deploy-secret lists are inconsistent — see errors above." >&2
fi
exit "$status"
