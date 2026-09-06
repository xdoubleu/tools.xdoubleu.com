---
name: dependabot-triage
description: Go through every open Dependabot/code-scanning/secret-scanning alert, bump whatever has an available fix (grouped into a small number of reviewable PRs), and file a tracking issue for whatever doesn't. Use whenever the user asks to "triage security alerts", "go through Dependabot", "check security alerts", or as part of a monitoring-sweep.
---

# Dependabot Triage

Bulk triage for GitHub's three alert types (Dependabot, code scanning, secret
scanning), all returned together by the `get_security_alerts` MCP tool.
Every alert this skill can't resolve with a code fix is either dismissed
directly via the `dismiss_security_alert` MCP tool or filed as a tracking
issue — never a silent skip. `dismiss_security_alert` takes `alert_type`
(`dependabot`|`code_scanning`|`secret_scanning`), `alert_number`, and a
`reason` whose valid values depend on `alert_type`: dependabot →
`fix_started`|`inaccurate`|`no_bandwidth`|`not_used`|`tolerable_risk`;
code_scanning → `"false positive"`|`"won't fix"`|`"used in tests"`;
secret_scanning →
`false_positive`|`wont_fix`|`revoked`|`used_in_tests`|`pattern_deleted`.

## Steps

1. **Pull every open alert**: `get_security_alerts`. Split by `alertType`:
   `SECURITY_ALERT_TYPE_DEPENDABOT`, `SECURITY_ALERT_TYPE_CODE_SCANNING`,
   `SECURITY_ALERT_TYPE_SECRET_SCANNING`.

2. **Dependabot alerts** — for each, determine:
   - Which module/package.json it belongs to (`api/go.mod`,
     `kobo-gateway/go.mod`, `sentrytools/go.mod`, or `web/package.json`) and
     whether it's a direct or transitive dependency.
   - Whether a fixed version exists upstream (the alert/advisory usually
     names the patched version).

   Group fixes into a **small number of PRs**, not one per alert — e.g. one
   PR per affected Go module bumping every fixable Go dependency together
   (`go get <module>@<fixed-version>` + `go mod tidy`, then `make
   test`/`make lint`), one PR bumping every fixable npm dependency together
   (`npm install <pkg>@<fixed-version>`, then `npm run lint`/`npm
   test`/`npm run build`). Each PR still needs its own tracking issue per
   this repo's `start-task`/`finish-task` convention — reference the alert
   numbers it resolves in the issue/PR body.

   For anything with **no fix available yet upstream**: don't invent a
   workaround. Collect these into one tracking GitHub issue (`issue_write`,
   not a full `start-task`/`finish-task` — there's no code change) listing
   each alert number/package/reason, so it's revisited periodically. Leave
   the alerts open — only call `dismiss_security_alert(alert_type:
   "dependabot", reason: tolerable_risk)` on one once the team has actually
   accepted the risk (recorded in that tracking issue), never as part of
   this sweep itself.

3. **Code scanning alerts** — read the flagged file and the specific rule
   (e.g. `actions/cache-poisoning/direct-cache`). Fix the actual hole the
   rule describes (check the rule's own documentation/semantics — don't
   pattern-match a generic fix) via the normal `start-task`/`finish-task`
   flow, verifying the fix doesn't break whatever legitimate mechanism the
   flagged code implements (e.g. a build cache other workflows depend on).

4. **Secret scanning alerts** — never assume "false positive" without
   checking. For each alert:
   - Search the full git history (not just the current tree) for the
     matching pattern — `git log -p -S <distinctive-substring>` — since the
     value may have been introduced and later removed but the alert
     persists until dismissed.
   - If it's a genuine fixture/placeholder value that happens to match a
     real-secret regex: fix the fixture to something that obviously can't
     match (a clearly-fake placeholder, consistent with this codebase's
     existing fake-credential conventions) via a normal PR, then once that
     PR merges call `dismiss_security_alert(alert_type: "secret_scanning",
     reason: false_positive)` for each alert number it resolves.
   - If it looks like a real, currently-valid credential: **stop, do not
     treat it as safe, and do not dismiss it.** Rotating a real secret
     requires human access to the actual provider dashboard, which is
     outside what this skill (or any subagent) should do unilaterally — file
     a tracking issue immediately (`issue_write`, titled to make clear this
     is a live credential exposure, not a routine finding) and move on to
     the next alert rather than waiting on a response.

5. **Report back**: a table of alert → outcome — fixed in PR #N, dismissed
   this run via `dismiss_security_alert` (with alert number and reason),
   tracked in issue #M pending a human accept-risk/rotate-credential
   decision, or still open awaiting that PR to merge.

## Notes

- This mirrors `sentry-triage`'s shape (bulk sweep → cluster → fix-or-file →
  report) but for the three GitHub-native alert types instead of Sentry.
  `dismiss_security_alert` is this skill's equivalent of
  `resolve_sentry_issue` — the only alerts left un-dismissed at the end of a
  run are the ones genuinely waiting on a human (no fix upstream yet, or a
  real credential needing rotation), tracked in the issue filed for them.
- Don't force a breaking major-version bump through automatically just
  because it's the only available fix — flag it for a human decision
  instead of guessing whether the breakage is acceptable.
- Usually invoked as part of `monitoring-sweep`'s security-alerts
  workstream, but stands alone fine as a periodic sweep on its own.
