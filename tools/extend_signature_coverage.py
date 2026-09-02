#!/usr/bin/env python3
"""
Rewrite a `go tool cover` profile so a function's body block also spans the
lines of its signature.

Go's cover tool opens a function's first block at the opening brace of the
body, not at the `func` keyword. When `golines` wraps a signature over the
repo's 88-character limit, the parameter lines belong to no coverage block
at all -- Codecov's parser reports them as missed and the `) ... {` line as
partial, so every newly added function forfeits 2-3 patch lines it can
never cover (issue #1376: `codecov/patch` reported 71.42% on PR #1375 for
code both `make test/cov/diff` and a local replay of CI's own pipeline
measured as 100% covered).

This moves the start of each such block up to its `func` keyword, which is
what `diff_coverage_go.py` effectively already assumes: the signature lines
carry the same count as the body.

Usage (cwd must be api/, whose sources the profile's module-relative paths
resolve against):

    python3 ../tools/extend_signature_coverage.py coverage.out
"""

import re
import sys

MODULE_PREFIX = 'tools.xdoubleu.com/'

BLOCK_RE = re.compile(r'^(\S+):(\d+)\.(\d+),(\d+)\.(\d+) (\d+) (\d+)$')
FUNC_RE = re.compile(r'^func\b', re.MULTILINE)


def blank_comments_and_literals(src):
    """Return src with comment and string/rune literal bodies replaced by
    spaces, so a naive bracket scan can't be thrown off by a brace inside
    one. Newlines are preserved, so line numbers still line up."""
    out = []
    i, n = 0, len(src)
    while i < n:
        char = src[i]
        two = src[i:i + 2]
        if two == '//':
            while i < n and src[i] != '\n':
                out.append(' ')
                i += 1
        elif two == '/*':
            while i < n and src[i:i + 2] != '*/':
                out.append('\n' if src[i] == '\n' else ' ')
                i += 1
            out.append('  ')
            i += 2
        elif char in '"\'`':
            quote = char
            out.append(' ')
            i += 1
            while i < n and src[i] != quote:
                if quote != '`' and src[i] == '\\':
                    out.append(' ')
                    i += 1
                if i < n:
                    out.append('\n' if src[i] == '\n' else ' ')
                    i += 1
            out.append(' ')
            i += 1
        else:
            out.append(char)
            i += 1
    return ''.join(out)


def signature_starts(src):
    """Return {body_brace_line: func_keyword_line} for every top-level
    function whose signature spans more than one line."""
    cleaned = blank_comments_and_literals(src)
    line_of = [0] * (len(cleaned) + 1)
    line = 1
    for idx, char in enumerate(cleaned):
        line_of[idx] = line
        if char == '\n':
            line += 1
    line_of[len(cleaned)] = line

    starts = {}
    for match in FUNC_RE.finditer(cleaned):
        func_line = line_of[match.start()]
        depth = 0
        i = match.end()
        while i < len(cleaned):
            char = cleaned[i]
            if char in '([':
                depth += 1
            elif char in ')]':
                depth -= 1
            elif char == '{':
                if depth == 0:
                    brace_line = line_of[i]
                    if brace_line > func_line:
                        starts.setdefault(brace_line, func_line)
                break
            elif char == '\n' and depth == 0 and cleaned[i + 1:i + 5] == 'func':
                break  # a body-less declaration (e.g. an assembly stub)
            i += 1
    return starts


def rewrite(profile_path):
    with open(profile_path) as handle:
        lines = handle.read().splitlines()

    cache = {}
    out = []
    for line in lines:
        match = BLOCK_RE.match(line)
        if not match:
            out.append(line)
            continue
        path, start_line, start_col, end_line, end_col, stmts, count = match.groups()
        if path not in cache:
            cache[path] = {}
            if path.startswith(MODULE_PREFIX):
                try:
                    with open(path[len(MODULE_PREFIX):]) as src:
                        cache[path] = signature_starts(src.read())
                except OSError:
                    pass
        func_line = cache[path].get(int(start_line))
        if func_line is not None:
            start_line, start_col = str(func_line), '1'
        out.append(
            f'{path}:{start_line}.{start_col},{end_line}.{end_col} {stmts} {count}'
        )

    with open(profile_path, 'w') as handle:
        handle.write('\n'.join(out) + '\n')


def main():
    if len(sys.argv) != 2:
        print(
            'Usage: python3 extend_signature_coverage.py <path-to-coverage.out>',
            file=sys.stderr,
        )
        sys.exit(1)
    rewrite(sys.argv[1])


if __name__ == '__main__':
    main()
