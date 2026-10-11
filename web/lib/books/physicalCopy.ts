import type {
  BookReadingStateData,
  PhysicalChapter,
  PhysicalCopy
} from '@/lib/gen/books/v1/library_pb'

export interface PhysicalAnchorDraft {
  page: number
  href: string
  offset: number
}

/** Editable copy details; anchors without a page are dropped on save. */
export interface PhysicalCopyDraft {
  pageCount: number
  textStartPage: number
  anchors: PhysicalAnchorDraft[]
}

export function copyDraft(copy: PhysicalCopy | undefined): PhysicalCopyDraft {
  return {
    pageCount: copy?.pageCount ?? 0,
    textStartPage: copy?.textStartPage || 1,
    anchors: (copy?.anchors ?? []).map((a) => ({
      page: a.page,
      href: a.position?.href ?? '',
      offset: a.position?.offset ?? 0
    }))
  }
}

/** The draft as saved: only anchors with a page and a position. */
export function savableDraft(draft: PhysicalCopyDraft): PhysicalCopyDraft {
  return { ...draft, anchors: draft.anchors.filter((a) => a.page > 0 && a.href !== '') }
}

/** A chapter's position as one select value. */
export function chapterValue(href: string, offset: number): string {
  return `${href}#${offset}`
}

export function chapterAt(chapters: PhysicalChapter[], value: string) {
  return chapters.find(
    (c) => chapterValue(c.position?.href ?? '', c.position?.offset ?? 0) === value
  )
}

export interface ContinueHint {
  page: number
  excerpt: string
}

/** Where to continue in the physical copy when the newest position is digital. */
export function continueHint(state: BookReadingStateData | undefined): ContinueHint | null {
  if (!state || state.source === 'physical' || state.physicalPage <= 0) return null
  return { page: state.physicalPage, excerpt: state.physicalExcerpt }
}
