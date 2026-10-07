#!/usr/bin/env python3
"""
Check a diff-scoped `gremlins unleash` report against the lines changed vs
origin/main; see `api/Makefile`'s `test/mutation/diff` target.

- SKIPPED on a changed line fails: gremlins matches its diff paths to mutant
  paths verbatim, so a path mismatch skips every mutant silently.
- NOT COVERED on a changed line is listed as a warning: gremlins only runs a
  package's own tests, so code tested solely through app-root integration
  tests (repositories, most services) is never mutation-tested.

Usage:
    python3 ../tools/check_mutation_report_go.py <package dir> <report.json>

Run with cwd set to api/. <package dir> is the path gremlins ran on (as
printed by diff_packages_go.py); <report.json> is its `--output` file.
Untracked files are ignored: git diff omits them, so gremlins can't see them.
"""

import json
import os
import sys

sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))

from diff_coverage_go import ALL_LINES, get_changed_lines, run_git  # noqa: E402


def changed_mutants(pkg, report, changed_lines, status):
    """Returns "path:line:col TYPE" for each mutant with status on a changed
    line."""
    found = []
    for file in report.get('files') or []:
        path = os.path.normpath(os.path.join(pkg, file['file_name']))
        lines = changed_lines.get(path)
        if not lines or lines == ALL_LINES:
            continue
        for m in file['mutations']:
            if m['status'] == status and m['line'] in lines:
                found.append(f"{path}:{m['line']}:{m['column']} {m['type']}")
    return found


def main():
    pkg, report_path = sys.argv[1], sys.argv[2]
    project_dir = os.getcwd()

    repo_root = run_git(['rev-parse', '--show-toplevel'], project_dir).strip()
    project_rel = os.path.relpath(project_dir, repo_root)
    changed_lines = get_changed_lines(repo_root, project_rel)

    with open(report_path) as f:
        report = json.load(f)

    uncovered = changed_mutants(pkg, report, changed_lines, 'NOT COVERED')
    if uncovered:
        print(
            f'warning: {len(uncovered)} mutant(s) on changed lines in {pkg} '
            'have no same-package test and were not run:'
        )
        for u in uncovered:
            print(f'  {u}')

    skipped = changed_mutants(pkg, report, changed_lines, 'SKIPPED')
    if skipped:
        print(
            f'gremlins SKIPPED {len(skipped)} mutant(s) on changed lines in '
            f'{pkg}; its diff paths no longer match its mutant paths:',
            file=sys.stderr,
        )
        for s in skipped:
            print(f'  {s}', file=sys.stderr)
        sys.exit(1)


if __name__ == '__main__':
    main()
