#!/usr/bin/env python3
"""
Report line/branch coverage for files changed vs origin/main, scoped from
an lcov.info report, mirroring what CI's codecov/patch check gates on.

Usage:
    python3 ../tools/diff_coverage.py coverage/lcov.info

Run with cwd set to the project directory the lcov paths are relative to
(e.g. web/), after a coverage run has produced that lcov.info file.
"""

import os
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


def get_changed_files(repo_root, project_dir):
    merge_base_out = run_git(
        ['merge-base', 'origin/main', 'HEAD'], repo_root
    )
    base_ref = merge_base_out.strip() if merge_base_out else 'origin/main'

    diff_out = run_git(['diff', '--name-only', base_ref], repo_root) or ''
    untracked_out = run_git(
        ['ls-files', '--others', '--exclude-standard'], repo_root
    ) or ''

    all_files = set(diff_out.splitlines()) | set(untracked_out.splitlines())

    project_prefix = project_dir.rstrip('/') + '/'
    relevant = []
    for f in all_files:
        if not f.startswith(project_prefix):
            continue
        rel = f[len(project_prefix):]
        if is_relevant(rel):
            relevant.append(rel)

    return sorted(relevant)


def parse_lcov(lcov_path):
    files = {}
    current = None

    with open(lcov_path, 'r') as f:
        for line in f:
            line = line.strip()
            if line.startswith('SF:'):
                current = line[3:]
                files[current] = {'lines': [], 'branches': []}
            elif line.startswith('DA:') and current:
                _, hits = line[3:].split(',', 1)
                files[current]['lines'].append(int(hits))
            elif line.startswith('BRDA:') and current:
                parts = line[5:].split(',')
                hits = parts[3]
                files[current]['branches'].append(0 if hits == '-' else int(hits))
            elif line == 'end_of_record':
                current = None

    return files


def pct(covered, total):
    if total == 0:
        return None
    return (covered / total) * 100.0


def main():
    if len(sys.argv) != 2:
        print('Usage: python3 diff_coverage.py <path-to-lcov.info>', file=sys.stderr)
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
    changed_files = get_changed_files(repo_root, project_rel)

    if not changed_files:
        print('No coverage-relevant files changed.')
        sys.exit(0)

    lcov_files = parse_lcov(lcov_arg)

    flagged = 0
    print(
        f'Diff coverage check (changed files vs origin/main, threshold {THRESHOLD}%)\n'
    )

    for path in changed_files:
        data = lcov_files.get(path)
        if data is None:
            print(f'  ✗ {path:<50} no coverage data found')
            flagged += 1
            continue

        line_pct = pct(sum(1 for h in data['lines'] if h > 0), len(data['lines']))
        branch_pct = pct(
            sum(1 for h in data['branches'] if h > 0), len(data['branches'])
        )

        line_str = f'{line_pct:5.1f}% lines' if line_pct is not None else '  --  lines'
        branch_str = (
            f'{branch_pct:5.1f}% branches' if branch_pct is not None else '  --  branches'
        )

        is_flagged = (
            line_pct is None
            or line_pct < THRESHOLD
            or (branch_pct is not None and branch_pct < THRESHOLD)
        )
        mark = '✗' if is_flagged else '✓'
        if is_flagged:
            flagged += 1

        print(f'  {mark} {path:<50} {line_str}, {branch_str}')

    print()
    if flagged:
        print(f'{flagged}/{len(changed_files)} file(s) below {THRESHOLD}% threshold.')
        sys.exit(1)

    print(f'All {len(changed_files)} changed file(s) meet the {THRESHOLD}% threshold.')
    sys.exit(0)


if __name__ == '__main__':
    main()
