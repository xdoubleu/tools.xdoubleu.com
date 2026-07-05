'use client'

import { useState } from 'react'
import { swrKeys } from '@/lib/swrKeys'
import { mutate } from 'swr'
import {
  useCatalogBooks,
  useResyncBooks,
  useResyncOpenLibrary,
  useSetBookISBN
} from '@/hooks/useBooks'
import { useProgressSocket } from '@/lib/progressSocket'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import ConfirmIsbnMergeDialog, {
  type IsbnMergeTarget
} from '@/components/books/ConfirmIsbnMergeDialog'
import {
  applyFilters,
  FILTER_KEYS,
  FILTER_LABELS,
  groupBooks,
  type CatalogGroup,
  type FilterKey
} from '@/components/books/catalogGroups'

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
  // Merge-on-collision state: set when the entered ISBN belongs to another entry.
  const [mergeTarget, setMergeTarget] = useState<IsbnMergeTarget | null>(null)

  const { isRefreshing, processed, total } = useProgressSocket(
    'books',
    'resync-openlibrary',
    triggerResync,
    () => {
      void mutate(swrKeys.bookCatalog)
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

    // Check whether the entered ISBN already belongs to a different catalog entry.
    const allBooks = data?.books ?? []
    const groupIdSet = new Set(group.ids)
    const existing = allBooks.find((b) => b.isbn13 === normalized && !groupIdSet.has(b.id))
    if (existing) {
      // ISBN collision: prompt to merge instead of setting directly.
      setMergeTarget({
        winnerId: existing.id,
        winnerTitle: existing.title,
        winnerAuthors: [...existing.authors],
        loserIds: group.ids,
        loserTitle: group.title,
        loserAuthors: [...group.authors]
      })
      return
    }

    setSavingIsbnKey(group.key)
    try {
      await setBookISBN(group.representativeId, normalized)
      setIsbnInputs((prev) => ({ ...prev, [group.key]: '' }))
      void mutate(swrKeys.bookCatalog)
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
                      {/* Only surface UniCat when neither OL nor GB found it, or
                          when UniCat did find it (positive signal). */}
                      {group.unicatStatus &&
                        (group.unicatStatus === 'found' ||
                          (group.openlibraryStatus !== 'found' &&
                            group.googlebooksStatus !== 'found')) && (
                          <span className="text-xs text-muted">
                            UC: <ResyncStatusBadge status={group.unicatStatus} />
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

      <ConfirmIsbnMergeDialog target={mergeTarget} onClose={() => setMergeTarget(null)} />
    </div>
  )
}
