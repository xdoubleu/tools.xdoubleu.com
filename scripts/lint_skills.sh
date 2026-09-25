#!/usr/bin/env bash
# Validate SKILL.md frontmatter in .claude/skills, .agents/skills and
# .opencode/skills: must parse as YAML with `name` and a non-empty
# `description`, or harnesses silently drop the skill.
set -euo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"

status=0
checked=0
for dir in "$repo_root/.claude/skills" "$repo_root/.agents/skills" "$repo_root/.opencode/skills"; do
	[ -d "$dir" ] || continue
	while IFS= read -r file; do
		checked=$((checked + 1))
		if ! error=$(python3 -c "
import sys, yaml

path = sys.argv[1]
text = open(path, encoding='utf-8').read()
parts = text.split('---', 2)
if len(parts) < 3 or parts[0].strip():
    sys.exit('missing YAML frontmatter (file must start with a --- fence)')
try:
    fm = yaml.safe_load(parts[1])
except yaml.YAMLError as e:
    sys.exit(f'frontmatter is not valid YAML: {e}')
if not isinstance(fm, dict):
    sys.exit('frontmatter is not a YAML mapping')
if not fm.get('name'):
    sys.exit('frontmatter has no non-empty name')
if not fm.get('description'):
    sys.exit('frontmatter has no non-empty description (skill would never be advertised)')
" "$file" 2>&1); then
			echo "invalid skill: ${file#"$repo_root"/}" >&2
			echo "  $error" >&2
			status=1
		fi
	done < <(find "$dir" -name SKILL.md -type f 2>/dev/null)
done

if [ "$status" -ne 0 ]; then
	exit 1
fi

echo "ok: $checked SKILL.md frontmatters parse as YAML with name + description"
