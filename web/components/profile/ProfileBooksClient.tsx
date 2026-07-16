'use client'

import { useMemo, useState } from 'react'
import { useSharedLibrary, useSharedBooksProgress } from '@/hooks/useProfile'
import type { GetSharedLibraryResponse } from '@/lib/gen/books/v1/public_pb'
import type { LibraryResponse, UserBook } from '@/lib/gen/books/v1/library_pb'
import LibrarySidebar, {
  buildShelves,
  buildTags,
  type ShelfId
} from '@/components/books/LibrarySidebar'
import BookCover from '@/components/books/BookCover'
import BookProgressBar from '@/components/books/BookProgressBar'
import BooksProgressChart from '@/components/books/BooksProgressChart'
import GamesStatCard from '@/components/games/GamesStatCard'
import { Button } from '@/components/ui/button'
import { Card } from '@/components/ui/card'
import { Input } from '@/components/ui/input'
import { DateInput } from '@/components/ui/date-input'
import { formatDateTime, oneYearAgo, today } from '@/lib/dates'
import { ytdProgress } from '@/lib/books/ytdProgress'
import { statusLabel, displayTags } from '@/lib/books/bookShelves'

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
  if (shelfId === 'to-read') return library.wishlist
  if (shelfId === 'read') return library.finished
  return library.shelves.find((s) => s.name === shelfId)?.books ?? []
}

// Read-only book card: no link (book detail pages are owner-only), no
// favourite toggle or status editing.
function ProfileBookCard({ userBook }: { userBook: UserBook }) {
  const book = userBook.book
  if (!book) return null
  const tags = displayTags(userBook.tags)
  return (
    <Card className="flex gap-3 p-4">
      <BookCover coverUrl={book.coverUrl} title={book.title} size="md" />
      <div className="min-w-0 flex-1">
        <h3 className="font-semibold truncate">
          {book.title}
          {userBook.tags.includes('favourite') && (
            <span className="ml-2 text-amber-500" aria-label="Favourite">
              ♥
            </span>
          )}
        </h3>
        <p className="text-sm text-muted truncate">{book.authors.join(', ')}</p>
        <p className="text-sm text-muted">{statusLabel(userBook.status)}</p>
        {userBook.rating > 0 && (
          <p className="text-sm text-amber-500" aria-label={`Rated ${userBook.rating} of 5`}>
            {'★'.repeat(userBook.rating)}
            <span className="text-border">{'★'.repeat(Math.max(0, 5 - userBook.rating))}</span>
          </p>
        )}
        {userBook.status === 'currently-reading' && (
          <div className="mt-2">
            <BookProgressBar userBook={userBook} />
          </div>
        )}
        {tags.length > 0 && <p className="text-xs text-muted truncate mt-1">{tags.join(', ')}</p>}
      </div>
    </Card>
  )
}

type Selection = { kind: 'shelf'; id: ShelfId } | { kind: 'tag'; tag: string }

export default function ProfileBooksClient({
  token,
  initialData
}: {
  token: string
  initialData?: GetSharedLibraryResponse
}) {
  const { data, error, isLoading } = useSharedLibrary(token, initialData)

  const [view, setView] = useState<'ytd' | 'all'>('ytd')
  const [progressStart, setProgressStart] = useState(oneYearAgo())
  const [progressEnd, setProgressEnd] = useState(today())
  const { data: progressData } = useSharedBooksProgress(
    view === 'all' ? token : '',
    progressStart,
    progressEnd
  )

  const [selection, setSelection] = useState<Selection>({ kind: 'shelf', id: 'all' })
  const [search, setSearch] = useState('')

  const library = data?.library
  const reading = library?.reading ?? []
  const ytd = ytdProgress(library?.finished ?? [])

  const allTimeChartData =
    progressData?.progress?.labels?.map((label: string, idx: number) => ({
      label,
      value: parseInt(progressData.progress?.values?.[idx] ?? '0', 10)
    })) ?? []

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
    <section className="flex flex-col gap-6">
      {data?.lastSyncedAt && (
        <p className="text-xs text-muted">Last synced: {formatDateTime(data.lastSyncedAt)}</p>
      )}

      <div className="grid grid-cols-2 gap-3 sm:grid-cols-3 lg:grid-cols-5">
        <GamesStatCard
          label="Total books"
          value={reading.length + library.wishlist.length + library.finished.length}
        />
        <GamesStatCard label={statusLabel('currently-reading')} value={reading.length} />
        <GamesStatCard label={statusLabel('read')} value={library.finished.length} />
        <GamesStatCard label="Read this year" value={ytd.total} />
        <GamesStatCard label={statusLabel('to-read')} value={library.wishlist.length} />
      </div>

      <div className="grid gap-6 lg:grid-cols-2">
        <div>
          <h2 className="mb-2 text-base font-semibold">Currently reading</h2>
          {reading.length === 0 && <p className="text-muted text-sm">No books in progress.</p>}
          {reading.length > 0 && (
            <div className="flex flex-wrap content-start gap-3">
              {reading.map((ub) => (
                <div key={ub.id} className="w-full sm:w-72">
                  <ProfileBookCard userBook={ub} />
                </div>
              ))}
            </div>
          )}
        </div>

        <div>
          <div className="mb-2 flex flex-wrap items-end justify-between gap-3">
            <div
              role="tablist"
              aria-label="Chart view"
              className="flex gap-1 rounded-xl border border-border bg-surface p-1"
            >
              <Button
                role="tab"
                aria-selected={view === 'ytd'}
                size="sm"
                variant={view === 'ytd' ? 'default' : 'ghost'}
                onClick={() => setView('ytd')}
              >
                This year
              </Button>
              <Button
                role="tab"
                aria-selected={view === 'all'}
                size="sm"
                variant={view === 'all' ? 'default' : 'ghost'}
                onClick={() => setView('all')}
              >
                All time
              </Button>
            </div>

            {view === 'all' && (
              <div className="flex gap-3">
                <div>
                  <label htmlFor="profile-books-from" className="mb-1 block text-xs text-muted">
                    From
                  </label>
                  <DateInput
                    id="profile-books-from"
                    value={progressStart}
                    onChange={setProgressStart}
                    className="h-9 w-40"
                  />
                </div>
                <div>
                  <label htmlFor="profile-books-to" className="mb-1 block text-xs text-muted">
                    To
                  </label>
                  <DateInput
                    id="profile-books-to"
                    value={progressEnd}
                    onChange={setProgressEnd}
                    className="h-9 w-40"
                  />
                </div>
              </div>
            )}
          </div>

          {view === 'ytd' && (
            <>
              {ytd.series.length === 0 && (
                <p className="text-muted text-sm">No books finished this year yet.</p>
              )}
              {ytd.series.length > 0 && <BooksProgressChart data={ytd.series} />}
            </>
          )}
          {view === 'all' && <BooksProgressChart data={allTimeChartData} />}
        </div>
      </div>

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
    </section>
  )
}
