#!/usr/bin/env python3
"""
Report line/branch coverage for web/ TS/TSX files changed vs origin/main,
scoped to the changed lines only, from an lcov.info report -- mirroring
what CI's codecov/patch check gates on so a locally-missed branch shows up
before push instead of after a CI round trip. See diff_coverage_go.py for
the equivalent over api/'s Go coverage profiles.

Scoping to changed lines is what keeps this honest: scoring a changed
file's entire line set instead would fail a one-line edit to a file with
pre-existing gaps, while codecov/patch -- which only ever looks at the
diff -- passes it (issue #1301).

Usage:
    python3 ../tools/diff_coverage_ts.py coverage/lcov.info

Run with cwd set to the project directory the lcov paths are relative to
(e.g. web/), after a coverage run has produced that lcov.info file.
"""

import os
import re
import subprocess
import sys

THRESHOLD = 80

# Mirrors web/jest.config.js's collectCoverageFrom allowlist/excludes.
INCLUDE_DIRS = ('components/', 'lib/', 'hooks/', 'app/')
INCLUDE_SINGLE_FILES = ('instrumentation-client.ts',)
EXCLUDE_PREFIXES = ('lib/gen/',)
EXCLUDE_EXACT = ('app/manifest.ts', 'app/layout.tsx')
EXCLUDE_SUFFIXES = ('/apple-icon.tsx', '/icon.tsx', '.d.ts')


def run_git(args, cwd):
    result = subprocess.run(
        ['git'] + args, cwd=cwd, capture_output=True, text=True
    )
    if result.returncode != 0:
        return None
    return result.stdout


def is_relevant(path):
    if not (path.endswith('.ts') or path.endswith('.tsx')):
        return False

    included = path in INCLUDE_SINGLE_FILES or path.startswith(INCLUDE_DIRS)
    if not included:
        return False

    if path.startswith(EXCLUDE_PREFIXES):
        return False
    if path in EXCLUDE_EXACT:
        return False
    if path.endswith(EXCLUDE_SUFFIXES):
        return False

    return True


# ALL_LINES marks a file whose every covered line counts as changed --
# an untracked file is new in its entirety, so there's no hunk to scope to.
ALL_LINES = 'all'


def get_changed_lines(repo_root, project_dir):
    """Returns {relative_path: set(changed_line_numbers) | ALL_LINES} for
    coverage-relevant web/ files changed vs origin/main, based on unified
    diff hunk headers."""
    merge_base_out = run_git(
        ['merge-base', 'origin/main', 'HEAD'], repo_root
    )
    base_ref = merge_base_out.strip() if merge_base_out else 'origin/main'

    project_prefix = project_dir.rstrip('/') + '/'
    diff_out = run_git(
        ['diff', '-U0', base_ref, '--', project_dir], repo_root
    ) or ''

    changed = {}
    current_file = None
    hunk_re = re.compile(r'^@@ -\d+(?:,\d+)? \+(\d+)(?:,(\d+))? @@')

    for line in diff_out.splitlines():
        if line.startswith('+++ b/'):
            f = line[len('+++ b/'):]
            current_file = (
                f[len(project_prefix):] if f.startswith(project_prefix) else None
            )
            continue
        match = hunk_re.match(line)
        if match and current_file and is_relevant(current_file):
            start = int(match.group(1))
            count = int(match.group(2)) if match.group(2) is not None else 1
            # A pure deletion hunk (+n,0) adds no lines to score.
            if count == 0:
                continue
            changed.setdefault(current_file, set()).update(
                range(start, start + count)
            )

    untracked_out = run_git(
        ['ls-files', '--others', '--exclude-standard'], repo_root
    ) or ''
    for f in untracked_out.splitlines():
        if not f.startswith(project_prefix):
            continue
        rel = f[len(project_prefix):]
        if is_relevant(rel):
            changed[rel] = ALL_LINES

    return changed


