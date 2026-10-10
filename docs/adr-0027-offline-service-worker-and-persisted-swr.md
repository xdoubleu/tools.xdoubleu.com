# ADR-0027: offline reads via a hand-written service worker and a persisted SWR cache

- Status: Accepted
- Issues: #2133 (part of epic #386); book files #2161, #2162
- Affects: `web/lib/offline/`, `web/app/sw.js/route.ts`, `web/components/SWRProvider.tsx`, `web/lib/books/offlineBooks.ts`, `web/lib/books/offlineSync.ts`

## Context

The apps should work without a connection: pages and data seen online load
offline, and writes queue for later (#2135). Every page is `force-dynamic`
with a per-request CSP nonce. Data arrives as ConnectRPC POSTs through SWR,
which the HTTP cache and the Cache API can't key. Next's
`experimental.useOffline` only retries RSC navigations and Server Actions.

## Decision

- **Service worker** (`lib/offline/serviceWorker.ts`): one self-contained
  function that `/sw.js` serves as `(${fn})(self, enabled)`,
  so Jest tests the real logic.
  - Navigations are network-first, falling back to the last saved copy, or an
    offline page if there is none. A saved page keeps its own CSP header and
    nonce together.
  - `/_next/static` is cache-first, since it is content-hashed.
  - An EPUB's WebPub (manifest and sections, served by the API) is kept by
    the page in the Cache API (`tools-webpub`), network-first, and warmed by
    the offline sync (`lib/books/webpubCache.ts`).
  - After each client-side route change, the page asks the worker to save its
    URL. These requests are throttled to once per 10 min per URL. Saving a
    page also caches the `/_next/static` assets it references.
- **Data**: an SWR middleware saves every successful fetch to IndexedDB. On a
  network error it resolves with the saved copy instead of failing, and the
  banner shows how old that copy is. Live-only and admin keys are excluded.
- **Books**: a file opened in the reader is stored in IndexedDB by book and
  format, with the version the library reports for it. The reader opens the
  stored copy until that version changes, and still opens it offline.
  - After each live library fetch, currently-reading books are downloaded
    one at a time in this device's chosen format. This is skipped offline
    or with data saver on. Each book's reader page and the reader modules
    are saved with it.
  - Stored files of finished or removed books are deleted. So is a book's
    other format (KEPUB vs original) once the stored choice's file is stored.
    Only a live library fetch triggers deletion, never a failed, cached or
    still-revalidating one.
- **Privacy**: signing out, or a different user ID appearing, wipes IndexedDB
  and the page cache. Entries older than 30 days are pruned; stored books
  are not, but `/books/settings` can remove them.
- **Kill switch**: with `OFFLINE_DISABLED=1`, `/sw.js` serves a worker that
  deletes its caches and unregisters itself.

## Alternatives considered

- **Serwist**: precaches the build's static assets via a bundler plugin
  alongside Sentry's wrapper. That buys little when every page is per-user
  HTML.
- **Caching API responses in the worker**: POST bodies can't be Cache API
  keys, and the worker can't see the SWR keys.
- **`experimental.useOffline`**: doesn't cover SWR/Connect calls or full
  reloads.

## Consequences

- Only pages opened online are available offline. Images from other origins
  aren't cached.
- A soft navigation renders its page a second time on the server, at most
  once per 10 minutes per URL.
- IndexedDB holds the user's data on the device until sign-out or 30 days.
- A saved page carries the server-rendered data from when it was saved, until
  SWR's saved or live copy replaces it.

## Revisit when

The double render shows up in server load, or browsers ship a way to cache
RSC payloads for offline use.
