'use client'

import { useMemo, useState } from 'react'
import { useSharedLibrary } from '@/hooks/useProfile'
import type { GetSharedLibraryResponse } from '@/lib/gen/reading/v1/public_pb'
import type { LibraryResponse, UserBook } from '@/lib/gen/reading/v1/library_pb'
import LibrarySidebar, {
  buildShelves,
  buildTags,
  type ShelfId
} from '@/components/reading/LibrarySidebar'
import ProfileBookCard from '@/components/profile/ProfileBookCard'
import { Input } from '@/components/ui/input'
import { flattenLibrary } from '@/lib/reading/bookShelves'

function booksForShelf(library: LibraryResponse, shelfId: ShelfId): UserBook[] {
  if (shelfId === 'all') return flattenLibrary(library)
  if (shelfId === 'favourite')
    return flattenLibrary(library).filter((b) => b.tags.includes('favourite'))
  if (shelfId === 'currently-reading') return library.reading
  if (shelfId === 'to-read') return library.wishlist
  if (shelfId === 'read') return library.finished
  return library.shelves.find((s) => s.name === shelfId)?.books ?? []
}

type Selection = { kind: 'shelf'; id: ShelfId } | { kind: 'tag'; tag: string }

// Read-only shared library: the same shelf/tag sidebar and book grid as the
// owner's /reading/library, but without any editing affordances.
export default function ProfileBooksLibrary({
  token,
  initialData
}: {
  token: string
  initialData?: GetSharedLibraryResponse
}) {
  const { data, error, isLoading } = useSharedLibrary(token, initialData)

  const [selection, setSelection] = useState<Selection>({ kind: 'shelf', id: 'all' })
  const [search, setSearch] = useState('')

  const library = data?.library

  const shelfBooks = useMemo(() => {
    if (!library) return []
    if (selection.kind === 'tag') {
      return flattenLibrary(library).filter((b) => b.tags.includes(selection.tag))
    }
    return booksForShelf(library, selection.id)
  }, [library, selection])

  const filteredBooks = useMemo(() => {
    const q = search.trim().toLowerCase()
    if (!q || !library) return shelfBooks
    return flattenLibrary(library).filter((ub) => {
      const book = ub.book
      if (!book) return false
      if (book.title.toLowerCase().includes(q)) return true
      return book.authors.some((a) => a.toLowerCase().includes(q))
    })
  }, [library, shelfBooks, search])

  if (isLoading && !library) return <p className="text-muted">Loading books…</p>
  if (error && !library) return <p className="text-danger">Failed to load books.</p>
  if (!library) return null

  const shelves = buildShelves(library)
  const allTags = buildTags(library)
  const currentShelf =
    selection.kind === 'shelf' ? shelves.find((s) => s.id === selection.id) : null
  const headerLabel = search.trim()
    ? 'Search results'
    : selection.kind === 'tag'
      ? selection.tag
      : (currentShelf?.label ?? '')

  return (
    <div>
      <Input
        type="search"
        value={search}
        onChange={(e) => setSearch(e.target.value)}
        placeholder="Search books…"
        className="mb-4 max-w-md"
      />
      <div className="flex flex-col md:flex-row gap-6">
        <LibrarySidebar
          shelves={shelves}
          allTags={allTags}
          selectedShelfId={selection.kind === 'shelf' ? selection.id : null}
          selectedTag={selection.kind === 'tag' ? selection.tag : null}
          onSelectShelf={(id) => setSelection({ kind: 'shelf', id })}
          onSelectTag={(tag) =>
            setSelection((prev) =>
              prev.kind === 'tag' && prev.tag === tag
                ? { kind: 'shelf', id: 'all' }
                : { kind: 'tag', tag }
            )
          }
        />
        <div className="flex-1 min-w-0">
          <h2 className="text-lg font-semibold mb-3">
            {headerLabel}
            <span className="ml-2 text-sm font-normal text-muted">{filteredBooks.length}</span>
          </h2>
          <div className="grid grid-cols-1 sm:grid-cols-2 gap-3">
            {filteredBooks.map((ub) => (
              <ProfileBookCard key={ub.id} userBook={ub} />
            ))}
            {filteredBooks.length === 0 && (
              <p className="col-span-full py-16 text-center text-sm text-muted">No books.</p>
            )}
          </div>
        </div>
      </div>
    </div>
  )
}
