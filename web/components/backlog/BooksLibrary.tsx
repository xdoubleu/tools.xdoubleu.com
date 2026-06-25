'use client'

import { useState, useCallback, useMemo } from 'react'
import type { LibraryResponse, UserBook } from '@/lib/gen/backlog/v1/books_pb'
import BookCard from '@/components/backlog/BookCard'
import { Button } from '@/components/ui/button'
import { Select } from '@/components/ui/select'
import { cn } from '@/lib/cn'
import { displayProgressPercent } from '@/lib/backlog/bookProgress'

const PAGE_SIZE = 20

type FilterKey = 'physical' | 'digital' | 'pdf' | 'epub'

type SortKey = 'added' | 'title' | 'author' | 'rating' | 'progress'

const OWNERSHIP_FILTERS: { key: FilterKey; label: string }[] = [
  { key: 'physical', label: 'Physical' },
  { key: 'digital', label: 'Digital' }
]

const FORMAT_FILTERS: { key: FilterKey; label: string }[] = [
  { key: 'pdf', label: 'PDF' },
  { key: 'epub', label: 'EPUB' }
]

const SORT_OPTIONS: { value: SortKey; label: string }[] = [
  { value: 'added', label: 'Date added' },
  { value: 'title', label: 'Title' },
  { value: 'author', label: 'Author' },
  { value: 'rating', label: 'Rating' },
  { value: 'progress', label: 'Progress' }
]

type ShelfId = 'currently-reading' | 'wishlist' | 'finished' | (string & Record<never, never>)

interface Shelf {
  id: ShelfId
  label: string
  books: UserBook[]
}

function buildShelves(library: LibraryResponse): Shelf[] {
  const fixed: Shelf[] = [
    { id: 'currently-reading', label: 'Currently Reading', books: library.reading },
    { id: 'wishlist', label: 'Wishlist', books: library.wishlist },
    { id: 'finished', label: 'Finished', books: library.finished }
  ]
  const dynamic: Shelf[] = library.shelves.map((s) => ({
    id: s.name,
    label: s.name,
    books: s.books
  }))
  return [...fixed, ...dynamic]
}

function passesFilter(book: UserBook, activeFilters: Set<FilterKey>): boolean {
  for (const f of activeFilters) {
    if (f === 'physical' && !book.tags.includes('own-physical')) return false
    if (f === 'digital' && !book.tags.includes('own-digital')) return false
    if (f === 'pdf' && !book.formats.includes('pdf')) return false
    if (f === 'epub' && !book.formats.includes('epub')) return false
  }
  return true
}

function sortBooks(books: UserBook[], sortKey: SortKey): UserBook[] {
  const sorted = [...books]
  switch (sortKey) {
    case 'title':
      return sorted.sort((a, b) => (a.book?.title ?? '').localeCompare(b.book?.title ?? ''))
    case 'author':
      return sorted.sort((a, b) =>
        (a.book?.authors[0] ?? '').localeCompare(b.book?.authors[0] ?? '')
      )
    case 'rating':
      return sorted.sort((a, b) => b.rating - a.rating)
    case 'progress':
      return sorted.sort((a, b) => displayProgressPercent(b) - displayProgressPercent(a))
    default:
      // 'added' — preserve backend order
      return sorted
  }
}

interface ShelfSidebarProps {
  shelves: Shelf[]
  selected: ShelfId
  onSelect: (id: ShelfId) => void
}

function ShelfSidebar({ shelves, selected, onSelect }: ShelfSidebarProps) {
  return (
    <>
      {/* Desktop: vertical sidebar */}
      <nav className="hidden md:flex flex-col gap-1 min-w-44 shrink-0" aria-label="Shelves">
        {shelves.map((shelf) => (
          <button
            key={shelf.id}
            onClick={() => onSelect(shelf.id)}
            className={cn(
              'flex items-center justify-between w-full text-left px-3 py-2 rounded-xl text-sm transition-colors',
              selected === shelf.id
                ? 'bg-accent/10 text-accent font-medium'
                : 'text-subtle hover:bg-surface hover:text-foreground'
            )}
          >
            <span className="truncate">{shelf.label}</span>
            <span className="ml-2 text-xs text-muted shrink-0">{shelf.books.length}</span>
          </button>
        ))}
      </nav>

      {/* Mobile: horizontal scrollable chip row */}
      <div
        className="flex md:hidden gap-2 overflow-x-auto pb-2 -mx-1 px-1"
        role="tablist"
        aria-label="Shelves"
      >
        {shelves.map((shelf) => (
          <button
            key={shelf.id}
            role="tab"
            aria-selected={selected === shelf.id}
            onClick={() => onSelect(shelf.id)}
            className={cn(
              'flex items-center gap-1 shrink-0 px-3 py-1.5 rounded-full text-sm whitespace-nowrap transition-colors',
              selected === shelf.id
                ? 'bg-accent/10 text-accent font-medium'
                : 'bg-surface text-subtle hover:text-foreground'
            )}
          >
            {shelf.label}
            <span className="text-xs opacity-60">{shelf.books.length}</span>
          </button>
        ))}
      </div>
    </>
  )
}

interface BooksLibraryProps {
  library: LibraryResponse
  knownShelves: string[]
  onSaved: () => void
}

