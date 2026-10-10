// Format-neutral reading positions (adr-0029). An EPUB offset counts UTF-16
// code units of the section body's textContent.

/** `{href, offset}` for an EPUB (href relative to the zip root), `{page}` (1-based) for a PDF. */
export type ReaderPosition = { href: string; offset: number } | { page: number }

/** Where to reopen a book: the stored position, else the stored percent. */
export interface ReaderResume {
  position?: ReaderPosition
  /** The same place in the other file of a PDF-sourced book, as the server translates it. */
  alsoAt?: ReaderPosition
  percent: number
}

/** Where the reader is; `section` is the spine index, or the page index for a PDF. */
export interface ReaderLocation {
  fraction: number
  section: number
  tocLabel?: string
  tocHref?: string
  /** Set only on locations passed to `onRelocate`. */
  position?: ReaderPosition
}

/** A contents entry; `href` is missing on grouping entries that don't link anywhere. */
export interface ReaderTocItem {
  label: string
  href?: string
  subitems?: ReaderTocItem[]
}

/** Reads GetReadingState's state; a missing state opens at the start. */
export function resumeFromState(
  state: { percent: number; position?: { href: string; offset: number; page: number } } | undefined
): ReaderResume {
  const percent = state?.percent ?? 0
  const p = state?.position
  const epub = p?.href ? { href: p.href, offset: p.offset } : undefined
  if (p?.page) return { position: { page: p.page }, ...(epub && { alsoAt: epub }), percent }
  if (epub) return { position: epub, percent }
  return { percent }
}
