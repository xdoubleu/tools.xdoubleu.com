import type { UserBook } from '@/lib/gen/backlog/v1/books_pb'
import { displayProgressPercent, PROGRESS_MODE_PAGES } from '@/lib/backlog/bookProgress'

export default function BookProgressBar({ userBook }: { userBook: UserBook }) {
  const percent = displayProgressPercent(userBook)
  const pageCount = userBook.book?.pageCount ?? 0
  const label =
    userBook.progressMode === PROGRESS_MODE_PAGES && pageCount > 0
      ? `${userBook.currentPage} / ${pageCount} pages`
      : `${percent}%`

  return (
    <div>
      <div
        className="h-2 w-full overflow-hidden rounded-full bg-surface"
        role="progressbar"
        aria-valuenow={percent}
        aria-valuemin={0}
        aria-valuemax={100}
      >
        <div
          className="h-full rounded-full bg-accent transition-[width] duration-300"
          style={{ width: `${percent}%` }}
        />
      </div>
      <p className="mt-1 text-xs text-muted">{label}</p>
    </div>
  )
}
