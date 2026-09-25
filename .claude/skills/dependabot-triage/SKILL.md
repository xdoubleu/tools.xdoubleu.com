---
name: dependabot-triage
description: Go through every open Dependabot/code-scanning/secret-scanning alert, bump whatever has an available fix (grouped into a small number of reviewable PRs), and file a tracking issue for whatever doesn't. Use whenever the user asks to "triage security alerts", "go through Dependabot", "check security alerts", or as part of a monitoring-sweep.
---

# Dependabot Triage

Bulk triage of Dependabot, code-scanning, and secret-scanning alerts from
`get_security_alerts`. Every alert ends fixed, dismissed, or tracked in an
issue — never silently skipped.

`dismiss_security_alert(alert_type, alert_number, reason)` valid reasons:

- `dependabot`: `fix_started`|`inaccurate`|`no_bandwidth`|`not_used`|`tolerable_risk`
- `code_scanning`: `"false positive"`|`"won't fix"`|`"used in tests"`
- `secret_scanning`: `false_positive`|`wont_fix`|`revoked`|`used_in_tests`|`pattern_deleted`

## Steps

1. **Pull alerts:** `get_security_alerts`, split by `alertType`
   (`SECURITY_ALERT_TYPE_DEPENDABOT`, `_CODE_SCANNING`, `_SECRET_SCANNING`).

2. **Dependabot** — per alert, find the module (`api/go.mod`,
   `kobo-gateway/go.mod`, `sentrytools/go.mod`, `web/package.json`), direct
   vs. transitive, and the patched version.
   - Group fixes into **few PRs** — one per Go module (`go get
     <module>@<fixed>` + `go mod tidy`, then `make test`/`make lint`), one
     for npm (`npm install <pkg>@<fixed>`, then `npm run lint`/`npm test`/`npm
     run build`). Each goes through `start-task`/`finish-task`; list the
     alert numbers it resolves in the issue/PR body.
   - **No upstream fix:** no workarounds. Collect into one tracking issue
     (`issue_write`) listing alert/package/reason; leave alerts open. Dismiss
     as `tolerable_risk` only once a human accepted the risk in that issue,
     never during the sweep.
   - **Breaking major bump** as the only fix → flag for a human decision.

3. **Code scanning** — read the flagged file and the rule's own semantics
   (e.g. `actions/cache-poisoning/direct-cache`); fix the actual hole via
   `start-task`/`finish-task` without breaking the legitimate mechanism the
   code implements (e.g. a cache other workflows depend on).

4. **Secret scanning** — never assume false positive:
   - Search full history: `git log -p -S <distinctive-substring>`.
   - Fixture/placeholder matching a secret regex → replace with an obviously
     fake value via a normal PR; after merge, dismiss each resolved alert as
     `false_positive`.
   - Looks real → **don't dismiss, don't treat as safe.** File a tracking
     issue titled as a live credential exposure (rotation needs a human with
     provider access) and move on.

5. **Report** a table: alert → fixed in PR #N / dismissed (number + reason) /
   tracked in issue #M awaiting a human / open pending PR merge.

Usually run from `monitoring-sweep`'s security workstream; also standalone.
