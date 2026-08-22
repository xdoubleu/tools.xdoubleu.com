---
name: proto-generate
description: Regenerate ConnectRPC stubs correctly after adding or changing a .proto file in tools.xdoubleu.com, in the order that avoids CI's proto-staleness check failing. Use whenever a file under proto/ was added or changed, before running lint/fix on the rest of the change.
---

# Proto Generate

Any `.proto` change needs **both** generators run, and the order matters —
getting it backwards passes locally but fails CI's proto-staleness check.

## Steps

1. Edit the `.proto` file(s) under `proto/`.
2. Regenerate both sides:
   ```bash
   cd api && make proto/generate    # regenerates api/gen/
   cd web && npm run generate       # regenerates web/lib/gen/
   ```
3. `make lint/proto` (part of `make lint`/`make lint/fix`) runs `buf lint` —
   e.g. RPC response types must be named `<Method>Response`. Run it locally
   before committing.
4. Do the rest of the task's work and lint as normal (`make lint/fix` in
   `api`, `npm run lint:fix` in `web` — see the `finish-task` skill).
5. **`make proto/generate` must be the last step touching `api/gen`/`web/lib/gen`
   before committing.** `make lint/fix`'s `gci` pass runs across the whole repo,
   including generated files, and reorders their import groups — CI's
   proto-staleness check diffs a raw `buf generate` (never gci'd) against the
   committed files, so a gci pass *after* generation makes it look stale even
   though nothing semantic changed.

   If `make lint/fix` ran after (or in the same session as) `make
   proto/generate` for any reason, re-run `make proto/generate` one more
   time afterward.
6. Before committing, confirm there's nothing left to re-stale:
   ```bash
   git diff api/gen web/lib/gen
   ```
   This should show nothing.

## Notes

- Generated stubs (`api/gen/`, `web/lib/gen/`) **are** committed.
- Don't read `api/gen/` or `web/lib/gen/` to discover field names or RPC
  signatures — read the corresponding `.proto` file instead; it's smaller
  and is the source of truth.
