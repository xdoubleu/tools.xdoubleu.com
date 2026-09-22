---
name: ship-pr
description: Rebase a finished branch on the latest default branch, push, open the GitHub PR, decide on auto-merge, and watch CI to green. Use when a task's code changes are complete and ready to ship, or when asked to "open a PR", "ship this", "finish up".
---

# Ship PR

The closing move of any task, paired with `task-worktree`. Run these steps
in order — don't skip ahead to opening the PR before this repo's own
lint/test/build checks pass, and don't stop at "CI is running" as if that
were done.

## 1. Rebase on the latest default branch, then open the PR yourself

Before opening the PR — not on every later push, see the note below — bring
the branch up to date:

```bash
git fetch origin <default>
git rebase origin/<default>
```

If the rebase reports conflicts, resolve them the normal way (fix the
files, `git add`, `git rebase --continue`) — never `git rebase --abort` and
skip this step. If the rebase actually replayed any upstream commits (check
`git log --oneline origin/<default>..HEAD` before and after — a no-op
rebase changes nothing there), re-run this repo's own lint/test/build
checks before pushing, since the rebase can shift line numbers or interact
with this task's own changes.

Then push and open the PR:

```bash
git push -u origin HEAD --force-with-lease
gh pr view --json number >/dev/null 2>&1 || gh pr create --fill --base <default>
```

`--force-with-lease` (not `-f`) is safe here — this is this task's own
feature branch, not the default branch, and it refuses to overwrite anything
if someone else pushed to the same branch since your last fetch. On a
brand-new branch (nothing pushed yet) it behaves like a normal push.

