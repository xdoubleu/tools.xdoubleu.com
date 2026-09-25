#!/usr/bin/env bash
# Enforces docs/convention-concise-docs-and-comments.md:
#   1. word budgets for the instruction files agents load every session;
#   2. no added comment block longer than MAX_COMMENT_LINES, diff-scoped
#      against BASE_REF (default origin/main) so existing code doesn't fail.
# Usage: scripts/lint_docs.sh [files...] — with files, only checks budgets for those.
set -euo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$repo_root"

python3 - "$@" <<'PY'
import glob, os, re, subprocess, sys

MAX_COMMENT_LINES = 6
SKILL_BUDGET = 900
BUDGETS = {
    "AGENTS.md": 1300,
    "CLAUDE.md": 500,
    "api/AGENTS.md": 1800,
    "web/AGENTS.md": 800,
    "kobo-gateway/AGENTS.md": 600,
}
for skill in glob.glob(".claude/skills/*/SKILL.md"):
    BUDGETS.setdefault(skill, SKILL_BUDGET)

only = {os.path.relpath(os.path.abspath(f)) for f in sys.argv[1:]}
failed = False

for path, budget in sorted(BUDGETS.items()):
    if only and path not in only:
        continue
    if not os.path.exists(path):
        continue
    words = len(open(path, encoding="utf-8").read().split())
    if words > budget:
        failed = True
        print(f"{path}: {words} words, budget {budget} — say it once, don't overexplain")

if not only:
    base = os.environ.get("BASE_REF", "origin/main")
    try:
        merge_base = subprocess.check_output(
            ["git", "merge-base", base, "HEAD"], text=True, stderr=subprocess.DEVNULL
        ).strip()
    except subprocess.CalledProcessError:
        print(f"lint/docs: {base} not found, skipping comment check", file=sys.stderr)
        merge_base = None
    if merge_base:
        diff = subprocess.check_output(
            ["git", "diff", "-U0", merge_base, "--", "*.go", "*.ts", "*.tsx", "*.js",
             "*.mjs", "*.sh", "*.yml", "*.yaml", "*Makefile",
             ":!api/gen/**", ":!web/lib/gen/**", ":!**/mocks/**"],
            text=True,
        )
        comment = re.compile(r"^\s*(//|#(?!!)|/\*|\*(\s|/|$))")
        path = None
        run = 0
        start = 0
        line = 0

        def flush():
            global failed
            if run > MAX_COMMENT_LINES:
                failed = True
                print(f"{path}:{start}: {run}-line comment block (max {MAX_COMMENT_LINES}) — trim it")

        for raw in diff.splitlines():
            if raw.startswith("+++ "):
                flush(); run = 0
                path = raw[6:] if raw.startswith("+++ b/") else None
            elif raw.startswith("@@"):
                flush(); run = 0
                line = int(re.search(r"\+(\d+)", raw).group(1))
            elif raw.startswith("+") and path:
                if comment.match(raw[1:]):
                    if run == 0:
                        start = line
                    run += 1
                else:
                    flush(); run = 0
                line += 1
        flush()

if failed:
    sys.exit(1)
print("ok: doc budgets and comment blocks")
PY
