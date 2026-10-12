import { monthLabel } from '@/lib/movies/format'

export interface DomainInfo {
  id: string
  label: string
  /** CSS colour for the domain's chart series. */
  color: string
}

// Stryker disable StringLiteral: labels and colours are presentation only.
export const DOMAINS: readonly DomainInfo[] = [
  { id: 'books', label: 'Books', color: 'rgb(var(--series-1-rgb))' },
  { id: 'feeds', label: 'Feeds', color: 'rgb(var(--series-2-rgb))' },
  { id: 'games', label: 'Games', color: 'rgb(var(--series-3-rgb))' },
  { id: 'movies', label: 'Movies', color: 'rgb(var(--series-4-rgb))' },
  { id: 'series', label: 'Series', color: 'rgb(var(--series-5-rgb))' },
  { id: 'podcasts', label: 'Podcasts', color: 'rgb(var(--series-6-rgb))' }
]
// Stryker restore StringLiteral

export function domainLabel(id: string): string {
  return DOMAINS.find((d) => d.id === id)?.label ?? id
}

/** Badge variant and words for a -2..2 signal. */
export function signalBadge(signal: number): {
  label: string
  variant: 'success' | 'secondary' | 'danger'
} {
  if (signal >= 2) return { label: 'Loved', variant: 'success' }
  if (signal === 1) return { label: 'Liked', variant: 'success' }
  if (signal === 0) return { label: 'Neutral', variant: 'secondary' }
  if (signal === -1) return { label: 'Disliked', variant: 'danger' }
  return { label: 'Hated', variant: 'danger' }
}

interface MonthCountsLike {
  month: string
  counts: { domain: string; count: number }[]
}

/** Chart rows: one per month, one numeric column per domain label. */
export function monthRows(months: MonthCountsLike[]): Record<string, string | number>[] {
  return months.map((m) => {
    const row: Record<string, string | number> = { month: monthLabel(m.month) }
    for (const d of DOMAINS) row[d.label] = 0
    for (const c of m.counts) row[domainLabel(c.domain)] = c.count
    return row
  })
}
