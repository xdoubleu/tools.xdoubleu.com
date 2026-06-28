'use client'

import { useState } from 'react'
import { mutate } from 'swr'
import {
  useCatalogBooks,
  useResyncBooks,
  useResyncOpenLibrary,
  useSetBookISBN
} from '@/hooks/useBacklog'
import { useProgressSocket } from '@/lib/backlog/progressSocket'
import { normalizeTitle, normalizeAuthor } from '@/lib/backlog/normalizeBook'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import type { CatalogBookStatus } from '@/lib/gen/backlog/v1/books_pb'

// ---------------------------------------------------------------------------
// Filter chip types
// ---------------------------------------------------------------------------

type FilterKey = 'missing_isbn' | 'not_in_ol' | 'not_in_gb'

const FILTER_KEYS: FilterKey[] = ['missing_isbn', 'not_in_ol', 'not_in_gb']

const FILTER_LABELS: Record<FilterKey, string> = {
  missing_isbn: 'Missing ISBN',
  not_in_ol: 'Not in Open Library',
  not_in_gb: 'Not in Google Books'
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

interface CatalogGroup {
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
function groupBooks(books: CatalogBookStatus[]): CatalogGroup[] {
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
      lastResyncAt: statusSource.lastResyncAt,
      count: members.length
    })
  }

  return groups.sort((a, b) => a.title.localeCompare(b.title))
}

// ---------------------------------------------------------------------------
// Filter logic
// ---------------------------------------------------------------------------

function matchesFilter(group: CatalogGroup, filter: FilterKey): boolean {
  switch (filter) {
    case 'missing_isbn':
      return !group.isbn13
    case 'not_in_ol':
      return group.openlibraryStatus === 'not_found'
    case 'not_in_gb':
      // Only flag when Open Library also did not find it — a group already
      // sourced from OL has its metadata covered, so GB absence is not actionable.
      return group.googlebooksStatus === 'not_found' && group.openlibraryStatus !== 'found'
  }
}

function applyFilters(groups: CatalogGroup[], active: Set<FilterKey>): CatalogGroup[] {
  if (active.size === 0) return groups
  return groups.filter((g) => [...active].some((f) => matchesFilter(g, f)))
}

// ---------------------------------------------------------------------------
// Sub-components
// ---------------------------------------------------------------------------

function FilterChip({
  label,
  active,
  onClick
}: {
  label: string
  active: boolean
  onClick: () => void
}) {
  return (
    <button
      type="button"
      onClick={onClick}
      className={`rounded-lg border px-3 py-1 text-xs font-medium transition-colors ${
        active
          ? 'border-accent bg-accent/10 text-accent'
          : 'border-border bg-surface text-muted hover:bg-hover'
      }`}
    >
      {label}
    </button>
  )
}

function ResyncStatusBadge({ status }: { status: string }) {
  if (!status) return <span className="text-xs text-muted">Never synced</span>
  if (status === 'found') return <span className="text-xs text-success">Found</span>
  return <span className="text-xs text-danger">Not found</span>
}

// ---------------------------------------------------------------------------
// Main component
// ---------------------------------------------------------------------------

