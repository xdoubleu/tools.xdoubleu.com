# ADR-0013: Gate on coverage of changed lines, not whole-file percentages

- Status: Accepted
- Issues: #1301, #1364, #1376
- Affects: `tools/diff_coverage_ts.py`, `tools/diff_coverage_go.py`, `tools/merge_coverage.py`, `tools/extend_signature_coverage.py`, `api/Makefile`, `web/package.json`

## Context

A whole-file coverage percentage is dishonest in both directions: a one-line edit
to a file with pre-existing gaps fails, while a wholly untested new function
added to an otherwise well-covered file hides behind that file's healthy overall
number.

## Decision

Gate on coverage of the **lines changed vs `origin/main`**, matching what CI's
`codecov/patch` check gates on:

- `npm run test:cov:diff` (web) → `tools/diff_coverage_ts.py`, a plain-Python
  lcov parser
- `make test/cov/diff` (api) → `tools/diff_coverage_go.py`
- `make test/cov/per-pkg` merges per-package Go profiles via
  `tools/merge_coverage.py`

Run these **before pushing**, to catch an uncovered branch locally instead of
after a CI round trip.

`diff_coverage_ts.py` exits non-zero on any changed file whose *changed lines*
fall under 80% line/branch coverage, **or that has no coverage data at all** —
e.g. a new file nothing imports yet, exactly the gap that used to only surface in
CI. A file whose diff touches no instrumented lines is reported but never
flagged. It only covers files matched by `collectCoverageFrom` in
`jest.config.js`.

### The signature-coverage fixup (#1376)

`make test/cov/report` post-processes the profile before upload via
`tools/extend_signature_coverage.py`.

Go opens a function's first coverage block **at its body's opening brace**, so
the parameter lines of a signature `golines` wrapped over the 88-char limit
belong to no block at all — Codecov reports them as missed, and the `) ... {`
line as partial. That cost every newly added function 2–3 patch lines it could
never cover, and sank `codecov/patch` to 71.42% on PR #1375 for code that both
`make test/cov/diff` and a local replay of CI's pipeline measured as 100%
covered.

The script extends each such block back to its `func` keyword, matching what
`diff_coverage_go.py` already effectively assumes. (#1376 is distinct from
#1364's auto-discovery of the pre-dedup intermediates, despite the same visible
symptom.)

## Alternatives considered

### Whole-file coverage thresholds

The status quo #1301 replaced. Dishonest in both directions — see Context.

### Loosening the Codecov patch threshold to absorb the signature lines

Rejected: it would hide real gaps to work around a measurement artifact. Fixing
the profile is the honest fix.

### Not wrapping long signatures

Not an option — `golines` enforces the 88-char limit repo-wide.

## Consequences

- Local and CI coverage verdicts agree, so a green local run is meaningful.
- `extend_signature_coverage.py` is coupled to Go's coverage block semantics; a
  toolchain change to how blocks are emitted would need it revisited.

## Revisit when

Go's coverage tooling opens function blocks at the `func` keyword itself, making
the fixup unnecessary.
