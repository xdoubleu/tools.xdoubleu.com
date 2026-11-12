import { useState, useCallback, useMemo, useEffect, useRef } from 'react'
import type { LibraryResponse, UserBook, ExternalBookResult } from '@/lib/gen/books/v1/library_pb'
import LibrarySidebar, {
  buildShelves,
  buildTags,
  type ShelfId
} from '@/components/books/LibrarySidebar'
import BooksTable from '@/components/books/BooksTable'
import { LibraryTableControls } from '@/components/books/BooksTableToolbar'
import BookSearchBar from '@/components/books/BookSearchBar'
import { applyFilters, useLibraryTableState } from '@/components/books/useLibraryTableState'
import BookCard from '@/components/books/BookCard'
import ExternalBookCard from '@/components/books/ExternalBookCard'
import ManageShelvesTagsDialog from '@/components/books/ManageShelvesTagsDialog'
import { useSearchExternal } from '@/hooks/useBooks'
import { SPECIAL_TAGS, flattenLibrary } from '@/lib/books/bookShelves'
import { buildSeriesSummaries } from '@/lib/books/series'
import { CollectionGrid } from '@/components/ui/collection-grid'
import { SegmentedTabs } from '@/components/ui/segmented-tabs'
import { EmptyState, LoadingState } from '@/components/ui/states'
import { useLocalStorage } from '@/hooks/useLocalStorage'
import { SectionHeading } from '@/components/ui/section-heading'
import { Toolbar } from '@/components/ui/toolbar'

type LibraryView = 'cards' | 'table'

type Selection = { kind: 'shelf'; id: ShelfId } | { kind: 'tag'; tag: string }

function booksForShelf(library: LibraryResponse, shelfId: ShelfId): UserBook[] {
  if (shelfId === 'all') return flattenLibrary(library)
  if (shelfId === 'favourite')
    return flattenLibrary(library).filter((b) => b.tags.includes('favourite'))
  if (shelfId === 'currently-reading') return library.reading
  if (shelfId === 'to-read') return library.wishlist
  if (shelfId === 'read') return library.finished
  return library.shelves.find((s) => s.name === shelfId)?.books ?? []
}

interface BooksLibraryProps {
  library: LibraryResponse
  knownShelves: string[]
  searchQuery: string
  onSearchChange: (query: string) => void
  onSaved: () => void
}

export default function BooksLibrary({
  library,
  knownShelves,
  searchQuery,
  onSearchChange,
  onSaved
}: BooksLibraryProps) {
  const shelves = buildShelves(library)
  const allTags = buildTags(library)
  const seriesSummaries = buildSeriesSummaries(library)

  const [selection, setSelection] = useState<Selection>({ kind: 'shelf', id: 'all' })
  const [manageOpen, setManageOpen] = useState(false)
  const table = useLibraryTableState()
  // The view is a per-device choice; cards are the default.
  const [view, setView] = useLocalStorage<LibraryView>('books:library:view', 'cards')
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

  // A search spans the whole library; otherwise shelf/tag filters apply.
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

  // No library matches: fall back to a debounced external search.
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
  const tableView = view === 'table'
  const resultCount = isSearching
    ? filteredBooks.length + externalResults.length
    : tableView
      ? applyFilters(filteredBooks, table.filters).length
      : filteredBooks.length

  return (
    <>
      <Toolbar
        aria-label="Filter books"
        search={<BookSearchBar query={searchQuery} onChange={onSearchChange} />}
        trailing={`${resultCount} ${resultCount === 1 ? 'book' : 'books'}`}
      >
        {!isSearching && (
          <SegmentedTabs
            aria-label="Library view"
            value={view}
            onChange={setView}
            options={[
              { value: 'cards', label: 'Cards' },
              { value: 'table', label: 'Table' }
            ]}
          />
        )}
        {!isSearching && tableView && <LibraryTableControls state={table} />}
      </Toolbar>

      <div className="flex flex-col md:flex-row gap-6">
        <LibrarySidebar
          shelves={shelves}
          allTags={allTags}
          selectedShelfId={selection.kind === 'shelf' ? selection.id : null}
          selectedTag={selection.kind === 'tag' ? selection.tag : null}
          onSelectShelf={handleSelectShelf}
          onSelectTag={handleSelectTag}
          onManage={() => setManageOpen(true)}
          series={seriesSummaries}
        />

        <div className="flex-1 min-w-0">
          <SectionHeading className="mb-3 truncate">{headerLabel}</SectionHeading>

          {isSearching ? (
            <CollectionGrid>
              {filteredBooks.map((ub) => (
                <BookCard key={ub.id} userBook={ub} onSaved={onSaved} query={searchQuery} />
              ))}
              {externalResults.map((book) => (
                <ExternalBookCard key={`${book.provider}-${book.providerId}`} book={book} />
              ))}
              {!isSearchingExternal && resultCount === 0 && (
                <EmptyState className="col-span-full">No results.</EmptyState>
              )}
              {isSearchingExternal && <LoadingState label="results" className="col-span-full" />}
            </CollectionGrid>
          ) : !tableView ? (
            filteredBooks.length === 0 ? (
              <EmptyState placement="inline">Nothing on this shelf yet.</EmptyState>
            ) : (
              <CollectionGrid>
                {filteredBooks.map((ub) => (
                  <BookCard key={ub.id} userBook={ub} onSaved={onSaved} />
                ))}
              </CollectionGrid>
            )
          ) : (
            <BooksTable
              books={filteredBooks}
              knownShelves={knownShelves}
              knownTags={knownTags}
              onSaved={onSaved}
              state={table}
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
