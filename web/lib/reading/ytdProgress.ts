import type { UserBook } from '@/lib/gen/reading/v1/library_pb'

interface YtdProgressPoint {
  label: string
  value: number
}

export interface YtdProgressResult {
  series: YtdProgressPoint[]
  total: number
}

/**
 * Computes year-to-date reading progress from finished user books.
 *
 * Every finish event (entry in finishedAt) that falls within the current
 * calendar year is counted — a re-read of the same book counts twice.
 * Returns a cumulative series starting at 0 on the first finish event of
 * the year, plus the total count for the stat card.
 */
export function ytdProgress(finished: UserBook[]): YtdProgressResult {
  const currentYear = new Date().getFullYear()

  // Collect all finish dates in the current year (YYYY-MM-DD label).
  const dateCounts = new Map<string, number>()

  for (const ub of finished) {
    for (const iso of ub.finishedAt) {
      const d = new Date(iso)
      if (d.getFullYear() !== currentYear) continue
      const label = d.toISOString().slice(0, 10)
      dateCounts.set(label, (dateCounts.get(label) ?? 0) + 1)
    }
  }

  if (dateCounts.size === 0) {
    return { series: [], total: 0 }
  }

  const sortedDates = [...dateCounts.keys()].sort()

  let cumulative = 0
  const series: YtdProgressPoint[] = sortedDates.map((label) => {
    cumulative += dateCounts.get(label)!
    return { label, value: cumulative }
  })

  return { series, total: cumulative }
}
