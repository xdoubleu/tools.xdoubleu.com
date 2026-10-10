import type { SeriesSummary } from '@/lib/books/series'

/** Read-so-far per series. Plain text, as the public dashboard shares it. */
export default function BooksSeriesProgress({ series }: { series: SeriesSummary[] }) {
  return (
    <ul className="flex flex-col gap-3">
      {series.map((s) => {
        const percent = Math.round((s.read / s.total) * 100)
        return (
          <li key={s.name}>
            <div className="flex items-baseline justify-between gap-2 text-sm">
              <span className="truncate">{s.name}</span>
              <span className="shrink-0 text-xs text-muted">
                {s.read} of {s.total} read
              </span>
            </div>
            <div
              className="mt-1 h-2 w-full overflow-hidden rounded-sm bg-surface"
              role="progressbar"
              aria-label={`${s.name} progress`}
              aria-valuenow={percent}
              aria-valuemin={0}
              aria-valuemax={100}
            >
              <div className="h-full rounded-sm bg-accent" style={{ width: `${percent}%` }} />
            </div>
          </li>
        )
      })}
    </ul>
  )
}
