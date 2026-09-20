#!/usr/bin/env bash
# Validate that every GitHub Actions workflow file parses as YAML before
# merge (issue #1791 — a session-retro finding: two separate sessions
# independently ran the same ad-hoc `python3 -c "import yaml; ..."` snippet
# to sanity-check a main.yml edit, because no build-tool target existed for
# it despite the same pattern already covering infra/prometheus.yml
# (lint/infra) and the Grafana dashboards (lint/grafana).
#
# Run via `make lint/workflows`; also run by main.yml's own workflow-lint job.
set -euo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
workflows_dir="$repo_root/.github/workflows"

status=0
for file in "$workflows_dir"/*.yml "$workflows_dir"/*.yaml; do
	[ -e "$file" ] || continue
	if ! error=$(python3 -c "
import sys, yaml
try:
    yaml.safe_load(open(sys.argv[1]))
except yaml.YAMLError as e:
    sys.exit(str(e))
" "$file" 2>&1); then
		echo "invalid YAML: $file" >&2
		echo "$error" >&2
		status=1
	fi
done

if [ "$status" -ne 0 ]; then
	exit 1
fi

echo "ok: every .github/workflows/*.yml parses as YAML"
