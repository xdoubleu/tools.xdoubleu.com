# ADR-0028: offline writes go through a client outbox with client-generated IDs

- Status: Accepted
- Issues: #2135 (part of epic #386)
- Affects: `web/lib/offline/outbox.ts`, `web/lib/offline/registry.ts`, `web/lib/<app>/offlineWrites.ts`, create RPCs of apps with offline writes

## Context

Some writes must work without a connection and reach the API later
(ADR-0027 covers reads). They used to be direct Connect calls followed by an
SWR refetch, with nothing optimistic. Creates got their IDs from
`gen_random_uuid()`. That meant a replayed create could duplicate, and a
change made offline couldn't reference a row created offline. Refresh tokens
are single-use, so concurrent replays after the access token expires would
race each other and lose the session.

## Decision

- **Outbox.** Writes are queued in IndexedDB as encoded protobuf requests and
  sent strictly in order, one at a time. A Web Lock (`tools-outbox`) keeps
  other tabs from sending at the same time.
  - Sending starts on load, on `online`, after each enqueue, and every 30
    seconds while anything is queued.
  - A network error stops the drain and keeps the queue.
  - `Unauthenticated` pauses the queue and asks the user to sign in again.
  - Any other rejection moves the write to a "couldn't sync" list the user
    dismisses.
  - Server errors are retried, at most 5 attempts.
- **Optimistic reducers.** Each write declares an idempotent `apply` per SWR
  key. It runs once against the cache when queued. The SWR middleware
  re-applies it to every fetched or saved response while the write is still
  queued, so a refetch never hides a pending change.
- **Last write wins.** Writes replay as sent, with no version checks.
- **Client IDs.** A create used offline takes an optional client UUID. A
  repeat with the same ID returns the existing row, while an ID owned by
  someone else is `NotFound`. With no ID, the database generates one, as
  before.
- **Sign-out** sends what it can, then wipes the queue together with the
  saved data.

## Alternatives considered

- **Background Sync**: not supported in iOS Safari.
- **Idempotency-key table**: adds a table and a cleanup job, and still
  doesn't let offline writes reference rows created offline.
- **Version checks with conflict UI**: more than a single family's data needs.

## Consequences

- Writes no longer raise errors to their form. Server rejections surface in
  the banner, so forms check obvious conflicts, such as duplicate names,
  against cached data before queueing.
- A write's reducer mirrors server behaviour (sort order, formatting) until
  the follow-up refetch replaces it.
- A later write that depends on a rejected create fails as well.

## Revisit when

Two family members routinely edit the same rows offline and lose changes.
