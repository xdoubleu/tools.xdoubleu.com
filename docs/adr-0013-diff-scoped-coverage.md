# ADR-0013: Gate on coverage of changed lines, not whole-file percentages

- Status: Accepted
- Issues: #1301, #1364, #1376, #1632, #1868
- Affects: `tools/diff_coverage_ts.py`, `tools/diff_coverage_go.py`, `tools/merge_coverage.py`, `tools/extend_signature_coverage.py`, `api/Makefile`, `web/package.json`

## Context

Whole-file percentages mislead both ways: a one-line edit to a gappy file fails,
while an untested new function hides in a well-covered file.

## Decision

Gate on **lines changed vs `origin/main`**, like Codecov's `codecov/patch`:

- `npm run test:cov:diff` → `tools/diff_coverage_ts.py`; `make test/cov/diff` →
  `tools/diff_coverage_go.py`; `make test/cov/per-pkg` merges profiles via
  `tools/merge_coverage.py`.
- `diff_coverage_ts.py` fails a changed file under 80% line/branch on its
  changed lines, **or with no coverage data at all** (e.g. a new unimported
  file). Only files in `jest.config.js`'s `collectCoverageFrom` count.
- Local numbers approximate Codecov, not predict it: Codecov's line counting
  can diverge either way (#1868), so `diff_coverage_go.py` also reports a
  conservative block-boundary-only number.
- **Signature fixup** (#1376): Go opens a function's coverage block at the body
  brace, so `golines`-wrapped parameter lines count as missed.
  `make test/cov/report` runs `extend_signature_coverage.py` to extend each
  block back to `func`.
- Mutation testing (`make test/mutation/diff`, `npm run test:mutation:diff`,
  #1632) reuses the same diff scoping via `tools/diff_packages_go.py` /
  `tools/diff_files_ts.py`.

## Alternatives considered

- **Whole-file thresholds** — misleading, as above.
- **Loosen Codecov's patch threshold** — hides real gaps.
- **Don't wrap signatures** — `golines` enforces 88 chars.

## Consequences

- A green local run is meaningful, though not a guarantee of Codecov's verdict.
- `extend_signature_coverage.py` depends on Go's block semantics.

## Revisit when

Go opens function coverage blocks at `func`.
