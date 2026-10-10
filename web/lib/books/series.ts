import type { Book, GetSeriesResponse, LibraryResponse } from '@/lib/gen/books/v1/library_pb'
import { flattenLibrary } from '@/lib/books/bookShelves'

export interface SeriesSummary {
  name: string
  /** The user's books in the series. */
  owned: number
  read: number
  /** Main-volume count per Hardcover, else `owned`. */
  total: number
}

export function seriesHref(name: string): string {
  return `/books/series/${encodeURIComponent(name)}`
}

/** "#4", "#1.5"; empty for an unordered entry. */
export function formatSeriesPosition(position: number | undefined): string {
  return position === undefined ? '' : `#${position}`
}

/** "Discworld #4", or just the name when unordered; empty without a series. */
export function seriesLabel(book: Pick<Book, 'seriesName' | 'seriesPosition'>): string {
  if (!book.seriesName) return ''
  const pos = formatSeriesPosition(book.seriesPosition)
  return pos ? `${book.seriesName} ${pos}` : book.seriesName
}

/** One summary per series in the library, by name. */
export function buildSeriesSummaries(library: LibraryResponse | null | undefined): SeriesSummary[] {
  const byName = new Map<string, SeriesSummary>()
  for (const ub of flattenLibrary(library)) {
    const book = ub.book
    if (!book?.seriesName) continue
    const name = book.seriesName
    const s = byName.get(name) ?? { name, owned: 0, read: 0, total: 0 }
    s.owned++
    if (ub.status === 'read') s.read++
    s.total = Math.max(s.total, book.seriesTotal)
    byName.set(name, s)
  }
  return Array.from(byName.values())
    .map((s) => ({ ...s, total: Math.max(s.total, s.owned) }))
    .sort((a, b) => a.name.localeCompare(b.name))
}

/** The edit form's position: unset without a series or a valid number. */
export function parseSeriesPosition(name: string, raw: string): number | undefined {
  if (!name.trim() || raw.trim() === '') return undefined
  const n = Number(raw)
  return Number.isFinite(n) && n >= 0 ? n : undefined
}

/** "3 of 7 read · 4 in your library · 3 missing". */
export function seriesDescription(series: GetSeriesResponse): string {
  const owned = series.entries.filter((e) => e.userBook).length
  const read = series.entries.filter((e) => e.userBook?.status === 'read').length
  const missing = series.entries.length - owned
  const parts = [`${read} of ${Math.max(series.total, owned)} read`, `${owned} in your library`]
  if (missing > 0) parts.push(`${missing} missing`)
  return parts.join(' · ')
}
