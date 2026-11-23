"""
Changed-lines helper shared by the mutation-diff scripts: the lines changed
vs the merge base with origin/main, from `git diff -U0` hunk headers, plus
untracked new files in full.
"""

import re
import subprocess

# ALL_LINES marks a file new in its entirety (untracked: git diff omits it).
ALL_LINES = 'all'

HUNK_RE = re.compile(r'^@@ -\d+(?:,\d+)? \+(\d+)(?:,(\d+))? @@')


def run_git(args, cwd):
    result = subprocess.run(['git'] + args, cwd=cwd, capture_output=True, text=True)
    if result.returncode != 0:
        return None
    return result.stdout


def get_changed_lines(repo_root, project_dir, is_relevant):
    """Returns {path relative to project_dir: set(line numbers) | ALL_LINES}
    for files where is_relevant(path) holds."""
    merge_base = run_git(['merge-base', 'origin/main', 'HEAD'], repo_root)
    base_ref = merge_base.strip() if merge_base else 'origin/main'
    prefix = project_dir.rstrip('/') + '/'
    if prefix == './':
        prefix = ''

    diff_out = run_git(['diff', '-U0', base_ref, '--', project_dir], repo_root) or ''
    changed = {}
    current = None
    for line in diff_out.splitlines():
        if line.startswith('+++ b/'):
            f = line[len('+++ b/'):]
            current = f[len(prefix):] if f.startswith(prefix) else None
            continue
        match = HUNK_RE.match(line)
        if match and current and is_relevant(current):
            start = int(match.group(1))
            count = int(match.group(2)) if match.group(2) is not None else 1
            if count:
                changed.setdefault(current, set()).update(range(start, start + count))

    untracked = run_git(['ls-files', '--others', '--exclude-standard'], repo_root) or ''
    for f in untracked.splitlines():
        if f.startswith(prefix) and is_relevant(f[len(prefix):]):
            changed[f[len(prefix):]] = ALL_LINES
    return changed


def is_go_source(path):
    if not path.endswith('.go') or path.endswith(('_test.go', '_mock.go')):
        return False
    return '/gen/' not in path and not path.startswith('gen/')


def is_ts_source(path):
    if not path.endswith(('.ts', '.tsx')) or path.endswith('.d.ts'):
        return False
    if not path.startswith(('components/', 'lib/', 'hooks/', 'app/', 'src/')):
        return False
    return not path.startswith('lib/gen/') and path != 'src/main.tsx'
