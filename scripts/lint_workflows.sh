#!/usr/bin/env bash
# Validate that every .github/workflows/*.yml parses as YAML. Ruby's stdlib
# Psych is used because it ships with Ruby on macOS and the CI runners, where
# python3 doesn't always have PyYAML.
set -euo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
workflows_dir="$repo_root/.github/workflows"

status=0
for file in "$workflows_dir"/*.yml "$workflows_dir"/*.yaml; do
	[ -e "$file" ] || continue
	if ! error=$(ruby -ryaml -e '
begin
  YAML.parse_file(ARGV[0])
rescue Psych::SyntaxError => e
  warn e.message
  exit 1
end
' "$file" 2>&1); then
		echo "invalid YAML: $file" >&2
		echo "$error" >&2
		status=1
	fi
done

if [ "$status" -ne 0 ]; then
	exit 1
fi

echo "ok: every .github/workflows/*.yml parses as YAML"
