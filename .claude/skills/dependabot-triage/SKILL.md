---
name: dependabot-triage
description: Go through every open Dependabot/code-scanning/secret-scanning alert, bump whatever has an available fix (grouped into a small number of reviewable PRs), and file a tracking issue for whatever doesn't. Use whenever the user asks to "triage security alerts", "go through Dependabot", "check security alerts", or as part of a monitoring-sweep.
---

# Dependabot Triage

Bulk triage for GitHub's three alert types (Dependabot, code scanning, secret
scanning), all returned together by the `get_security_alerts` MCP tool. This
repo has no MCP tool that dismisses any of the three — every alert this
skill can't resolve with a code fix ends in a manual-dismissal instruction
to the user, never a silent skip.

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
   each alert number/package/reason, so it's revisited periodically and can
   be manually dismissed with reason "no fix available" if the team accepts
   the risk meanwhile.

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
     existing fake-credential conventions) via a normal PR, and report the
     exact alert numbers that should be manually dismissed as false
     positives once that PR merges.
   - If it looks like a real, currently-valid credential: **stop, do not
     treat it as safe, and flag it to the user immediately** — rotating a
     real secret requires human access to the actual provider dashboard and
     is outside what this skill (or any subagent) should do unilaterally.

5. **Report back**: a table of alert → outcome (fixed in PR #N, tracked in
   issue #M for no-fix-available, or flagged for human attention), plus the
   exact list of alert numbers that need manual dismissal in the GitHub UI
   (Security tab) and why, since no tool here can do that dismissal itself.

## Notes

- This mirrors `sentry-triage`'s shape (bulk sweep → cluster → fix-or-file →
  report) but for the three GitHub-native alert types instead of Sentry, and
  with an extra manual-dismissal reporting step since there's no equivalent
  of `resolve_sentry_issue` for any of these three alert types.
- Don't force a breaking major-version bump through automatically just
  because it's the only available fix — flag it for a human decision
  instead of guessing whether the breakage is acceptable.
- Usually invoked as part of `monitoring-sweep`'s security-alerts
  workstream, but stands alone fine as a periodic sweep on its own.
