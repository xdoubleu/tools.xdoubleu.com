#!/usr/bin/env python3
"""
Resolve the set of api/ Go package directories changed vs origin/main, for
scoping a diff-only `gremlins unleash` mutation-testing run (see
`api/Makefile`'s `test/mutation/diff` target and issue #1632). Reuses
`diff_coverage_go.py`'s `get_changed_lines` -- the same
git-diff-against-origin/main mechanism adr-0013's diff-scoped coverage
already established -- rather than writing a second implementation of
"which files/packages changed".

Usage:
    python3 ../tools/diff_packages_go.py

Run with cwd set to api/. Prints one changed package directory per line
(e.g. "./apps/recipes/internal/services"), suitable for
`gremlins unleash $(python3 ../tools/diff_packages_go.py)`. Prints nothing
(and exits 0) when no Go package changed vs origin/main.
"""

import os
import sys

sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))

from diff_coverage_go import get_changed_lines, run_git  # noqa: E402


def main():
    project_dir = os.getcwd()

    repo_root_out = run_git(['rev-parse', '--show-toplevel'], project_dir)
    if not repo_root_out:
        print('Not a git repository.', file=sys.stderr)
        sys.exit(1)
    repo_root = repo_root_out.strip()

    project_rel = os.path.relpath(project_dir, repo_root)
    changed_lines = get_changed_lines(repo_root, project_rel)

    dirs = set()
    for path in changed_lines:
        d = os.path.dirname(path)
        dirs.add('./' + d if d else '.')

    for d in sorted(dirs):
        print(d)


if __name__ == '__main__':
    main()
