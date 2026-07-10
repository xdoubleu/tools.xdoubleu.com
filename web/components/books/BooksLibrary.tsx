'use client'

import { useState, useCallback, useMemo, useEffect, useRef } from 'react'
import type { LibraryResponse, UserBook, ExternalBookResult } from '@/lib/gen/books/v1/library_pb'
import LibrarySidebar, {
  buildShelves,
  buildTags,
  type ShelfId
} from '@/components/books/LibrarySidebar'
import BooksTable from '@/components/books/BooksTable'
import BookCard from '@/components/books/BookCard'
import ExternalBookCard from '@/components/books/ExternalBookCard'
import ManageShelvesTagsDialog from '@/components/books/ManageShelvesTagsDialog'
import { useSearchExternal } from '@/hooks/useBooks'
import { SPECIAL_TAGS } from '@/lib/books/bookShelves'

type Selection = { kind: 'shelf'; id: ShelfId } | { kind: 'tag'; tag: string }

function flattenLibrary(library: LibraryResponse): UserBook[] {
  return [
    ...library.reading,
    ...library.wishlist,
    ...library.finished,
    ...library.shelves.flatMap((s) => s.books)
  ]
}

function booksForShelf(library: LibraryResponse, shelfId: ShelfId): UserBook[] {
  if (shelfId === 'all') return flattenLibrary(library)
  if (shelfId === 'favourite')
    return flattenLibrary(library).filter((b) => b.tags.includes('favourite'))
  if (shelfId === 'currently-reading') return library.reading
  if (shelfId === 'wishlist') return library.wishlist
  if (shelfId === 'finished') return library.finished
  return library.shelves.find((s) => s.name === shelfId)?.books ?? []
}

interface BooksLibraryProps {
  library: LibraryResponse
  knownShelves: string[]
  /** Free-text query from the search bar. Empty string means no filter. */
  searchQuery: string
  onSaved: () => void
}

export default function BooksLibrary({
  library,
  knownShelves,
  searchQuery,
  onSaved
}: BooksLibraryProps) {
  const shelves = buildShelves(library)
  const allTags = buildTags(library)

  const [selection, setSelection] = useState<Selection>({ kind: 'shelf', id: 'all' })
  const [manageOpen, setManageOpen] = useState(false)
  const searchExternal = useSearchExternal()
  const [externalResults, setExternalResults] = useState<ExternalBookResult[]>([])
  const [isSearchingExternal, setIsSearchingExternal] = useState(false)
  const debounceTimer = useRef<ReturnType<typeof setTimeout> | null>(null)

  const handleSelectShelf = useCallback((id: ShelfId) => {
    setSelection({ kind: 'shelf', id })
  }, [])

  const handleSelectTag = useCallback((tag: string) => {
    setSelection((prev) => {
      if (prev.kind === 'tag' && prev.tag === tag) {
        return { kind: 'shelf', id: 'all' }
      }
      return { kind: 'tag', tag }
    })
  }, [])

  const shelfBooks = useMemo(() => {
    if (selection.kind === 'tag') {
      return flattenLibrary(library).filter((b) => b.tags.includes(selection.tag))
    }
    return booksForShelf(library, selection.id)
  }, [library, selection])

  // When a search query is active, filter across the whole library; otherwise
  // respect the shelf/tag selection.
  const filteredBooks = useMemo(() => {
    const q = searchQuery.trim().toLowerCase()
    if (!q) return shelfBooks

    return flattenLibrary(library).filter((ub) => {
      const book = ub.book
      if (!book) return false
      if (book.title.toLowerCase().includes(q)) return true
      return book.authors.some((a) => a.toLowerCase().includes(q))
    })
  }, [library, shelfBooks, searchQuery])

  // When the query has no library matches, fall back to an external (Open
  // Library) search so not-in-library results still show up as cards.
  // Debounced so typing doesn't fire a request per keystroke.
  useEffect(() => {
    const q = searchQuery.trim()
    if (!q || filteredBooks.length > 0) {
      setExternalResults([])
      setIsSearchingExternal(false)
      return
    }

    if (debounceTimer.current) clearTimeout(debounceTimer.current)
    debounceTimer.current = setTimeout(async () => {
      setIsSearchingExternal(true)
      try {
        const resp = await searchExternal(q)
        setExternalResults(resp.results)
      } catch {
        setExternalResults([])
      } finally {
        setIsSearchingExternal(false)
      }
    }, 300)

    return () => {
      if (debounceTimer.current) clearTimeout(debounceTimer.current)
    }
  }, [searchQuery, filteredBooks.length, searchExternal])

  // All known user-visible tags for the shelf/tag cell checkboxes
  const knownTags = useMemo(() => {
    const all = flattenLibrary(library)
    const seen = new Set<string>()
    for (const ub of all) {
      for (const t of ub.tags) {
        if (!SPECIAL_TAGS.has(t)) seen.add(t)
      }
    }
    return Array.from(seen).sort()
  }, [library])

  const currentShelf =
    selection.kind === 'shelf' ? shelves.find((s) => s.id === selection.id) : null
  const isSearching = searchQuery.trim() !== ''
  const headerLabel = isSearching
    ? 'Search results'
    : selection.kind === 'tag'
      ? selection.tag
      : (currentShelf?.label ?? '')
  const resultCount = filteredBooks.length + externalResults.length

  return (
    <>
      <div className="flex flex-col md:flex-row gap-6">
        <LibrarySidebar
          shelves={shelves}
          allTags={allTags}
          selectedShelfId={selection.kind === 'shelf' ? selection.id : null}
          selectedTag={selection.kind === 'tag' ? selection.tag : null}
          onSelectShelf={handleSelectShelf}
          onSelectTag={handleSelectTag}
          onManage={() => setManageOpen(true)}
        />

        <div className="flex-1 min-w-0">
          <h2 className="text-lg font-semibold mb-3">
            {headerLabel}
            <span className="ml-2 text-sm font-normal text-muted">{resultCount}</span>
          </h2>

          {isSearching ? (
            <div className="grid grid-cols-1 sm:grid-cols-2 gap-3">
              {filteredBooks.map((ub) => (
                <BookCard key={ub.id} userBook={ub} onSaved={onSaved} />
              ))}
              {externalResults.map((book) => (
                <ExternalBookCard key={`${book.provider}-${book.providerId}`} book={book} />
              ))}
              {!isSearchingExternal && resultCount === 0 && (
                <p className="col-span-full py-16 text-center text-sm text-muted">No results.</p>
              )}
              {isSearchingExternal && (
                <p className="col-span-full text-sm text-muted">Searching…</p>
              )}
            </div>
          ) : (
            <BooksTable
              books={filteredBooks}
              knownShelves={knownShelves}
              knownTags={knownTags}
              onSaved={onSaved}
            />
          )}
        </div>
      </div>

      <ManageShelvesTagsDialog
        open={manageOpen}
        onOpenChange={setManageOpen}
        shelves={shelves}
        tags={allTags.map((t) => t.name)}
      />
    </>
  )
}
