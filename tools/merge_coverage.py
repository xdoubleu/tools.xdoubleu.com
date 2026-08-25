#!/usr/bin/env python3
"""
Merge multiple Go coverage profiles and compute combined coverage statistics.

Usage:
    python3 tools/merge_coverage.py cov_todos_pkg.out cov_backlog_pkg.out cov_publish_pkg.out

Each profile should be a Go coverage file (output from go test -coverprofile=...).
Merges all blocks, taking the MAX count for duplicate file:range keys.
Prints a summary table with per-package and combined coverage percentages.
"""

import sys
import os
from collections import defaultdict


def parse_coverage_file(filepath):
    """
    Parse a Go coverage profile and return a dict of {(file, start, end, numStmt): count}.
    Returns None if file doesn't exist.
    """
    if not os.path.exists(filepath):
        return None

    blocks = {}
    try:
        with open(filepath, 'r') as f:
            for line in f:
                line = line.strip()
                if not line or line.startswith('mode:'):
                    continue

                parts = line.split()
                if len(parts) != 3:
                    continue

                range_part = parts[0]
                num_stmt = parts[1]
                count = parts[2]

                # Skip mock files and generated stubs (mirrors test/cov/report filter)
                file_part = range_part.split(':')[0]
                if file_part.endswith('_mock.go') or '/gen/' in file_part:
                    continue

                try:
                    key = (range_part, num_stmt)
                    blocks[key] = int(count)
                except (ValueError, IndexError):
                    continue

    except IOError:
        return None

    return blocks


def label_from_filename(filepath):
    """
    Extract label by stripping common prefix 'cov_' and suffix '_pkg.out'.
    Example: 'cov_todos_pkg.out' -> 'todos'
    """
    filename = os.path.basename(filepath)

    if filename.startswith('cov_'):
        filename = filename[4:]
    if filename.endswith('_pkg.out'):
        filename = filename[:-8]

    return filename


def extract_package_from_range(range_part):
    """
    Extract the package name from a Go coverage range string.
    Format: "module.path/path/to/file.go:line.col,line.col"
    Examples:
      - tools.xdoubleu.com/apps/todos/... -> 'todos'
      - tools.xdoubleu.com/cmd/publish/... -> 'publish'
      - tools.xdoubleu.com/internal/... -> 'internal'
    """
    if ':' not in range_part:
        return 'unknown'

    file_part = range_part.split(':')[0]
    parts = file_part.split('/')

    # Skip module name (tools.xdoubleu.com) and find the next level
    for i, part in enumerate(parts):
        if part in ('apps', 'cmd'):
            return parts[i + 1] if i + 1 < len(parts) else 'unknown'
        elif part == 'internal' and i == 1:  # internal at top level
            return 'internal'

    return 'unknown'


def main():
    if len(sys.argv) < 2:
        print(
            'Usage: python3 tools/merge_coverage.py <coverage_file> [<coverage_file> ...]',
            file=sys.stderr
        )
        sys.exit(1)

    coverage_files = sys.argv[1:]
    all_blocks = {}
    any_valid = False

    for filepath in coverage_files:
        blocks = parse_coverage_file(filepath)
        if blocks is None:
            print(
                f'Warning: coverage file not found: {filepath}',
                file=sys.stderr
            )
            continue

        any_valid = True

        for key, count in blocks.items():
            if key not in all_blocks:
                all_blocks[key] = count
            else:
                all_blocks[key] = max(all_blocks[key], count)

    if not any_valid:
        print('Error: no valid coverage files found', file=sys.stderr)
        sys.exit(1)

    # Compute stats from merged blocks (each block counted exactly once)
    file_stats = defaultdict(lambda: {'total': 0, 'covered': 0})
    for (range_part, num_stmt), count in all_blocks.items():
        package = extract_package_from_range(range_part)
        try:
            num_stmt_int = int(num_stmt)
            file_stats[package]['total'] += num_stmt_int
            if count > 0:
                file_stats[package]['covered'] += num_stmt_int
        except ValueError:
            continue

    combined_total = 0
    combined_covered = 0

    for pkg in sorted(file_stats.keys()):
        stats = file_stats[pkg]
        combined_total += stats['total']
        combined_covered += stats['covered']

    print()
    for pkg in sorted(file_stats.keys()):
        stats = file_stats[pkg]
        if stats['total'] > 0:
            pct = (100.0 * stats['covered']) / stats['total']
            print(f'{pkg:<10} {stats["covered"]:>5} / {stats["total"]:<5} = {pct:>5.1f}%')

    print('-' * 35)
    if combined_total > 0:
        pct = (100.0 * combined_covered) / combined_total
        print(
            f'{"combined":<10} {combined_covered:>5} / {combined_total:<5} = {pct:>5.1f}%'
        )
    else:
        print('combined: no statements')

    sys.exit(0)


if __name__ == '__main__':
    main()
