import type { SeriesSummary } from '@/lib/books/series'
import { ProgressBar } from '@/components/ui/progress-bar'

/** Read-so-far per series. Plain text, as the public dashboard shares it. */
export default function BooksSeriesProgress({ series }: { series: SeriesSummary[] }) {
  return (
    <ul className="flex flex-col gap-3">
      {series.map((s) => {
        return (
          <li key={s.name}>
            <div className="flex items-baseline justify-between gap-2 text-sm">
              <span className="truncate">{s.name}</span>
              <span className="shrink-0 text-xs text-muted">
                {s.read} of {s.total} read
              </span>
            </div>
            <ProgressBar
              className="mt-1"
              value={Math.round((s.read / s.total) * 100)}
              aria-label={`${s.name} progress`}
            />
          </li>
        )
      })}
    </ul>
  )
}
