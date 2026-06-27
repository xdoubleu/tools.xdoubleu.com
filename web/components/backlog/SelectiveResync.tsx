'use client'

import { useState } from 'react'
import { mutate } from 'swr'
import { useCatalogBooks, useResyncBooks } from '@/hooks/useBacklog'
import { useProgressSocket } from '@/lib/backlog/progressSocket'
import { useResyncOpenLibrary } from '@/hooks/useBacklog'
import { Button } from '@/components/ui/button'
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

function matchesFilter(book: CatalogBookStatus, filter: FilterKey): boolean {
  switch (filter) {
    case 'missing_isbn':
      return !book.isbn13
    case 'not_in_ol':
      return book.openlibraryStatus === 'not_found'
    case 'not_in_gb':
      // Only flag when Open Library also did not find it — a book found in OL
      // already has its metadata sourced, so GB absence is not actionable.
      return book.googlebooksStatus === 'not_found' && book.openlibraryStatus !== 'found'
  }
}

function applyFilters(books: CatalogBookStatus[], active: Set<FilterKey>): CatalogBookStatus[] {
  if (active.size === 0) return books
  return books.filter((b) => [...active].some((f) => matchesFilter(b, f)))
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

  const [activeFilters, setActiveFilters] = useState<Set<FilterKey>>(new Set())
  const [selected, setSelected] = useState<Set<string>>(new Set())
  const [force, setForce] = useState(false)
  const [error, setError] = useState<string | null>(null)

  const { isRefreshing, processed, total } = useProgressSocket(
    'resync-openlibrary',
    triggerResync,
    () => {
      void mutate('/backlog/books/catalog')
      setSelected(new Set())
    }
  )

  const allBooks = data?.books ?? []
  const filtered = applyFilters(allBooks, activeFilters)

  function toggleFilter(key: FilterKey) {
    setActiveFilters((prev) => {
      const next = new Set(prev)
      if (next.has(key)) next.delete(key)
      else next.add(key)
      return next
    })
    // Clear selection when filters change — the visible set changes.
    setSelected(new Set())
  }

  function toggleBook(id: string) {
    setSelected((prev) => {
      const next = new Set(prev)
      if (next.has(id)) next.delete(id)
      else next.add(id)
      return next
    })
  }

  function toggleAll() {
    if (selected.size === filtered.length) {
      setSelected(new Set())
    } else {
      setSelected(new Set(filtered.map((b) => b.id)))
    }
  }

  async function handleResync() {
    setError(null)
    const ids = [...selected]
    if (ids.length === 0) return
    try {
      await resyncBooks(ids, force)
    } catch (e) {
      setError(e instanceof Error ? e.message : 'Resync failed.')
    }
  }

  const allSelected = filtered.length > 0 && selected.size === filtered.length

  return (
    <div>
      {/* Filter chips */}
      <div className="mb-3 flex flex-wrap gap-2">
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
            onClick={() => {
              setActiveFilters(new Set())
              setSelected(new Set())
            }}
            className="text-xs text-muted underline hover:text-fg"
          >
            Clear filters
          </button>
        )}
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
            <input
              type="checkbox"
              checked={allSelected}
              onChange={toggleAll}
              aria-label="Select all"
              className="h-4 w-4 cursor-pointer rounded"
            />
            <span className="text-xs text-muted">
              {selected.size > 0
                ? `${selected.size} of ${filtered.length} selected`
                : `${filtered.length} book${filtered.length !== 1 ? 's' : ''}`}
            </span>
          </div>

          {/* Rows */}
          <ul className="divide-y divide-border">
            {filtered.map((book) => (
              <li key={book.id} className="flex items-start gap-3 px-3 py-2 hover:bg-hover">
                <input
                  type="checkbox"
                  checked={selected.has(book.id)}
                  onChange={() => toggleBook(book.id)}
                  aria-label={`Select ${book.title}`}
                  className="mt-0.5 h-4 w-4 cursor-pointer rounded"
                />
                <div className="min-w-0 flex-1">
                  <p className="truncate text-sm font-medium text-fg">{book.title}</p>
                  {book.authors.length > 0 && (
                    <p className="truncate text-xs text-muted">{book.authors.join(', ')}</p>
                  )}
                  <div className="mt-1 flex flex-wrap gap-x-3 gap-y-0.5">
                    {!book.isbn13 && <span className="text-xs text-warn">No ISBN</span>}
                    {!book.hasCover && <span className="text-xs text-muted">No cover</span>}
                    {!book.hasDescription && (
                      <span className="text-xs text-muted">No description</span>
                    )}
                    {!book.hasPageCount && (
                      <span className="text-xs text-muted">No page count</span>
                    )}
                  </div>
                  {book.lastResyncAt && (
                    <div className="mt-1 flex flex-wrap gap-x-4">
                      <span className="text-xs text-muted">
                        OL: <ResyncStatusBadge status={book.openlibraryStatus} />
                      </span>
                      <span className="text-xs text-muted">
                        GB: <ResyncStatusBadge status={book.googlebooksStatus} />
                      </span>
                    </div>
                  )}
                </div>
              </li>
            ))}
          </ul>
        </div>
      )}

      {/* Actions */}
      {filtered.length > 0 && (
        <div className="mt-4 flex flex-wrap items-center gap-3">
          <Button
            type="button"
            variant="secondary"
            disabled={selected.size === 0 || isRefreshing}
            onClick={() => void handleResync()}
          >
            {isRefreshing
              ? `Resyncing… (${processed ?? 0} / ${total ?? '?'})`
              : `Resync ${selected.size > 0 ? selected.size : ''} selected`}
          </Button>

          <label className="flex cursor-pointer items-center gap-2 text-sm text-fg">
            <input
              type="checkbox"
              checked={force}
              onChange={(e) => setForce(e.target.checked)}
              className="h-4 w-4 rounded"
            />
            Force re-fetch
          </label>
        </div>
      )}

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
