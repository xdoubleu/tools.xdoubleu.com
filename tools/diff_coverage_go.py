#!/usr/bin/env python3
"""
Report line coverage for api/ Go files changed vs origin/main, scoped to
the changed lines only, from a `go tool cover` profile — a first
approximation of what CI's codecov/patch check gates on. See
diff_coverage_ts.py for the equivalent over web/'s lcov coverage reports.

Codecov's exact patch accounting cannot be replicated locally (issue
#1868: it false-greened twice on PR #1861): Codecov counts an
unexplained subset of the diff's changed lines and marks lines partial
whose every covering block was hit locally. Two numbers are therefore
reported per file:

- the primary percentage — the pass/fail gate — counts a changed line
  covered when every block touching it was hit (go-cover line
  semantics, partial lines get no credit);
- a conservative secondary percentage counts a changed line covered
  only when it is the start or end line of a >0-count block with no
  0-count block touching it, treating interior lines of hit blocks as
  misses. On #1861's pushes this boundary-only view is what matched
  Codecov's hit attribution, so a big gap between the two numbers is a
  hint the local result is optimistic — not a prediction of Codecov's
  number.

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

# coverage.out itself already excludes _mock.go and /gen/ (see api/Makefile's
# test/cov/report), but is_relevant() below filters them too -- it's also
# reused by diff_packages_go.py, which reads the raw git diff directly
# rather than a pre-filtered coverage.out, so without this a generated
# proto file changed alongside real source (any RPC field addition) gets
# handed to `gremlins unleash`, which can't gather coverage on the gen/
# package at all.


def run_git(args, cwd):
    result = subprocess.run(['git'] + args, cwd=cwd, capture_output=True, text=True)
    if result.returncode != 0:
        return None
    return result.stdout


def is_relevant(path):
    if not path.endswith('.go') or path.endswith('_test.go'):
        return False
    if path.endswith('_mock.go') or '/gen/' in path or path.startswith('gen/'):
        return False
    return True


# ALL_LINES marks a file whose every instrumented line counts as changed --
# an untracked new file is new in its entirety, so there's no diff hunk to
# scope to. git diff omits untracked files, so without this a `make
# test/cov/diff` run before `git add` silently skips brand-new source files
# and passes, while codecov/patch (which sees them) later fails.
ALL_LINES = 'all'


def get_changed_lines(repo_root, project_dir):
    """Returns {relative_path: set(changed_line_numbers) | ALL_LINES} for
    api/*.go files changed vs origin/main, based on unified diff hunk
    headers, plus untracked new .go files scored in full."""
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

    project_prefix = project_dir.rstrip('/') + '/'
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


def parse_profile(profile_path):
    """Returns {relative_path: {line_number: (hit, miss, boundary)}}.
    hit/miss each report whether some block touching that line had
    count>0 / count==0 respectively. A physical line can host more than
    one Go coverage block (e.g. an `if err != nil {` condition plus its
    body) that disagree on hit/miss -- Codecov's own patch check marks
    that line "partial" and gives it no coverage credit, so callers must
    require hit and not miss for a line to count as covered, rather than
    a plain max(count).

    boundary reports whether some count>0 block starts or ends on the
    line. Interior lines of a fully-hit block carry hit=True yet Codecov
    still marked them partial on PR #1861, while its counted hits were
    consistently block start/end lines -- the conservative secondary
    percentage in main() uses this to flag local results that look
    better than Codecov's accounting would produce."""
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
                hit, miss, boundary = file_lines.get(line_no, (False, False, False))
                if count > 0:
                    hit = True
                    # Start/end lines of a hit block are what Codecov's
                    # counted hits matched on PR #1861; interior lines
                    # only. A single-line block is both start and end.
                    if line_no == start_line or line_no == last_line:
                        boundary = True
                else:
                    miss = True
                file_lines[line_no] = (hit, miss, boundary)

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
        if added_lines is ALL_LINES:
            instrumented = dict(file_lines) if file_lines else {}
        else:
            instrumented = {
                ln: file_lines[ln]
                for ln in added_lines
                if file_lines and ln in file_lines
            }

        if not instrumented:
            print(f'  -  {path:<60} no instrumented lines changed')
            continue

        # A line counts as covered only when every block touching it was
        # hit -- a line with both a hit and a miss block is "partial" in
        # Codecov's own accounting and gets no credit there either.
        covered = sum(
            1 for hit, miss, _ in instrumented.values() if hit and not miss
        )
        # Conservative view: only block start/end lines of hit blocks
        # count, mirroring what matched Codecov's hit attribution on
        # issue #1868's PR #1861 investigation. Interior lines of hit
        # blocks, which Codecov marked partial there, count as misses.
        conservative = sum(
            1 for hit, miss, boundary in instrumented.values()
            if hit and not miss and boundary
        )
        total = len(instrumented)
        line_pct = (covered / total) * 100.0
        cons_pct = (conservative / total) * 100.0

        is_flagged = line_pct < THRESHOLD
        mark = '✗' if is_flagged else '✓'
        if is_flagged:
            flagged += 1

        print(
            f'  {mark} {path:<60} {line_pct:5.1f}% of {total} changed lines'
            f'  (conservative: {cons_pct:5.1f}%)'
        )

    print()
    print(
        'The gate uses the primary percentage; the conservative percentage\n'
        'counts only block start/end lines of hit blocks (issue #1868).\n'
        'On validated PRs (#1869, #1863) Codecov\'s reported patch coverage\n'
        'fell between the two, so a large gap between them means the local\n'
        'result is optimistic -- but neither predicts Codecov\'s number.\n'
    )
    if flagged:
        print(f'{flagged} file(s) below {THRESHOLD}% threshold on changed lines.')
        sys.exit(1)

    print(f'All changed files meet the {THRESHOLD}% threshold on changed lines.')
    sys.exit(0)


if __name__ == '__main__':
    main()
