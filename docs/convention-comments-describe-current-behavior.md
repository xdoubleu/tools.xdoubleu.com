# Convention: comments describe current behavior, not history

- Enforced by: nothing but review
- Issues: —

## Rule

**Never write a comment that references removed code, superseded architecture, or
frames a landed change as still-pending.**

If historical context genuinely explains *why* the current code looks the way it
does, phrase it so it stays true regardless of when it's read.

## Why

A stale claim actively misleads the next reader, human or Claude. A comment
saying a change "hasn't happened yet" is worse than no comment once it has: the
reader trusts it and reasons from a false premise.

This is also why decision history belongs in `docs/` rather than in comments or
CLAUDE.md prose — a document can be dated and superseded; an inline comment
silently rots.

## Worked examples

`api/cmd/api/kamal_proxy_shim.go`'s note describes itself as **"replicating what
the now-retired `gateway/` module used to provide"** — phrased so it stays true
regardless of when it's read, rather than "until `gateway/` is removed", which
would have become false the moment it was.

Follow that pattern: *"replicating what X used to provide"*, never *"X hasn't
happened yet"*.
