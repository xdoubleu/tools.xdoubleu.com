// Format-neutral reading positions (adr-0029) and their foliate-js ranges.
// An EPUB offset counts UTF-16 code units of the section body's textContent.

/** `{href, offset}` for EPUB/KEPUB (href relative to the zip root), `{page}` (1-based) for PDF. */
export type ReaderPosition = { href: string; offset: number } | { page: number }

/** Where to reopen a book: the stored position, else the stored percent. */
export interface ReaderResume {
  position?: ReaderPosition
  percent: number
}

/** A foliate-js section: EPUB ids are manifest hrefs, PDF ids page indexes. */
export interface ReaderSection {
  id: string | number
}

export type ResumeTarget =
  { index: number; anchor?: (doc: Document) => Range } | { fraction: number } | null

/** Text offset of the boundary point (node, offset) within the body. */
export function textOffsetAt(node: Node, offset: number): number {
  const doc = node.ownerDocument
  if (!doc?.body) return 0
  const range = doc.createRange()
  range.setStart(doc.body, 0)
  range.setEnd(node, offset)
  return range.toString().length
}

/** A collapsed range at a body text offset (at the start of a node on a boundary), clamped to the end. */
export function rangeAtTextOffset(doc: Document, offset: number): Range {
  const range = doc.createRange()
  range.setStart(doc.body, 0)
  const walker = doc.createTreeWalker(doc.body, NodeFilter.SHOW_TEXT)
  let remaining = offset
  let last: Text | null = null
  for (let node = walker.nextNode(); node; node = walker.nextNode()) {
    // eslint-disable-next-line @typescript-eslint/no-unsafe-type-assertion -- SHOW_TEXT only yields Text nodes
    last = node as Text
    if (remaining < last.length) {
      range.setStart(last, remaining)
      return range
    }
    remaining -= last.length
  }
  if (last) range.setStart(last, last.length)
  return range
}

/** The neutral position for a relocate in section `index` starting at `range`. */
export function positionAt(
  sections: ReaderSection[] | undefined,
  index: number,
  range?: Range
): ReaderPosition | undefined {
  const section = sections?.[index]
  if (!section) return undefined
  if (typeof section.id === 'number') return { page: index + 1 }
  const offset = range ? textOffsetAt(range.startContainer, range.startOffset) : 0
  return { href: section.id, offset }
}

/** Where to open: the stored position if it fits this file, else the percent. */
export function resumeTarget(
  sections: ReaderSection[] | undefined,
  resume: ReaderResume | undefined
): ResumeTarget {
  const position = resume?.position
  if (sections && position) {
    if ('page' in position) {
      const index = position.page - 1
      if (typeof sections[index]?.id === 'number') return { index }
    } else {
      const index = sections.findIndex((s) => s.id === position.href)
      if (index >= 0) {
        const { offset } = position
        return { index, anchor: (doc) => rangeAtTextOffset(doc, offset) }
      }
    }
  }
  const percent = resume?.percent ?? 0
  return percent > 0 ? { fraction: percent / 100 } : null
}

/** Reads GetReadingState's state; a missing state opens at the start. */
export function resumeFromState(
  state: { percent: number; position?: { href: string; offset: number; page: number } } | undefined
): ReaderResume {
  const percent = state?.percent ?? 0
  const p = state?.position
  if (p?.page) return { position: { page: p.page }, percent }
  if (p?.href) return { position: { href: p.href, offset: p.offset }, percent }
  return { percent }
}
