import type { UserBook } from '@/lib/gen/books/v1/library_pb'
import {
  displayProgressPercent,
  ownsPhysical,
  pageCountOf,
  PROGRESS_MODE_PAGES
} from '@/lib/books/bookProgress'

/** Progress bar; a physical copy also read as an EPUB shows its page too. */
export default function BookProgressBar({ userBook }: { userBook: UserBook }) {
  const percent = displayProgressPercent(userBook)
  const pageCount = pageCountOf(userBook)
  let label = `${percent}%`
  if (userBook.progressMode === PROGRESS_MODE_PAGES && pageCount > 0) {
    label = `${userBook.currentPage} / ${pageCount} pages`
  } else if (
    ownsPhysical(userBook) &&
    userBook.formats.includes('epub') &&
    userBook.currentPage > 0
  ) {
    label = `${percent}% · p. ~${userBook.currentPage}`
  }

  return (
    <div>
      <div
        className="h-2 w-full overflow-hidden rounded-sm bg-surface"
        role="progressbar"
        aria-valuenow={percent}
        aria-valuemin={0}
        aria-valuemax={100}
      >
        <div
          className="h-full rounded-sm bg-accent transition-[width] duration-300"
          style={{ width: `${percent}%` }}
        />
      </div>
      <p className="mt-1 text-xs text-muted">{label}</p>
    </div>
  )
}
