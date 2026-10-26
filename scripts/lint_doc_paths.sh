#!/usr/bin/env bash
# Fails when a backticked repo path in the instruction files or docs/*.md no
# longer exists. Checked: tokens under a known top-level dir, or ending in a
# known extension (suffix-matched, so subtree-relative and bare names work).
# Accepted: git-ignored paths. Skipped: fenced code, globs, placeholders, URLs,
# `- Affects (historical):` lines, and lines ending in `<!-- historical -->`.
# Usage: scripts/lint_doc_paths.sh [files...]
set -euo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$repo_root"

python3 - "$@" <<'PY'
import glob, os, re, subprocess, sys

TOP_DIRS = ("api", "web", "docs", "infra", "config", "scripts", "proto",
            "kobo-gateway", "sentrytools", "tools", ".github", ".claude",
            ".kamal", ".opencode", ".agents")
TOP_DIRS_PREFIX = tuple(d + "/" for d in TOP_DIRS)
EXTENSIONS = (".go", ".ts", ".tsx", ".js", ".mjs", ".json", ".md", ".yml",
              ".yaml", ".toml", ".sql", ".proto", ".sh", ".py", ".tf", ".css",
              ".html", ".mod")
INSTRUCTION_FILES = ["AGENTS.md", "CLAUDE.md", "api/AGENTS.md", "web/AGENTS.md",
                     "kobo-gateway/AGENTS.md"]
MARKER = "<!-- historical -->"
HISTORICAL_LINE = "- Affects (historical):"

# Tracked plus untracked-but-not-ignored, so the result doesn't depend on local
# build output and a file added in the same change counts.
listed = subprocess.check_output(
    ["git", "ls-files", "-z", "--cached", "--others", "--exclude-standard"],
    text=True,
).split("\0")
files = {p for p in listed if p and os.path.exists(p)}
dirs = {os.path.dirname(p) for p in files}
for d in list(dirs):
    while d:
        d = os.path.dirname(d)
        dirs.add(d)
dirs.discard("")

PATH_CHARS = re.compile(r"^[A-Za-z0-9._/@+~-]+$")
LINE_SUFFIX = re.compile(r":\d+(-\d+)?$")
SPAN = re.compile(r"`([^`\n]+)`")
BRACES = re.compile(r"\{([^{}]*,[^{}]*)\}")


def expand(token):
    # `web/lib/{a,b}.ts` is two paths; a brace group without a comma is a placeholder.
    m = BRACES.search(token)
    if not m:
        return [token]
    return [t for alt in m.group(1).split(",")
            for t in expand(token[:m.start()] + alt + token[m.end():])]


def candidates(raw):
    if "://" in raw:
        return []
    out = []
    for token in expand(LINE_SUFFIX.sub("", raw.strip())):
        token = token.rstrip("/")
        if not PATH_CHARS.match(token) or ".." in token.split("/"):
            continue
        if token in EXTENSIONS or token.startswith("/"):
            continue
        if token.startswith(TOP_DIRS_PREFIX) or token.endswith(EXTENSIONS):
            out.append(token)
    return out


def ignored(path):
    # The trailing slash lets directory-only patterns (`dist/`) match.
    return any(
        subprocess.run(["git", "check-ignore", "-q", p]).returncode == 0
        for p in (path, path + "/")
    )


def exists(token, doc):
    # A subtree doc (api/AGENTS.md) may write paths relative to its own dir.
    base = os.path.dirname(doc)
    for full in {token, os.path.normpath(os.path.join(base, token))}:
        if full in files or full in dirs or ignored(full):
            return True
    if token.startswith(TOP_DIRS_PREFIX):
        return False
    # Bare or subtree-relative name: any file ending in /<token>.
    return any(f.endswith("/" + token) for f in files)


targets = [os.path.relpath(os.path.abspath(f)) for f in sys.argv[1:]]
if not targets:
    targets = INSTRUCTION_FILES + sorted(glob.glob("docs/*.md"))

failed = False
for path in targets:
    if not os.path.exists(path):
        continue
    in_fence = False
    for n, line in enumerate(open(path, encoding="utf-8"), 1):
        if line.lstrip().startswith(("```", "~~~")):
            in_fence = not in_fence
            continue
        if in_fence or line.startswith(HISTORICAL_LINE) or line.rstrip().endswith(MARKER):
            continue
        for span in SPAN.findall(line):
            for token in candidates(span):
                if not exists(token, path):
                    failed = True
                    print(f"{path}:{n}: `{token}` does not exist — fix the "
                          f"reference or end the line with {MARKER}")

if failed:
    sys.exit(1)
print("ok: backticked paths exist")
PY
