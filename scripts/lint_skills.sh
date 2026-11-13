#!/usr/bin/env bash
# Validate SKILL.md frontmatter in .claude/skills, .agents/skills and
# .opencode/skills: must parse as YAML with `name` and a non-empty
# `description`, or harnesses silently drop the skill. Ruby's stdlib Psych is
# used because it ships with Ruby on macOS and the CI runners, where python3
# doesn't always have PyYAML.
set -euo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"

status=0
checked=0
for dir in "$repo_root/.claude/skills" "$repo_root/.agents/skills" "$repo_root/.opencode/skills"; do
	[ -d "$dir" ] || continue
	while IFS= read -r file; do
		checked=$((checked + 1))
		if ! error=$(ruby -ryaml -e '
path = ARGV[0]
text = File.read(path, encoding: "UTF-8")
parts = text.split("---", 3)
if parts.length < 3 or not parts[0].strip.empty?
  warn "missing YAML frontmatter (file must start with a --- fence)"
  exit 1
end
begin
  fm = YAML.safe_load(parts[1])
rescue Psych::SyntaxError => e
  warn "frontmatter is not valid YAML: #{e.message}"
  exit 1
end
unless fm.is_a?(Hash)
  warn "frontmatter is not a YAML mapping"
  exit 1
end
if fm["name"].to_s.empty?
  warn "frontmatter has no non-empty name"
  exit 1
end
if fm["description"].to_s.empty?
  warn "frontmatter has no non-empty description (skill would never be advertised)"
  exit 1
end
' "$file" 2>&1); then
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
