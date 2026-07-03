import { normalizeTitle, normalizeAuthor } from '@/lib/books/normalizeBook'
import type { CatalogBookStatus } from '@/lib/gen/books/v1/catalog_pb'

// ---------------------------------------------------------------------------
// Filter chip types
// ---------------------------------------------------------------------------

export type FilterKey = 'missing_isbn' | 'not_in_ol' | 'not_in_gb' | 'not_in_uc'

export const FILTER_KEYS: FilterKey[] = ['missing_isbn', 'not_in_ol', 'not_in_gb', 'not_in_uc']

export const FILTER_LABELS: Record<FilterKey, string> = {
  missing_isbn: 'Missing ISBN',
  not_in_ol: 'Not in Open Library',
  not_in_gb: 'Not in Google Books',
  not_in_uc: 'Not in UniCat'
}

// ---------------------------------------------------------------------------
// Catalog group — one or more raw catalog rows collapsed for display.
//
// Books with an ISBN13 are never collapsed (each has its own group).
// ISBN-less books sharing a normalised title + first-author last name are
// collapsed into one group so the resync list does not surface them as
// duplicates. The underlying catalog rows are left untouched; this is purely
// a display-level dedup.
// ---------------------------------------------------------------------------

export interface CatalogGroup {
  key: string
  /** All catalog row IDs in this display group. */
  ids: string[]
  /** The catalog row ID with the most metadata — used for ISBN assignment. */
  representativeId: string
  title: string
  authors: string[]
  isbn13: string
  hasCover: boolean
  hasDescription: boolean
  hasPageCount: boolean
  openlibraryStatus: string
  googlebooksStatus: string
  unicatStatus: string
  lastResyncAt: string
  count: number
}

function metaScore(b: CatalogBookStatus): number {
  return (b.hasCover ? 1 : 0) + (b.hasDescription ? 1 : 0) + (b.hasPageCount ? 1 : 0)
}

// groupBooks collapses catalog rows that represent the same book.
//
// Rows with an ISBN-13 are keyed by ISBN (Postgres already deduplicates them at
// the catalog level, so each ISBN produces exactly one row in practice).
//
// ISBN-less rows are grouped via union-find: two rows are the same book when
// they share a normalised title AND at least one normalised author last name.
// This mirrors the backend FindDuplicateGroups heuristic so the display matches
// the duplicate-detection logic.  Rows with no normalisable title or no authors
// fall back to a singleton `id:` key and are never collapsed.
export function groupBooks(books: CatalogBookStatus[]): CatalogGroup[] {
  // --- phase 1: assign an initial bucket key to each row ---
  // ISBN rows get a stable isbn: key.
  // ISBN-less rows are bucketed under every (normTitle, normAuthorLastName) pair
  // they produce; if they produce none, they get a singleton id: key.
  const parent: number[] = books.map((_, i) => i)

  function find(x: number): number {
    while (parent[x] !== x) {
      parent[x] = parent[parent[x]] // path compression
      x = parent[x]
    }
    return x
  }

  function union(a: number, b: number): void {
    const ra = find(a)
    const rb = find(b)
    if (ra !== rb) parent[rb] = ra
  }

  // isbn: rows union by ISBN key.
  const isbnFirst = new Map<string, number>()
  // noisbn: rows union by title+author bucket key.
  const titleAuthorFirst = new Map<string, number>()

  for (let i = 0; i < books.length; i++) {
    const b = books[i]
    if (b.isbn13) {
      const key = `isbn:${b.isbn13}`
      const first = isbnFirst.get(key)
      if (first === undefined) isbnFirst.set(key, i)
      else union(first, i)
    } else {
      const nt = normalizeTitle(b.title)
      if (!nt) continue // no usable title → singleton
      for (const a of b.authors) {
        const na = normalizeAuthor(a)
        if (!na) continue
        const key = `${nt}\x00${na}`
        const first = titleAuthorFirst.get(key)
        if (first === undefined) titleAuthorFirst.set(key, i)
        else union(first, i)
      }
    }
  }

  // --- phase 2: collect groups by root ---
  const rootMembers = new Map<number, CatalogBookStatus[]>()
  for (let i = 0; i < books.length; i++) {
    const r = find(i)
    const arr = rootMembers.get(r)
    if (arr) arr.push(books[i])
    else rootMembers.set(r, [books[i]])
  }

  // --- phase 3: build CatalogGroup for each root ---
  const groups: CatalogGroup[] = []
  for (const [root, members] of rootMembers) {
    // Representative: the member with the most metadata fields populated.
    const rep = members.reduce((best, m) => (metaScore(m) >= metaScore(best) ? m : best))

    // Status source: the most recently resynced member.
    const resynced = members
      .filter((m) => m.lastResyncAt)
      .sort((a, b) => (a.lastResyncAt > b.lastResyncAt ? 1 : -1))
    const statusSource = resynced.at(-1) ?? rep

    // Derive a stable group key.
    const groupKey = rep.isbn13
      ? `isbn:${rep.isbn13}`
      : (() => {
          const nt = normalizeTitle(rep.title)
          const firstAuthor = rep.authors[0] ? normalizeAuthor(rep.authors[0]) : ''
          return nt && firstAuthor ? `noisbn:${nt}\x00${firstAuthor}` : `id:${root}`
        })()

    groups.push({
      key: groupKey,
      ids: members.map((m) => m.id),
      representativeId: rep.id,
      title: rep.title,
      authors: [...rep.authors],
      isbn13: rep.isbn13,
      hasCover: members.some((m) => m.hasCover),
      hasDescription: members.some((m) => m.hasDescription),
      hasPageCount: members.some((m) => m.hasPageCount),
      openlibraryStatus: statusSource.openlibraryStatus,
      googlebooksStatus: statusSource.googlebooksStatus,
      unicatStatus: statusSource.unicatStatus,
      lastResyncAt: statusSource.lastResyncAt,
      count: members.length
    })
  }

  return groups.sort((a, b) => a.title.localeCompare(b.title))
}

// ---------------------------------------------------------------------------
// Filter logic
// ---------------------------------------------------------------------------

export function matchesFilter(group: CatalogGroup, filter: FilterKey): boolean {
  switch (filter) {
    case 'missing_isbn':
      return !group.isbn13
    case 'not_in_ol':
      return group.openlibraryStatus === 'not_found'
    case 'not_in_gb':
      // Only flag when Open Library also did not find it — a group already
      // sourced from OL has its metadata covered, so GB absence is not actionable.
      return group.googlebooksStatus === 'not_found' && group.openlibraryStatus !== 'found'
    case 'not_in_uc':
      // Only flag when neither OL nor GB found it — UniCat is a last-resort
      // fallback for Dutch/Flemish books, so its absence is only actionable
      // when the other providers also came up empty.
      return (
        group.unicatStatus === 'not_found' &&
        group.openlibraryStatus !== 'found' &&
        group.googlebooksStatus !== 'found'
      )
  }
}

export function applyFilters(groups: CatalogGroup[], active: Set<FilterKey>): CatalogGroup[] {
  if (active.size === 0) return groups
  return groups.filter((g) => [...active].some((f) => matchesFilter(g, f)))
}
