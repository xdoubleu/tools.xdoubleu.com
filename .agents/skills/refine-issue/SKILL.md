---
name: refine-issue
description: Create or refine a single GitHub issue — summary, type/scope labels, priority, and project-board status — and keep a "## Plan" section in its body in sync — and grill the user about the scope, round by round, before any code is written. Use when starting work that has no tracking issue yet, when a plan-mode plan needs to be recorded on an issue, or when development begins and an issue's status should move to "in progress".
---

# Refine Issue

Single-issue version of the refinement `issue-triage` does in bulk: give one
issue a summary, the right labels, a priority, and a status on the project
board (if one is configured). Also owns keeping a `## Plan` section on the
issue in sync with plan mode, and moving status to "in progress" once
development actually starts.

`issue-triage` reuses the config/labels/priority-rule below rather than
redefining them — keep the two in sync if any of this changes.

## Config

This skill is config-driven so it works across repos without editing the
skill itself. Look for `.claude/github-triage.config.json` at the target
repo's root:

```json
{
  "repo": "owner/name",
  "project": { "owner": "owner-login", "number": 8 },
  "labels": {
    "types": ["bug", "enhancement", "feature", "chore", "documentation"],
    "scopes": ["area-a", "area-b"],
    "fallbackScope": "platform"
  },
  "priorityRule": "P0 = fixes/restores something already working today that's now broken. P1 = improves existing, working functionality. P2 = brand-new features that don't exist yet. A shiny new feature never outranks a broken thing.",
  "markerLabel": "triaged"
}
```

- `project` is optional — omit it entirely if this repo has no project
  board, and skip every project-board step below.
- `labels.types`/`labels.scopes` should list labels that **already exist**
  in the repo — don't invent new ones; run `gh label list --repo <repo>` if
  unsure what's actually there.
- `priorityRule` is free text describing this repo's own P0/P1/P2 (or
  equivalent) ordering — apply it verbatim over any other instinct. When an
  issue is ambiguous, ask which bucket it means rather than guessing.

