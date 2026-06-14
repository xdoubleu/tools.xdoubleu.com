'use client'

import { useState, useCallback } from 'react'
import type { LibraryResponse, UserBook } from '@/lib/gen/backlog/v1/books_pb'
import BookCard from '@/components/backlog/BookCard'
import { Button } from '@/components/ui/button'
import { cn } from '@/lib/cn'

const PAGE_SIZE = 20

type FilterKey = 'physical' | 'digital' | 'pdf' | 'epub'

const FILTERS: { key: FilterKey; label: string }[] = [
  { key: 'physical', label: 'Physical' },
  { key: 'digital', label: 'Digital' },
  { key: 'pdf', label: 'PDF' },
  { key: 'epub', label: 'EPUB' }
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
  onEdit: (ub: UserBook) => void
}

export default function BooksLibrary({ library, onEdit }: BooksLibraryProps) {
  const shelves = buildShelves(library)
  const defaultShelf = shelves.find((s) => s.books.length > 0)?.id ?? shelves[0]?.id ?? ''

  const [selectedShelf, setSelectedShelf] = useState<ShelfId>(defaultShelf)
  const [activeFilters, setActiveFilters] = useState<Set<FilterKey>>(new Set())
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
  const pageCount = Math.max(1, Math.ceil(filtered.length / PAGE_SIZE))
  const safePage = Math.min(page, pageCount)
  const pageBooks = filtered.slice((safePage - 1) * PAGE_SIZE, safePage * PAGE_SIZE)

  return (
    <div className="flex flex-col md:flex-row gap-6">
      <ShelfSidebar shelves={shelves} selected={selectedShelf} onSelect={handleSelectShelf} />

      <div className="flex-1 min-w-0">
        {/* Filter chips */}
        <div className="flex items-center gap-2 flex-wrap mb-4">
          {FILTERS.map(({ key, label }) => (
            <button
              key={key}
              onClick={() => toggleFilter(key)}
              className={cn(
                'px-3 py-1 rounded-full text-sm border transition-colors',
                activeFilters.has(key)
                  ? 'bg-accent text-white border-accent'
                  : 'bg-surface text-subtle border-border hover:border-accent/50 hover:text-foreground'
              )}
              aria-pressed={activeFilters.has(key)}
            >
              {label}
            </button>
          ))}
          {activeFilters.size > 0 && (
            <button
              onClick={() => {
                setActiveFilters(new Set())
                setPage(1)
              }}
              className="px-3 py-1 rounded-full text-sm text-muted hover:text-foreground"
            >
              Clear
            </button>
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
          <div className="grid gap-3">
            {pageBooks.map((ub) => (
              <BookCard key={ub.id} userBook={ub} onEdit={onEdit} />
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