Only rebase+force-push right before the PR is first created. Once a PR is
open and under review (step 3's fix-and-repush loop), just push normally —
rebasing an already-reviewed branch rewrites commits a reviewer may have
already looked at.

Never push to the default branch directly; never open as `--draft`. If a
tracking issue exists for this work, reference it in the PR body using a
closing keyword (`Fixes #123`, `Closes #123`) so it auto-closes on merge —
a bare `#123` or "Related to #123" leaves it open even after merge.

## 2. Decide on auto-merge

Whether a change is safe to auto-merge is repo-specific — the calling
context (a project's own finish-task-style skill or CLAUDE.md) should supply
the actual criteria (size thresholds, which file patterns count as
"tooling/architectural", etc.). As a generic default absent any other rule:

- **Small, self-contained changes with no architectural/tooling footprint**:
  enable auto-merge right away, in the same breath as creating the PR —
  `gh pr merge --auto --squash` only merges once checks pass, so there's no
  reason to wait for green first:
  ```bash
  gh pr create --fill --base <default> && gh pr merge --auto --squash
  ```
- **Anything larger, or touching shared config/tooling/CI**: do **not**
  enable auto-merge. Open a normal (non-draft) PR and wait for the user's
  own review.

## 3. Monitor CI until green, fixing it yourself if it isn't

Wait using `gh`'s own watch and a loop that **exits on the outcome** — never a
foreground sleep-then-recheck cycle, which burns a turn per iteration and
tells you nothing the exit status wouldn't have.

Run both waits below in the background (Bash `run_in_background`): each ends by
itself once the result is decided, so you get one completion notification and
spend no turns waiting.

**a. Wait for the checks.** `--watch` polls inside the `gh` process
(`--interval`, 10s by default), so there is no loop to hand-roll — except for
one startup wrinkle: run immediately after PR creation, `gh pr checks
--required` can exit 1 with `no required checks reported` before GitHub has
registered any check runs on the branch yet — a transient gap, not a real
failure, since the workflow triggers off the push and hasn't started
reporting statuses. Wrap the watch in a small retry so that specific message
doesn't get mistaken for "no checks are required on this PR":

```bash
until out=$(gh pr checks --watch --fail-fast 2>&1); do
  code=$?
  if [ "$code" = 1 ] && grep -q "no checks reported" <<<"$out"; then
    sleep 10
    continue
  fi
  echo "$out"
  exit "$code"
done
echo "$out"
```

`--fail-fast` returns on the first failure instead of sitting through the
remaining checks. Exit status is the result: `0` all passed, `8` still
pending, anything else means a check failed.

Don't add `--required`: `gh pr checks` only recognizes required checks
declared via classic branch protection, not a repository ruleset's
`required_status_checks` rule (`gh api repos/<owner>/<repo>/rules/branches/<default>`
shows which one a repo uses). On a ruleset-protected repo `--required` fails
immediately with `no required checks reported on the '<branch>' branch`,
every single time, regardless of whether checks are actually running — a
plain `--fail-fast` wait still exits on any real failure without needing
required-check detection at all.

**b. Wait for the PR's terminal state.** Green checks are not the end state —
with auto-merge armed the merge itself lands seconds to minutes later, and
mergeability is reported separately from check status. Which condition you wait
for follows from step 2:

```bash
# mode=merged    → auto-merge was armed in step 2; wait for the merge to land
# mode=mergeable → no auto-merge; wait for a conflict-free PR, then hand off
mode=merged
while :; do
  read -r state mergeable mergestate <<<"$(gh pr view \
    --json state,mergeable,mergeStateStatus \
    --jq '"\(.state) \(.mergeable) \(.mergeStateStatus)"' 2>/dev/null)" || true
  echo "state=$state mergeable=$mergeable mergeStateStatus=$mergestate"
  case "$state" in
    MERGED|CLOSED) break ;;
  esac
  if [ "$mergestate" = DIRTY ]; then
    break                       # merge conflicts — needs you, not more waiting
  fi
  case "$mergeable" in
    MERGEABLE|CONFLICTING)      # UNKNOWN means still computing; keep polling
      if [ "$mode" = mergeable ]; then break; fi ;;
  esac
  sleep 20
done
```

Poll no faster than 20s — this is a remote API with rate limits — and let a
failed `gh` call fall through to the next iteration rather than killing the
loop; one flaky request is not an outcome.  Keep the variable named `mergestate` rather
than the obvious `status`: `status` is read-only in zsh, and the loop aborts on
its first line in any shell that happens to be one.

This second wait cannot be replaced by a single `gh pr view`, and the poll in it
is not an oversight: GitHub delivers state changes by webhook, which needs a
public endpoint a local CLI doesn't have; `mergeable` is computed lazily, so the
first query after a push legitimately returns `UNKNOWN` and merely *triggers*
the computation; and there is no `gh pr view --watch` to block on. `gh pr
checks --watch` is a poll too — the win is that its loop, and this one, run
somewhere you aren't spending turns.

A red PR or a non-mergeable state is not "done" — diagnose the actual failure
(don't just re-run blindly) and repeat from step 1. Once green + mergeable,
report the PR URL. Auto-merge was already armed in step 2 for small
self-contained changes; otherwise, stop here and wait for review.

## When `gh` isn't available (e.g. Claude Code on the web)

Check `command -v gh >/dev/null 2>&1` before step 1 rather than assuming —
some environments (notably Claude Code on the web) have no `gh` binary but
do have `mcp__github__*` tools mounted. Every step above still applies
conceptually; only the mechanics change:

- **Rebase and push (step 1's git parts)** are unaffected — plain `git
  fetch`/`rebase`/`push` never needed `gh`. Only the PR-creation line
  changes: replace `gh pr create --fill --base <default>` with whatever
  pull-request-create tool the mounted GitHub MCP server exposes. Exact tool
  names have churned across server releases (recent ones consolidated many
  single-purpose PR tools into fewer general ones, e.g. a `pull_request_write`-
  style tool with a `method`/`action` parameter) — don't hardcode a name
  here; discover what's actually mounted this run (`ToolSearch` with a
  query like `"select:mcp__github__*"` or a keyword search for "pull
  request"). Pass the same fields `--fill` would have inferred: title, body
  (including the closing keyword for the tracking issue — `Fixes #123`),
  base branch, head branch, and explicitly a non-draft flag.
- **Auto-merge (step 2)** — look for an auto-merge-eligible parameter on the
  same PR write/merge tool. If the mounted server's tool set has no way to
  arm auto-merge distinct from an immediate merge (this has genuinely
  varied across server versions), do not fake it by merging early or by
  silently leaving auto-merge unset — leave the PR open and non-draft, and
  say explicitly in your report that auto-merge could not be armed and
  needs a `gh pr merge --auto` from a session that has `gh`. Never relax the
  auto-merge *decision* itself (the size/footprint rule from step 2) because
  the mechanics changed — the decision is unaffected by which path enacts it.
- **Watching CI (step 3)** — replace `gh pr checks --watch` and `gh pr view`
  with the equivalent PR-status/checks-read tool (again, discover the actual
  name rather than assuming one). There's no MCP subscribe/watch primitive
  either, so poll it the same way: no faster than 20s, exit on a terminal
  state, tolerate one flaky call rather than aborting the loop.

Whichever path is used, the result must be identical: same PR body (closing
keyword and all), non-draft either way, same auto-merge decision made per
step 2's rule.

## Notes

- Never skip hooks (`--no-verify`) or bypass signing (`--no-gpg-sign`)
  unless the user has explicitly asked for it. If a hook fails, investigate
  and fix the underlying issue.
- Prefer whatever this repo's own lint/test/build commands are (Makefile
  targets, npm scripts, etc.) over ad-hoc equivalents for every check in
  this skill.
