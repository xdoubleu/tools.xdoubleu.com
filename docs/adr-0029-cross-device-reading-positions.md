# ADR-0029: reading positions are format-neutral, translated server-side, and the most recently read wins

- Status: Accepted
- Issues: #2156 (part of epic #2154); translation in #2158, #2160
- Affects: `books.book_reading_state`, `api/apps/books/internal/services/books.go` (`upsertReadingProgress`), `api/apps/books/internal/repositories/book_reading_state.go`, `api/apps/books/internal/services/kepub_spans*.go`, `api/apps/books/internal/services/reading_position_*.go`, `api/apps/books/internal/services/conversion_epubbuild_pages.go`, `web/lib/books/readerPosition.ts`, `web/hooks/useReadingState.ts`

## Context

A book is read on a Kobo (converted KEPUB, positions are `kobo.X.Y` spans)
and in the web reader (original EPUB or PDF, foliate-js ranges). Each device
can be offline for a while and report late. Percent alone isn't enough: the
Kobo firmware reopens at the cover without a `Location`, and a percent lands
on a different paragraph in each format.

## Decision

- **Neutral position.** `book_reading_state.position` stores
  `{"href","offset"}` for EPUB/KEPUB or `{"page"}` for PDF.
  - `href` is the content document path from the EPUB manifest, relative to
    the zip root (foliate's section `id`).
  - `offset` counts UTF-16 code units of that section's `body.textContent`
    before the position.
  - `page` is 1-based.
  - The raw Kobo bookmark stays in `kobo_location`. Every write replaces the
    whole row; a Kobo write also stores its translated `position`.
- **Server-side translation.** The server maps between forms:
  - KEPUB span ↔ EPUB offset, against the KEPUB's EPUB source file.
    kepubify's HTML5 parser changes a little text (whitespace after
    `</body>`, a `<pre>`'s leading newline, CDATA, `<noscript>`, U+FFFD), so
    each document's two texts are aligned with a bounded diff, not assumed
    equal. Ambiguous characters match late, so a span after dropped text
    lands on its own text.
  - Kobo → neutral: on a `KoboSpan` PUT, `Source` resolves exact →
    case-insensitive → unique path suffix, and `offset` is the span's start.
  - Neutral → Kobo: the span containing, or nearest before, `offset` (the
    first of spans sharing a start), sent with the zip-root path as `Source`.
  - Span maps are built lazily and kept in an in-memory LRU keyed by KEPUB
    file, converter version and source file. Only states the device lacks
    are translated. A request waits up to 3 s; past that a PUT's `position`
    is filled in once the map is built, and a sync holds that book's
    `ChangedReadingState` back for up to 5 min, then sends percent only.
  - Span ↔ PDF page, at page precision. The PDF converter marks where each
    page starts with `<span epub:type="pagebreak" id="pdfpage-N">`, inside
    the paragraph when one continues from the page before, plus a
    `page-list` nav; kepubify keeps both.
    - A PDF-sourced KEPUB's map is built from the KEPUB alone, so offsets
      are its own text's.
    - Its neutral form is `{page}`: a span maps to the page whose anchor
      precedes it, and a page to the first span at or after its anchor (a
      page with no anchor, such as a blank one, uses the nearest page before
      it).
    - `GetReadingState` and `TranslateReadingPosition` return both `{page}`
      and the KEPUB's `{href, offset}`, so the web reader resumes either
      file at the page.
  - A regenerated PDF-sourced KEPUB can renumber its spans, so until a
    device is sent the current KEPUB its bookmarks belong to the old one.
    - Such a PUT stores no position.
    - The sync that sends the replacement (`ChangedEntitlement`) re-derives
      the bookmark from the stored page, or sends percent only. It also
      clears `kobo_location`, so the bumped state goes out again on the next
      sync, translated from the page.
    - An EPUB source's spans are stable, because kepubify is deterministic.
  - Clients send and store only their own form, so an offline write can be
    translated when it replays.
- **Most recently read wins.** Each write carries `read_at`: the web's client
  time, or the Kobo bookmark's `LastModified`.
  - The upsert applies a write only if `read_at` is newer than the stored
    `read_at` (`updated_at` for older rows). It checks this atomically, in
    `ON CONFLICT … WHERE`.
  - A write without `read_at` counts as now, and a future `read_at` is
    clamped to now.
  - A rejected write is a no-op, not an error.
  - A newer write may move backwards, and its percent becomes the library
    progress.
- **Kobo guard.** A Kobo write with no location, or at 0%, still never
  lowers the percent. Those writes come from re-downloads and opens, not
  from reading.
- **Web reader.**
  - Opening seeks to the stored position if it fits the open file,
    otherwise to the percent.
  - It saves 2 s after the last page turn, and flushes on hide, pagehide and
    close. Saves queue in the outbox (ADR-0028), so offline reading syncs
    later; hide and pagehide also send directly, since the page may be
    frozen or killed first.
  - The relocates fired while opening are never saved, so opening a book
    doesn't overwrite a Kobo position.

## Alternatives considered

- **CFI as the stored position**: EPUB-only and tied to one file's DOM, so it
  can't map to a KEPUB span or a PDF page.
- **Client-side translation**: every client would need both files, and an
  offline write would be translated against whatever the client had cached.
- **Furthest position wins** (the old #1889 guard for everything): rereading
  an earlier chapter would never sync.
- **Server receive time wins**: a device that syncs late would overwrite
  newer reading done elsewhere.

## Consequences

- Device clocks matter. A slow clock loses to reading done just before; the
  clamp stops a fast clock from pinning the position.
- A book with no ready KEPUB, or the `kobo-format-pdf` tag, syncs percent
  only to the Kobo.
- A PDF-sourced KEPUB's `{href, offset}` (from the web reader on the KEPUB)
  is in the KEPUB's own text, so a regeneration that changes that text can
  shift it.
- Whether the device sends `Source` as a zip-root or OPF-relative path is
  unverified; matching accepts both.
- A position from a different file (PDF page in an EPUB) falls back to
  percent.

## Revisit when

Reading positions need sub-page PDF precision, or clock skew between devices
causes lost positions in practice.
