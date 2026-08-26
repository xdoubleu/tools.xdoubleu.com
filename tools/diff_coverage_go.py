#!/usr/bin/env python3
"""
Report line coverage for api/ Go files changed vs origin/main, scoped to
the changed lines only, from a `go tool cover` profile — approximating
what CI's codecov/patch check gates on so a locally-missed branch shows up
before push instead of after a CI round trip. See diff_coverage_ts.py for
the equivalent over web/'s lcov coverage reports.

Usage:
    python3 ../tools/diff_coverage_go.py coverage.out

Run with cwd set to api/ (the profile's file paths are module-relative,
e.g. tools.xdoubleu.com/internal/foo/bar.go), after `make test/cov/report`
has produced that coverage.out file.
"""

import os
import re
import subprocess
import sys

THRESHOLD = 80
MODULE_PREFIX = 'tools.xdoubleu.com/'

# Coverage.out itself already excludes _mock.go and /gen/ (see
# api/Makefile's test/cov/report), so nothing extra to filter there beyond
# skipping test files, which are never instrumented as coverage targets.


def run_git(args, cwd):
    result = subprocess.run(['git'] + args, cwd=cwd, capture_output=True, text=True)
    if result.returncode != 0:
        return None
    return result.stdout


def is_relevant(path):
    return path.endswith('.go') and not path.endswith('_test.go')


def get_changed_lines(repo_root, project_dir):
    """Returns {relative_path: set(changed_line_numbers)} for api/*.go files
    changed vs origin/main, based on unified diff hunk headers."""
    merge_base_out = run_git(['merge-base', 'origin/main', 'HEAD'], repo_root)
    base_ref = merge_base_out.strip() if merge_base_out else 'origin/main'

    diff_out = run_git(['diff', '-U0', base_ref, '--', project_dir], repo_root) or ''

    changed = {}
    current_file = None
    hunk_re = re.compile(r'^@@ -\d+(?:,\d+)? \+(\d+)(?:,(\d+))? @@')

    for line in diff_out.splitlines():
        if line.startswith('+++ b/'):
            f = line[len('+++ b/'):]
            project_prefix = project_dir.rstrip('/') + '/'
            current_file = (
                f[len(project_prefix):] if f.startswith(project_prefix) else None
            )
            continue
        match = hunk_re.match(line)
        if match and current_file and is_relevant(current_file):
            start = int(match.group(1))
            count = int(match.group(2)) if match.group(2) is not None else 1
            if count == 0:
                continue
            changed.setdefault(current_file, set()).update(
                range(start, start + count)
            )

    return changed


def parse_profile(profile_path):
    """Returns {relative_path: {line_number: max_count}}."""
    files = {}
    block_re = re.compile(
        r'^(\S+):(\d+)\.\d+,(\d+)\.(\d+) \d+ (\d+)$'
    )

    with open(profile_path, 'r') as f:
        next(f, None)  # skip "mode: ..." header
        for line in f:
            match = block_re.match(line.strip())
            if not match:
                continue
            module_path, start_line, end_line, end_col, count = match.groups()
            if not module_path.startswith(MODULE_PREFIX):
                continue
            rel_path = module_path[len(MODULE_PREFIX):]
            start_line, end_line, end_col, count = (
                int(start_line), int(end_line), int(end_col), int(count)
            )
            # A block ending at column 1 doesn't actually reach end_line.
            last_line = end_line - 1 if end_col == 1 else end_line

            file_lines = files.setdefault(rel_path, {})
            for line_no in range(start_line, last_line + 1):
                file_lines[line_no] = max(file_lines.get(line_no, 0), count)

    return files


def main():
    if len(sys.argv) != 2:
        print('Usage: python3 diff_coverage_go.py <path-to-coverage.out>', file=sys.stderr)
        sys.exit(1)

    profile_arg = sys.argv[1]
    project_dir = os.getcwd()

    if not os.path.exists(profile_arg):
        print(
            f'{profile_arg} not found -- run `make test/cov/report` first.',
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

    profile = parse_profile(profile_arg)

    flagged = 0
    print(
        f'Diff coverage check (changed lines vs origin/main, threshold {THRESHOLD}%)\n'
    )

    for path in sorted(changed_lines):
        file_lines = profile.get(path)
        added_lines = changed_lines[path]

        # Only lines go tool cover actually instrumented (statement lines)
        # count toward the denominator -- blank lines, comments, braces
        # never appear in the profile at all.
        instrumented = {ln: file_lines[ln] for ln in added_lines if file_lines and ln in file_lines}

        if not instrumented:
            print(f'  -  {path:<60} no instrumented lines changed')
            continue

        covered = sum(1 for count in instrumented.values() if count > 0)
        total = len(instrumented)
        line_pct = (covered / total) * 100.0

        is_flagged = line_pct < THRESHOLD
        mark = '✗' if is_flagged else '✓'
        if is_flagged:
            flagged += 1

        print(f'  {mark} {path:<60} {line_pct:5.1f}% of {total} changed lines')

    print()
    if flagged:
        print(f'{flagged} file(s) below {THRESHOLD}% threshold on changed lines.')
        sys.exit(1)

    print(f'All changed files meet the {THRESHOLD}% threshold on changed lines.')
    sys.exit(0)


if __name__ == '__main__':
    main()
