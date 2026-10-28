import { monthLabel } from '@/lib/dates'

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

export const TRACKS = [
  { value: 'more', label: 'More of what you like' },
  { value: 'broaden', label: 'Broaden your horizon' }
] as const

/** The view that lists every saved pick instead of a track's latest. */
export const SAVED_VIEW = 'saved'

/** The hostname of url without "www.", or url itself when it doesn't parse. */
export function hostLabel(url: string): string {
  try {
    return new URL(url).hostname.replace(/^www\./, '')
  } catch {
    return url
  }
}

/** The track's label, or the value itself when unknown. */
export function trackLabel(value: string): string {
  return TRACKS.find((t) => t.value === value)?.label ?? value
}

// Stryker disable StringLiteral: labels are presentation only.
export const FEEDBACK = [
  { value: 'hit', label: 'Hit' },
  { value: 'miss', label: 'Miss' },
  { value: 'not_interested', label: 'Not interested' },
  { value: 'known', label: 'Already know it' }
] as const
// Stryker restore StringLiteral

/** "67% (2 of 3)" for hits out of hits plus misses. */
export function hitRateLabel(hits: number, misses: number): string {
  const total = hits + misses
  if (total === 0) return 'No hits or misses yet'
  return `${Math.round((hits / total) * 100)}% (${hits} of ${total})`
}
