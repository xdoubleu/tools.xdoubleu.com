import type { UserBook } from '@/lib/gen/books/v1/library_pb'

export const PROGRESS_MODE_PAGES = 'pages'
export const PROGRESS_MODE_PERCENT = 'percent'

// defaultProgressMode returns the progress mode to use when opening an edit
// form. If the book already has a stored mode, that is respected. Otherwise:
// digital-only books default to percent (no page-count available), physical or
// mixed books default to pages.
export function defaultProgressMode(userBook: UserBook): string {
  if (userBook.progressMode) return userBook.progressMode
  const digital = userBook.tags.includes('own-digital')
  const physical = userBook.tags.includes('own-physical')
  if (digital && !physical) return PROGRESS_MODE_PERCENT
  return PROGRESS_MODE_PAGES
}

function clampPercent(p: number): number {
  if (p < 0) return 0
  if (p > 100) return 100
  return Math.round(p)
}

// displayProgressPercent returns the reading progress as a 0-100 percentage. In
// percent mode the stored percent is authoritative; in pages mode it is derived
// from the current page over the book's total page count. It returns 0 when the
// page count is unknown so callers never divide by zero.
export function displayProgressPercent(userBook: UserBook): number {
  if (userBook.progressMode === PROGRESS_MODE_PERCENT) {
    return clampPercent(userBook.progressPercent)
  }
  const pageCount = userBook.book?.pageCount ?? 0
  if (pageCount <= 0) return 0
  return clampPercent((userBook.currentPage / pageCount) * 100)
}
