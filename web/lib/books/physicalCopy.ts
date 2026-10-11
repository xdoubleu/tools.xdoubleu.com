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

export interface ChapterOption {
  value: string
  title: string
  href: string
  offset: number
}

export function chapterOptions(chapters: PhysicalChapter[]): ChapterOption[] {
  return chapters.map((c) => {
    const href = c.position?.href ?? ''
    const offset = c.position?.offset ?? 0
    return { value: chapterValue(href, offset), title: c.title, href, offset }
  })
}

/** A new anchor at the first chapter, its page still to fill in. */
export function newAnchor(options: ChapterOption[]): PhysicalAnchorDraft {
  return { page: 0, href: options[0]?.href ?? '', offset: options[0]?.offset ?? 0 }
}

/** anchor moved to the chapter with select value `value`. */
export function anchorAtChapter(
  anchor: PhysicalAnchorDraft,
  options: ChapterOption[],
  value: string
): PhysicalAnchorDraft {
  const option = options.find((o) => o.value === value)
  return option ? { ...anchor, href: option.href, offset: option.offset } : anchor
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