export default function SelectiveResync() {
  const { data, isLoading } = useCatalogBooks()
  const triggerResync = useResyncOpenLibrary()
  const resyncBooks = useResyncBooks()
  const setBookISBN = useSetBookISBN()

  const [activeFilters, setActiveFilters] = useState<Set<FilterKey>>(new Set())
  const [force, setForce] = useState(false)
  const [resyncingKey, setResyncingKey] = useState<string | null>(null)
  const [error, setError] = useState<string | null>(null)
  // Per-group ISBN input values and per-group saving state.
  const [isbnInputs, setIsbnInputs] = useState<Record<string, string>>({})
  const [savingIsbnKey, setSavingIsbnKey] = useState<string | null>(null)

  const { isRefreshing, processed, total } = useProgressSocket(
    'resync-openlibrary',
    triggerResync,
    () => {
      void mutate('/backlog/books/catalog')
      setResyncingKey(null)
    }
  )

  const allGroups = groupBooks(data?.books ?? [])
  const filtered = applyFilters(allGroups, activeFilters)

  function toggleFilter(key: FilterKey) {
    setActiveFilters((prev) => {
      const next = new Set(prev)
      if (next.has(key)) next.delete(key)
      else next.add(key)
      return next
    })
  }

  async function handleResync(ids: string[], key: string) {
    setError(null)
    setResyncingKey(key)
    try {
      await resyncBooks(ids, force)
    } catch (e) {
      setError(e instanceof Error ? e.message : 'Resync failed.')
      setResyncingKey(null)
    }
  }

  async function handleSetISBN(group: CatalogGroup) {
    const raw = isbnInputs[group.key] ?? ''
    const normalized = raw.replace(/[\s-]/g, '')
    if (!/^\d{13}$/.test(normalized)) {
      setError('ISBN must be exactly 13 digits.')
      return
    }
    setError(null)
    setSavingIsbnKey(group.key)
    try {
      await setBookISBN(group.representativeId, normalized)
      setIsbnInputs((prev) => ({ ...prev, [group.key]: '' }))
      void mutate('/backlog/books/catalog')
    } catch (e) {
      setError(e instanceof Error ? e.message : 'Failed to set ISBN.')
    } finally {
      setSavingIsbnKey(null)
    }
  }

  return (
    <div>
      {/* Filter chips + Force re-fetch toggle */}
      <div className="mb-3 flex flex-wrap items-center gap-2">
        {FILTER_KEYS.map((key) => (
          <FilterChip
            key={key}
            label={FILTER_LABELS[key]}
            active={activeFilters.has(key)}
            onClick={() => toggleFilter(key)}
          />
        ))}
        {activeFilters.size > 0 && (
          <button
            type="button"
            onClick={() => setActiveFilters(new Set())}
            className="text-xs text-muted underline hover:text-fg"
          >
            Clear filters
          </button>
        )}
        <label className="ml-auto flex cursor-pointer items-center gap-2 text-sm text-fg">
          <input
            type="checkbox"
            checked={force}
            onChange={(e) => setForce(e.target.checked)}
            aria-label="Force re-fetch"
            className="h-4 w-4 rounded"
          />
          Force re-fetch
        </label>
      </div>

      {/* Book list */}
      {isLoading ? (
        <p className="text-xs text-muted">Loading catalog…</p>
      ) : filtered.length === 0 ? (
        <p className="text-xs text-muted">
          {activeFilters.size > 0
            ? 'No books match the active filters.'
            : 'No books in the catalog.'}
        </p>
      ) : (
        <div className="overflow-hidden rounded-xl border border-border bg-card">
          {/* Header row */}
          <div className="flex items-center gap-3 border-b border-border bg-surface px-3 py-2">
            <span className="text-xs text-muted">
              {filtered.length} book{filtered.length !== 1 ? 's' : ''}
            </span>
          </div>

          {/* Rows */}
          <ul className="divide-y divide-border">
            {filtered.map((group) => (
              <li key={group.key} className="flex items-start gap-3 px-3 py-2 hover:bg-hover">
                <div className="min-w-0 flex-1">
                  <p className="truncate text-sm font-medium text-fg">
                    {group.title}
                    {group.count > 1 && (
                      <span className="ml-1 text-xs font-normal text-muted">x{group.count}</span>
                    )}
                  </p>
                  {group.authors.length > 0 && (
                    <p className="truncate text-xs text-muted">{group.authors.join(', ')}</p>
                  )}
                  <div className="mt-1 flex flex-wrap gap-x-3 gap-y-0.5">
                    {!group.isbn13 && <span className="text-xs text-warn">No ISBN</span>}
                    {!group.hasCover && <span className="text-xs text-muted">No cover</span>}
                    {!group.hasDescription && (
                      <span className="text-xs text-muted">No description</span>
                    )}
                    {!group.hasPageCount && (
                      <span className="text-xs text-muted">No page count</span>
                    )}
                  </div>
                  {/* Inline ISBN setter — only shown for books missing an ISBN */}
                  {!group.isbn13 && (
                    <div className="mt-2 flex items-center gap-2">
                      <Input
                        type="text"
                        inputMode="numeric"
                        placeholder="ISBN-13"
                        value={isbnInputs[group.key] ?? ''}
                        onChange={(e) =>
                          setIsbnInputs((prev) => ({ ...prev, [group.key]: e.target.value }))
                        }
                        className="h-7 w-36 text-xs"
                        aria-label={`ISBN-13 for ${group.title}`}
                      />
                      <Button
                        type="button"
                        variant="secondary"
                        size="sm"
                        disabled={savingIsbnKey === group.key}
                        onClick={() => void handleSetISBN(group)}
                      >
                        {savingIsbnKey === group.key ? 'Saving…' : 'Set ISBN'}
                      </Button>
                    </div>
                  )}
                  {group.lastResyncAt && (
                    <div className="mt-1 flex flex-wrap gap-x-4">
                      <span className="text-xs text-muted">
                        OL: <ResyncStatusBadge status={group.openlibraryStatus} />
                      </span>
                      {/* Only surface GB when it adds signal: OL didn't find it, or
                          GB did find it. Hiding a "Not found" GB badge when OL
                          already sourced the book avoids misleading noise. */}
                      {(group.openlibraryStatus !== 'found' ||
                        group.googlebooksStatus === 'found') && (
                        <span className="text-xs text-muted">
                          GB: <ResyncStatusBadge status={group.googlebooksStatus} />
                        </span>
                      )}
                    </div>
                  )}
                </div>
                <Button
                  type="button"
                  variant="secondary"
                  size="sm"
                  disabled={isRefreshing}
                  onClick={() => void handleResync(group.ids, group.key)}
                >
                  {resyncingKey === group.key ? 'Resyncing…' : 'Resync'}
                </Button>
              </li>
            ))}
          </ul>
        </div>
      )}

      {/* Progress bar */}
      {isRefreshing && total !== null && (
        <div className="mt-3">
          <div className="mb-1 flex justify-between text-xs text-muted">
            <span>Resyncing…</span>
            <span>
              {processed ?? 0} / {total}
            </span>
          </div>
          <div className="h-2 w-full overflow-hidden rounded-full bg-border">
            <div
              className="h-full rounded-full bg-fg transition-all duration-300"
              style={{
                width: `${total > 0 ? (((processed ?? 0) / total) * 100).toFixed(1) : 0}%`
              }}
            />
          </div>
        </div>
      )}

      {error && <p className="mt-2 text-sm text-danger">{error}</p>}
    </div>
  )
}
