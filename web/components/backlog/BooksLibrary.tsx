'use client'

import { useState, useCallback, useMemo } from 'react'
import type { LibraryResponse, UserBook } from '@/lib/gen/backlog/v1/books_pb'
import LibrarySidebar, {
  buildShelves,
  buildTags,
  type ShelfId
} from '@/components/backlog/LibrarySidebar'
import BooksTable from '@/components/backlog/BooksTable'
import ManageShelvesTagsDialog from '@/components/backlog/ManageShelvesTagsDialog'
import { SPECIAL_TAGS } from '@/lib/backlog/bookShelves'

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
  if (shelfId === 'currently-reading') return library.reading
  if (shelfId === 'wishlist') return library.wishlist
  if (shelfId === 'finished') return library.finished
  return library.shelves.find((s) => s.name === shelfId)?.books ?? []
}

function filterByTags(books: UserBook[], activeTags: Set<string>): UserBook[] {
  if (activeTags.size === 0) return books
  return books.filter((b) => [...activeTags].every((t) => b.tags.includes(t)))
}

interface BooksLibraryProps {
  library: LibraryResponse
  knownShelves: string[]
  onSaved: () => void
}

export default function BooksLibrary({ library, knownShelves, onSaved }: BooksLibraryProps) {
  const shelves = buildShelves(library)
  const allTags = buildTags(library)
  const defaultShelf: ShelfId = shelves.find((s) => s.id !== 'all' && s.count > 0)?.id ?? 'all'

  const [selectedShelf, setSelectedShelf] = useState<ShelfId>(defaultShelf)
  const [selectedTags, setSelectedTags] = useState<Set<string>>(new Set())
  const [manageOpen, setManageOpen] = useState(false)

  const handleSelectShelf = useCallback((id: ShelfId) => {
    setSelectedShelf(id)
  }, [])

  const handleToggleTag = useCallback((tag: string) => {
    setSelectedTags((prev) => {
      const next = new Set(prev)
      if (next.has(tag)) {
        next.delete(tag)
      } else {
        next.add(tag)
      }
      return next
    })
  }, [])

  const shelfBooks = useMemo(() => booksForShelf(library, selectedShelf), [library, selectedShelf])

  const filteredBooks = useMemo(
    () => filterByTags(shelfBooks, selectedTags),
    [shelfBooks, selectedTags]
  )

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

  const currentShelf = shelves.find((s) => s.id === selectedShelf)

  return (
    <>
      <div className="flex flex-col md:flex-row gap-6">
        <LibrarySidebar
          shelves={shelves}
          allTags={allTags}
          selectedShelf={selectedShelf}
          selectedTags={selectedTags}
          onSelectShelf={handleSelectShelf}
          onToggleTag={handleToggleTag}
          onManage={() => setManageOpen(true)}
        />

        <div className="flex-1 min-w-0">
          {/* Shelf header */}
          <h2 className="text-lg font-semibold mb-3">
            {currentShelf?.label ?? ''}
            <span className="ml-2 text-sm font-normal text-muted">
              {filteredBooks.length !== shelfBooks.length
                ? `${filteredBooks.length} of ${shelfBooks.length}`
                : shelfBooks.length}
            </span>
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
        tags={allTags}
      />
    </>
  )
}
