'use client'

import { useState, useCallback, useMemo, useEffect } from 'react'
import type { LibraryResponse, UserBook } from '@/lib/gen/backlog/v1/books_pb'
import LibrarySidebar, {
  buildShelves,
  buildTags,
  type ShelfId
} from '@/components/backlog/LibrarySidebar'
import BooksTable from '@/components/backlog/BooksTable'
import ManageShelvesTagsDialog from '@/components/backlog/ManageShelvesTagsDialog'
import { SPECIAL_TAGS } from '@/lib/backlog/bookShelves'

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
  /**
   * Notifies the parent whether the current search query matches any library
   * entries.  Used by BookSearchBar to decide when to show the OL fallback.
   */
  onSearchResultsChange: (hasResults: boolean) => void
  onSaved: () => void
}

export default function BooksLibrary({
  library,
  knownShelves,
  searchQuery,
  onSearchResultsChange,
  onSaved
}: BooksLibraryProps) {
  const shelves = buildShelves(library)
  const allTags = buildTags(library)
  const defaultShelf: ShelfId =
    shelves.find((s) => s.id !== 'all' && s.id !== 'favourite' && s.count > 0)?.id ?? 'all'

  const [selection, setSelection] = useState<Selection>({ kind: 'shelf', id: defaultShelf })
  const [manageOpen, setManageOpen] = useState(false)

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

  // Notify the parent so BookSearchBar knows whether to show the OL fallback.
  useEffect(() => {
    const q = searchQuery.trim()
    onSearchResultsChange(q === '' || filteredBooks.length > 0)
  }, [searchQuery, filteredBooks.length, onSearchResultsChange])

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
  const headerLabel = searchQuery.trim()
    ? 'Search results'
    : selection.kind === 'tag'
      ? selection.tag
      : (currentShelf?.label ?? '')

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
            <span className="ml-2 text-sm font-normal text-muted">{filteredBooks.length}</span>
          </h2>

          <BooksTable
            books={filteredBooks}
            knownShelves={knownShelves}
            knownTags={knownTags}
            onSaved={onSaved}
          />
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
