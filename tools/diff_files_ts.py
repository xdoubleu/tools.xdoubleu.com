#!/usr/bin/env python3
"""
Resolve the set of web/ TS/TSX files changed vs origin/main, for scoping a
diff-only StrykerJS mutation-testing run (see `web/scripts/test-mutation-
diff.sh` and issue #1632). Reuses `diff_coverage_ts.py`'s
`get_changed_lines` -- the same git-diff-against-origin/main mechanism
adr-0013's diff-scoped coverage already established -- rather than writing
a second implementation of "which files changed". StrykerJS's own CLI has
no built-in git-diff-based scoping (`--since` doesn't exist in the
installed version; `--incremental` only speeds up reruns of an unchanged
set), so `--mutate` is populated from this script's output instead.

Usage:
    python3 ../tools/diff_files_ts.py

Run with cwd set to web/. Prints one changed, coverage-relevant file path
per line (e.g. "components/foo/Bar.tsx"). Prints nothing (and exits 0) when
no relevant file changed vs origin/main.
"""

import os
import sys

sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))

from diff_coverage_ts import get_changed_lines, run_git  # noqa: E402


def main():
    project_dir = os.getcwd()

    repo_root_out = run_git(['rev-parse', '--show-toplevel'], project_dir)
    if not repo_root_out:
        print('Not a git repository.', file=sys.stderr)
        sys.exit(1)
    repo_root = repo_root_out.strip()

    project_rel = os.path.relpath(project_dir, repo_root)
    changed_lines = get_changed_lines(repo_root, project_rel)

    for path in sorted(changed_lines):
        print(path)


if __name__ == '__main__':
    main()
