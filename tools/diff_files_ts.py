#!/usr/bin/env python3
"""
Resolve the web/ TS/TSX lines changed vs origin/main, for scoping a
diff-only StrykerJS `--mutate` run (see `web/scripts/test-mutation-diff.sh`).
Reuses `diff_coverage_ts.py`'s `get_changed_lines`.

Usage:
    python3 ../tools/diff_files_ts.py

Run with cwd set to web/. Prints one StrykerJS mutate entry per line: a bare
path for a new file (e.g. "components/foo/Bar.tsx"), else one
"path:start-end" per contiguous changed range. Prints nothing (and exits 0)
when no relevant file changed vs origin/main.
"""

import os
import sys

sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))

from diff_coverage_ts import ALL_LINES, get_changed_lines, run_git  # noqa: E402


def ranges(lines):
    """Collapses line numbers into sorted inclusive (start, end) runs."""
    runs = []
    for n in sorted(lines):
        if runs and n == runs[-1][1] + 1:
            runs[-1][1] = n
        else:
            runs.append([n, n])
    return [tuple(r) for r in runs]


def mutate_entries(changed_lines):
    entries = []
    for path in sorted(changed_lines):
        lines = changed_lines[path]
        if lines == ALL_LINES:
            entries.append(path)
            continue
        entries.extend(f'{path}:{start}-{end}' for start, end in ranges(lines))
    return entries


def main():
    project_dir = os.getcwd()

    repo_root_out = run_git(['rev-parse', '--show-toplevel'], project_dir)
    if not repo_root_out:
        print('Not a git repository.', file=sys.stderr)
        sys.exit(1)
    repo_root = repo_root_out.strip()

    project_rel = os.path.relpath(project_dir, repo_root)
    for entry in mutate_entries(get_changed_lines(repo_root, project_rel)):
        print(entry)


if __name__ == '__main__':
    main()
