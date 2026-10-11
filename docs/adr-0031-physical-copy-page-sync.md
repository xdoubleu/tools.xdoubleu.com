# ADR-0031: a physical copy's page is mapped through its EPUB, and synced as a `physical` reading state

- Status: Accepted
- Issues: #2401 (follows epic #2154)
- Affects: `books.user_books.physical_*`, `api/apps/books/internal/services/physical_*.go`, `api/apps/books/connect_physical.go`, `web/components/books/BookProgressForm.tsx`, `web/components/books/BookPhysicalCopyFields.tsx`

## Context

A book owned on paper and as an EPUB is read in both. A page typed in by hand
moved nothing, and Kobo/web reading never moved the page. Positions are
format-neutral and the most recent read wins ([ADR-0029](adr-0029-cross-device-reading-positions.md)).
The catalog's page count may describe a different edition than the user's.

## Decision

- **Scope:** a book tagged `own-physical` with a ready EPUB. A KEPUB
  converted from a PDF is excluded, since its positions are PDF pages.
- **Per-copy details** on `user_books`, editable in the progress dialog:
  - page count (default: the catalog's)
  - page where the text starts (default 1)
  - anchors `{page, href, offset}`, picked from the EPUB's TOC
- **Mapping** runs over the EPUB's linear spine, in ADR-0029 offsets:
  1. **Print page markers.** These are the page list (nav or NCX), else
     `pagebreak` markers in the text. They map arabic page labels exactly,
     and roman front matter maps to the first arabic page.
  2. **Otherwise, interpolation.** Pages are linear between anchors: the
     text-start page at the body matter (the `bodymatter` landmark, else the
     guide's `text`, else the spine start), the user's anchors, and page count
     + 1 at the end of the text.
     - A user anchor replaces a default it conflicts with, and anchors out of
       order are dropped.
     - A page maps to its first offset and an offset maps to the page it's on,
       so a page round-trips.
- **Physical → digital.**
  - Saving a page writes a reading state with source `physical`, the
    translated `{href, offset}` and the client's save time as `read_at`.
  - It skips the library's percent switch, because the page is already the
    library progress.
  - The Kobo and the web reader follow it through the existing translation.
  - Past the 3 s budget, the write finishes in the background.
- **Digital → physical.**
  - After a web or Kobo write, or a Kobo position backfill, `current_page` is
    set in the background.
  - It only applies while that write is still the newest state, so it never
    overwrites a later page entry.
  - `GetReadingState` adds the page and the first sentence there, with block
    elements spaced apart, for "Continue on page ~N".
- **Text model.** Parsed lazily from the EPUB and kept in a 4-entry LRU,
  built the same way as span maps (slots, singleflight, empty on a broken
  file).

## Alternatives considered

- **Physical page as the stored position:** a third form every translator
  would need. Mapping to `{href, offset}` reuses ADR-0029's Kobo path
  unchanged.
- **Catalog page count only:** it is often another edition's, and
  interpolation is then off by whole chapters.
- **Updating `current_page` on read:** the library list shows it without
  reading the state.
- **OCR of a photographed page:** a possible follow-up, not needed for
  page-level sync.

## Consequences

- Interpolation is accurate to a page or two, which is why the hint shows
  the sentence to look for.
- The text model holds a whole book's text, which is why the cache is small.
- A digital write leaves the library in percent mode with the page beside it.
  A page entry switches it back to pages mode.
- More than one physical edition per book isn't supported.

## Revisit when

Users keep correcting pages by hand on books without print page markers, or
need several physical editions of one book.
