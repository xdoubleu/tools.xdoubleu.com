# ADR-0029: reading positions are format-neutral, translated server-side, and the most recently read wins

- Status: Accepted
- Issues: #2156 (part of epic #2154); translation in #2158, #2160
- Affects: `books.book_reading_state`, `api/apps/books/internal/services/books.go` (`upsertReadingProgress`), `api/apps/books/internal/repositories/book_reading_state.go`, `api/apps/books/internal/services/kepub_spans*.go`, `api/apps/books/internal/services/reading_position_translate.go`, `web/lib/books/readerPosition.ts`, `web/hooks/useReadingState.ts`

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
    file, converter version and source file. A request waits up to 3 s;
    past that the PUT stores no `position`, and a sync leaves that book's
    `ChangedReadingState` for the next sync.
  - Span ↔ PDF page (#2160), at page precision.
  - Clients send and read only their own form, so an offline write can be
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
    close.
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
- A book with no ready KEPUB, a PDF source, or the `kobo-format-pdf` tag
  syncs percent only to the Kobo.
- Whether the device sends `Source` as a zip-root or OPF-relative path is
  unverified; matching accepts both.
- A position from a different file (PDF page in an EPUB) falls back to
  percent.

## Revisit when

Reading positions need sub-page PDF precision, or clock skew between devices
causes lost positions in practice.