**If the config file doesn't exist yet**: ask the user for `repo` (default
to the current repo), whether there's a project board (and if so its
owner/number — `gh project list --owner <owner>` to find it), the label
lists (`gh label list --repo <repo>` to see what's available), and the
priority rule they want. Write the answers to
`.claude/github-triage.config.json` so future runs (in this repo, by this
skill or `issue-triage`) don't need to ask again.

If `project` is set, look up its `Status` and `Priority` (or equivalently
named) single-select fields' current field/option ids each run with `gh
project field-list <number> --owner <owner> --format json` — don't
hardcode ids in the config file, since they aren't worth tracking there and
can drift.

## Steps — create or refine an issue

1. If no tracking issue exists yet for the work: `gh issue create --repo <repo> --title <title> --body <body>`.
2. Write/update the body with a one-line summary up top, original text preserved below a divider:
   ```
   gh issue edit <num> --repo <repo> --body "$(printf '**Summary:** %s\n\n---\n\n%s' "$SUMMARY" "$ORIGINAL_BODY")"
   ```
3. Apply the type label, scope label, and the marker label in one call:
   `gh issue edit <num> --repo <repo> --add-label "bug,area-a,triaged"`.
4. If a project board is configured and the issue isn't on it yet, add it and capture the item id:
   `gh project item-add <number> --owner <owner> --url <issue-url> --format json --jq .id`
5. If a project board is configured, set its priority and status fields using the field/option ids looked up in Config. A reasonable default status mapping on creation: highest-severity priorities → a "ready to pick up" status, lowest priority (new feature) → a backlog-style status — adapt to whatever this repo's own status options actually are.
   ```
   gh project item-edit --project-id <PVT_id> --id <item-id> --field-id <field-id> --single-select-option-id <opt-id>
   ```

6. **Sanity-check the issue is actually refined, not just labeled.** Before
   considering an issue ready to work (and again before moving it to "in
   progress" below), read it critically:
   - **Is the body empty?** An issue with a title and nothing else is never
     self-explanatory — a title states a topic, not a scope, and the
     readings it permits usually differ enough to produce materially
     different work. Stop and ask the user what they want before planning,
     exploring further, or editing anything, then write their answer into
     the body so the next session doesn't have to ask again. This is not a
     judgement call and applies however obvious the title looks.
   - Is there anything else ambiguous or missing that would make you guess
     instead of ask? If so, ask the user rather than proceeding.
   - Does the issue's description reflect the **full blast radius** of the
     change, not just the most visible part? Check the actual codebase for
     other consumers of whatever is being changed — other interfaces that
     wrap or expose the same functionality (APIs, generated/served
     interfaces, admin or read-only tooling built on top of it, UI/consumers
     of it) — and if the issue is silent on one of them, add it to the body
     (or ask the user what the intended behavior there should be, e.g. what
     a UI change should look like) before treating the issue as ready.

   This isn't a one-time gate — re-run it whenever the issue's scope
   changes materially (a new `## Plan` section, a reopened issue, etc).
   Whatever it turns up feeds the grilling below, which is what actually
   puts it in front of the user.

## Steps — record a plan

Once a plan-mode plan exists for the issue's work, insert or replace a
`## Plan` section in the issue body — placed after the Summary line, before
the original-body divider. If a `## Plan` section already exists from a
prior session, replace it rather than appending a duplicate:
```
gh issue edit <num> --repo <repo> --body "$(printf '**Summary:** %s\n\n## Plan\n\n%s\n\n---\n\n%s' "$SUMMARY" "$PLAN" "$ORIGINAL_BODY")"
```

## Steps — grill the scope before work starts

Before the first code edit, **grill the user** about the scope until you
reach a shared understanding — don't settle for a single one-shot "here's
what I'll do, ok?" message. This is a required round-trip, not a judgement
call: do it even when the issue looks unambiguous to you, and even when the
user's request sounded specific. "It seemed clear" is exactly the state
that produces a correction later.

If a `grilling` skill is available, use it. Otherwise run its method by
hand: map the work as a **design tree** where every decision branches into
the decisions that hang off it, and work the tree in **rounds**. Each round,
ask the whole *frontier* — every decision whose prerequisites are already
settled — as a numbered list, each question with your recommended answer,
then wait for the user's answers before the next round. Their answers push
the frontier outward and unblock the next round. Find facts yourself
(codebase, tools, subagents); only the *decisions* go to the user. You are
done when the frontier is empty — every branch visited, nothing silently
assumed.

Across the rounds, make sure these are explicitly settled, not guessed:

- what will change, and what will **not** — the boundary is the part
  people actually correct
- the approach, whenever more than one reasonable one exists
- the blast radius surfaced by the check above, including any consumer
  you intend to leave alone
- anything you are assuming rather than know

Do not begin editing until the frontier is empty and the user confirms the
shared understanding. As answers come in, update the issue body (and its
`## Plan`, if present) to match, so the issue stays the record of what was
actually agreed.

This is unconditional — there is no "the issue looks trivial" or "the user's
message already spelled it out" escape hatch; a settled frontier just makes
for a short single round.

## When `gh` isn't available (e.g. Claude Code on the web)

Check `command -v gh >/dev/null 2>&1` before the first `gh` call rather than
assuming — some environments (notably Claude Code on the web) have no `gh`
binary but do have `mcp__github__*` tools mounted. Don't guess tool names:
exact ones have churned across GitHub MCP server releases (single-purpose
tools have been consolidated into fewer general ones, e.g. an issue-write
tool taking a `method`/`action` parameter for create/edit/label in one
call) — discover what's actually mounted this run (`ToolSearch` with a
query like `"select:mcp__github__*"` or a keyword search for "issue")
before mapping the steps below onto it.

- **Steps 1–3 (create/edit body/labels)** all map onto that issue read/write
  tool family — same fields (`repo`, `title`, `body`, `labels`), same
  summary-up-top-original-below-divider body format.
- **Steps 4–5 (project board)** need the GitHub MCP server's Projects
  toolset, which is newer and not guaranteed to be mounted even when other
  `mcp__github__*` tools are. If a project-board tool genuinely isn't
  available, this is a real capability gap, not a silent skip — state
  explicitly in your report that Priority/Status could not be set and needs
  a `gh project item-edit` from a session that has `gh`, the same way the
  no-project-configured case is already silent-by-design but this one is
  not.
- **The grilling steps** have no `gh` dependency at all and apply unchanged
  either way.

Whichever path is used, the result must be identical: same body content
(summary, `## Plan` section, original text preserved), same labels, and the
same project-board fields whenever that tool is actually reachable.

## Steps — move to in progress

When development actually starts (first code edit or commit on the branch,
not just issue creation), re-run the refined-enough/blast-radius check
above, and grill the scope with the user as described above, before
treating the issue as good to go. If a project board is
configured, set its status field to an "in progress" equivalent via the
same `gh project item-edit` pattern as above.

## Notes

- If the project board, its number, or its field names ever change,
  re-derive them from `gh project list --owner <owner>` / `gh project
  field-list <n> --owner <owner>` rather than trusting anything cached from
  a previous run — don't cache field/option ids in the config file itself.
- This skill doesn't know anything about a specific issue-tracker integration
  beyond GitHub's own Issues/Projects — if a repo also wants to resolve
  entries in an external error tracker (Sentry, etc.) once a fix ships, that
  belongs in a repo-specific wrapper skill, not here.