export default function BooksLibrary({ library, knownShelves, onSaved }: BooksLibraryProps) {
  const shelves = buildShelves(library)
  const defaultShelf = shelves.find((s) => s.books.length > 0)?.id ?? shelves[0]?.id ?? ''

  const [selectedShelf, setSelectedShelf] = useState<ShelfId>(defaultShelf)
  const [activeFilters, setActiveFilters] = useState<Set<FilterKey>>(new Set())
  const [sortKey, setSortKey] = useState<SortKey>('added')
  const [page, setPage] = useState(1)

  const handleSelectShelf = useCallback((id: ShelfId) => {
    setSelectedShelf(id)
    setPage(1)
  }, [])

  const toggleFilter = useCallback((key: FilterKey) => {
    setActiveFilters((prev) => {
      const next = new Set(prev)
      if (next.has(key)) {
        next.delete(key)
      } else {
        next.add(key)
      }
      return next
    })
    setPage(1)
  }, [])

  const currentShelf = shelves.find((s) => s.id === selectedShelf)
  const allBooks = currentShelf?.books ?? []
  const filtered = allBooks.filter((b) => passesFilter(b, activeFilters))
  const sorted = useMemo(() => sortBooks(filtered, sortKey), [filtered, sortKey])
  const pageCount = Math.max(1, Math.ceil(sorted.length / PAGE_SIZE))
  const safePage = Math.min(page, pageCount)
  const pageBooks = sorted.slice((safePage - 1) * PAGE_SIZE, safePage * PAGE_SIZE)

  const clearFilters = () => {
    setActiveFilters(new Set())
    setPage(1)
  }

  return (
    <div className="flex flex-col md:flex-row gap-6">
      <ShelfSidebar shelves={shelves} selected={selectedShelf} onSelect={handleSelectShelf} />

      <div className="flex-1 min-w-0">
        {/* Controls row: sort + filters */}
        <div className="flex flex-wrap items-center gap-2 mb-4">
          <Select
            value={sortKey}
            onChange={(e) => {
              const val = e.target.value
              const next = SORT_OPTIONS.find((o) => o.value === val)?.value ?? 'added'
              setSortKey(next)
              setPage(1)
            }}
            className="w-36 h-8 text-sm"
            aria-label="Sort books"
          >
            {SORT_OPTIONS.map(({ value, label }) => (
              <option key={value} value={value}>
                {label}
              </option>
            ))}
          </Select>

          <div className="h-4 w-px bg-border hidden sm:block" aria-hidden="true" />

          <span className="text-xs text-muted hidden sm:inline">Ownership:</span>
          {OWNERSHIP_FILTERS.map(({ key, label }) => (
            <Button
              key={key}
              variant="secondary"
              size="sm"
              onClick={() => toggleFilter(key)}
              aria-pressed={activeFilters.has(key)}
              className={cn(
                'text-xs rounded-full',
                activeFilters.has(key) && 'bg-accent text-white border-accent hover:bg-accent/90'
              )}
            >
              {label}
            </Button>
          ))}

          <div className="h-4 w-px bg-border hidden sm:block" aria-hidden="true" />

          <span className="text-xs text-muted hidden sm:inline">Format:</span>
          {FORMAT_FILTERS.map(({ key, label }) => (
            <Button
              key={key}
              variant="secondary"
              size="sm"
              onClick={() => toggleFilter(key)}
              aria-pressed={activeFilters.has(key)}
              className={cn(
                'text-xs rounded-full',
                activeFilters.has(key) && 'bg-accent text-white border-accent hover:bg-accent/90'
              )}
            >
              {label}
            </Button>
          ))}

          {activeFilters.size > 0 && (
            <Button variant="ghost" size="sm" onClick={clearFilters} className="text-xs text-muted">
              Clear
            </Button>
          )}
        </div>

        {/* Shelf header */}
        <h2 className="text-lg font-semibold mb-3">
          {currentShelf?.label ?? ''}
          <span className="ml-2 text-sm font-normal text-muted">
            {filtered.length !== allBooks.length
              ? `${filtered.length} of ${allBooks.length}`
              : allBooks.length}
          </span>
        </h2>

        {/* Book list */}
        {pageBooks.length === 0 ? (
          <p className="text-muted text-sm">No books match the current filters.</p>
        ) : (
          <div className="grid gap-2">
            {pageBooks.map((ub) => (
              <BookCard key={ub.id} userBook={ub} knownShelves={knownShelves} onSaved={onSaved} />
            ))}
          </div>
        )}

        {/* Pagination */}
        {pageCount > 1 && (
          <div className="flex items-center justify-center gap-3 mt-6">
            <Button
              variant="secondary"
              size="sm"
              onClick={() => setPage((p) => Math.max(1, p - 1))}
              disabled={safePage <= 1}
            >
              Prev
            </Button>
            <span className="text-sm text-muted">
              {safePage} / {pageCount}
            </span>
            <Button
              variant="secondary"
              size="sm"
              onClick={() => setPage((p) => Math.min(pageCount, p + 1))}
              disabled={safePage >= pageCount}
            >
              Next
            </Button>
          </div>
        )}
      </div>
    </div>
  )
}
