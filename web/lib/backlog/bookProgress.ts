import type { UserBook } from '@/lib/gen/backlog/v1/books_pb'

export const PROGRESS_MODE_PAGES = 'pages'
export const PROGRESS_MODE_PERCENT = 'percent'

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
