import type { UserBook } from '@/lib/gen/books/v1/library_pb'

interface YtdProgressPoint {
  label: string
  value: number
}

export interface YtdProgressResult {
  series: YtdProgressPoint[]
  total: number
}

/**
 * Cumulative year-to-date finishes (re-reads count again), starting at 0,
 * plus the total.
 */
export function ytdProgress(finished: UserBook[]): YtdProgressResult {
  const currentYear = new Date().getFullYear()

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
