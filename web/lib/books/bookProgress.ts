import type { UserBook } from '@/lib/gen/books/v1/library_pb'

export const PROGRESS_MODE_PAGES = 'pages'
export const PROGRESS_MODE_PERCENT = 'percent'

// defaultProgressMode: the stored mode, else percent for digital-only books
// (no page count) and pages otherwise.
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

// displayProgressPercent returns 0-100: stored percent, or current page over
// page count (0 when unknown).
export function displayProgressPercent(userBook: UserBook): number {
  if (userBook.progressMode === PROGRESS_MODE_PERCENT) {
    return clampPercent(userBook.progressPercent)
  }
  const pageCount = userBook.book?.pageCount ?? 0
  if (pageCount <= 0) return 0
  return clampPercent((userBook.currentPage / pageCount) * 100)
}
