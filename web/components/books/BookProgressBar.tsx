import type { UserBook } from '@/lib/gen/books/v1/library_pb'
import { ProgressBar } from '@/components/ui/progress-bar'
import {
  displayProgressPercent,
  ownsPhysical,
  pageCountOf,
  PROGRESS_MODE_PAGES
} from '@/lib/books/bookProgress'

/**
 * Progress bar; a physical copy also read as an EPUB shows its page too, when
 * a page count lets digital positions map to it.
 */
export default function BookProgressBar({ userBook }: { userBook: UserBook }) {
  const percent = displayProgressPercent(userBook)
  const pageCount = pageCountOf(userBook)
  let label = `${percent}%`
  if (userBook.progressMode === PROGRESS_MODE_PAGES && pageCount > 0) {
    label = `${userBook.currentPage} / ${pageCount} pages`
  } else if (
    ownsPhysical(userBook) &&
    userBook.formats.includes('epub') &&
    pageCount > 0 &&
    userBook.currentPage > 0
  ) {
    label = `${percent}% · p. ~${userBook.currentPage}`
  }

  return (
    <div>
      <ProgressBar value={percent} aria-label="Reading progress" valueText={label} />
      <p className="mt-1 text-xs text-muted">{label}</p>
    </div>
  )
}