def parse_lcov(lcov_path):
    """Returns {relative_path: {'lines': {lineno: hits},
    'branches': {lineno: [hits, ...]}}}.

    Line numbers are kept so coverage can be scoped to the diff -- an
    lcov DA: record is `DA:<line>,<hits>` and a BRDA: record is
    `BRDA:<line>,<block>,<branch>,<taken>`, where taken is `-` when the
    branch was never reached at all.
    """
    files = {}
    current = None

    with open(lcov_path, 'r') as f:
        for line in f:
            line = line.strip()
            if line.startswith('SF:'):
                current = line[3:]
                files[current] = {'lines': {}, 'branches': {}}
            elif line.startswith('DA:') and current:
                lineno, hits = line[3:].split(',', 1)
                # Jest can emit the same line twice; keep the best hit count.
                lineno = int(lineno)
                hits = int(hits.split(',')[0])
                prev = files[current]['lines'].get(lineno, 0)
                files[current]['lines'][lineno] = max(prev, hits)
            elif line.startswith('BRDA:') and current:
                parts = line[5:].split(',')
                lineno = int(parts[0])
                hits = parts[3]
                files[current]['branches'].setdefault(lineno, []).append(
                    0 if hits == '-' else int(hits)
                )
            elif line == 'end_of_record':
                current = None

    return files


def pct(covered, total):
    if total == 0:
        return None
    return (covered / total) * 100.0


def main():
    if len(sys.argv) != 2:
        print('Usage: python3 diff_coverage_ts.py <path-to-lcov.info>', file=sys.stderr)
        sys.exit(1)

    lcov_arg = sys.argv[1]
    project_dir = os.getcwd()

    if not os.path.exists(lcov_arg):
        print(
            f'{lcov_arg} not found -- run `npm run test:cov` first.',
            file=sys.stderr,
        )
        sys.exit(1)

    repo_root_out = run_git(['rev-parse', '--show-toplevel'], project_dir)
    if not repo_root_out:
        print('Not a git repository.', file=sys.stderr)
        sys.exit(1)
    repo_root = repo_root_out.strip()

    project_rel = os.path.relpath(project_dir, repo_root)
    changed_lines = get_changed_lines(repo_root, project_rel)

    if not changed_lines:
        print('No coverage-relevant files changed.')
        sys.exit(0)

    lcov_files = parse_lcov(lcov_arg)

    flagged = 0
    print(
        f'Diff coverage check (changed lines vs origin/main, threshold {THRESHOLD}%)\n'
    )

    changed_files = sorted(changed_lines)
    for path in changed_files:
        data = lcov_files.get(path)
        if data is None:
            print(f'  ✗ {path:<50} no coverage data found')
            flagged += 1
            continue

        wanted = changed_lines[path]

        def in_diff(lineno):
            return wanted == ALL_LINES or lineno in wanted

        scoped_lines = [
            hits for lineno, hits in data['lines'].items() if in_diff(lineno)
        ]
        scoped_branches = [
            hits
            for lineno, hit_list in data['branches'].items()
            if in_diff(lineno)
            for hits in hit_list
        ]

        # Only lines Jest actually instrumented count toward the
        # denominator -- comments, imports, and type-only lines never get a
        # DA:/BRDA: record, so a diff touching just those scopes to nothing.
        if not scoped_lines and not scoped_branches:
            print(f'  -  {path:<50} no instrumented lines changed')
            continue

        line_pct = pct(sum(1 for h in scoped_lines if h > 0), len(scoped_lines))
        branch_pct = pct(
            sum(1 for h in scoped_branches if h > 0), len(scoped_branches)
        )

        line_str = f'{line_pct:5.1f}% lines' if line_pct is not None else '  --  lines'
        branch_str = (
            f'{branch_pct:5.1f}% branches' if branch_pct is not None else '  --  branches'
        )

        is_flagged = (
            (line_pct is not None and line_pct < THRESHOLD)
            or (branch_pct is not None and branch_pct < THRESHOLD)
        )
        mark = '✗' if is_flagged else '✓'
        if is_flagged:
            flagged += 1

        print(
            f'  {mark} {path:<50} {line_str}, {branch_str} '
            f'({len(scoped_lines)} changed lines)'
        )

    print()
    if flagged:
        print(
            f'{flagged}/{len(changed_files)} file(s) below {THRESHOLD}% '
            'threshold on changed lines.'
        )
        sys.exit(1)

    print(
        f'All {len(changed_files)} changed file(s) meet the {THRESHOLD}% '
        'threshold on changed lines.'
    )
    sys.exit(0)


if __name__ == '__main__':
    main()
