# ADR-0030: the web reader renders EPUBs with Readium and PDFs with an in-house pdf.js pager

- Status: Accepted
- Issues: #2312, #2313, #2314, #2315, #2316, #2317
- Affects: `api/apps/books/webpub_routes.go`, `api/apps/books/internal/services/webpub.go`, `web/components/books/reader/`, `web/lib/books/{readium,pdf,webpubCache,readerModules}.ts`

## Context

foliate-js is unstable as a mobile web engine: it disclaims stability, was
only tested on WebKitGTK, and blank pages and PDF crashes on mobile had no
upstream fix. We carried a paginator patch, a pdf.js polyfill and a cache-bust
to keep it running.

## Decision

- **EPUB**: Readium's `@readium/navigator` (`EpubNavigator`). The API serves a
  WebPub manifest and single zip entries for the caller's original EPUB
  (`/books/api/book/{id}/webpub/…`), built on the same OPF parsing the
  converter uses; the reader never unzips a blob.
- **PDF**: a thin pager on pdf.js (`pdfjs-dist` legacy build), one canvas per
  page, outline-based contents. Readium has no PDF navigator.
- **KEPUB**: not read on the web (Readium can't render it); it exists only for
  Kobo sync. The web reader opens the original EPUB or PDF.
- **Positions** stay format-neutral ([adr-0029](adr-0029-cross-device-reading-positions.md)):
  `readium.ts` maps a locator's progression to a body-text offset and back.
- **Offline**: the offline sync warms the WebPub into the `tools-webpub`
  cache (network-first reads, cache fallback); sign-out clears it with the
  other `tools-` caches. The readers' chunks and the pdf.js worker are warmed
  through the service worker's `/_next/static` cache.

## Alternatives considered

- Keep foliate-js and patch it: each fix was another workaround on an
  unmaintained engine.
- Unzip the EPUB client-side into a Readium `Publication`: `@readium/shared`
  has no OPF parser, and presigned blob URLs expire.

## Consequences

- New dependencies (`@readium/navigator`, `@readium/shared`, `pdfjs-dist`) and
  a `worker-src 'self' blob:` CSP directive.
- Every WebPub request buffers the stored EPUB in memory (up to the upload
  cap); cache it if it shows up in metrics.
- An EPUB can't be read offline until its sync has warmed the cache.
- Annotations, search and TTS are absent, as before.

## Revisit when

Readium ships a PDF navigator, or WebPub requests become a load problem.
