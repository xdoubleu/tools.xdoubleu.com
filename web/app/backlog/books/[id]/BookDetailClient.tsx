'use client'

import { useMemo, useState } from 'react'
import { mutate } from 'swr'
import ReactMarkdown from 'react-markdown'
import remarkGfm from 'remark-gfm'
import { useBacklogLibrary } from '@/hooks/useBacklog'
import type { UserBook } from '@/lib/gen/backlog/v1/books_pb'
import BookCover from '@/components/backlog/BookCover'
import BookProgressEditor from '@/components/backlog/BookProgressEditor'
import BookRatingStars from '@/components/backlog/BookRatingStars'
import BookFavouriteButton from '@/components/backlog/BookFavouriteButton'
import BookOwnershipToggles from '@/components/backlog/BookOwnershipToggles'
import BookShelfPopover from '@/components/backlog/BookShelfPopover'
import KoboSyncToggle from '@/components/backlog/KoboSyncToggle'
import BookPreviewDialog from '@/components/backlog/BookPreviewDialog'
import { Breadcrumb, type BreadcrumbItem } from '@/components/ui/breadcrumb'
import { Button } from '@/components/ui/button'

function flattenLibrary(
  library: NonNullable<ReturnType<typeof useBacklogLibrary>['data']>['library']
): UserBook[] {
  if (!library) return []
  const shelfBooks = library.shelves.flatMap((s) => s.books)
  return [...library.reading, ...library.wishlist, ...library.finished, ...shelfBooks]
}

function formatDate(iso: string): string {
  if (!iso) return ''
  return new Date(iso).toLocaleDateString(undefined, {
    year: 'numeric',
    month: 'short',
    day: 'numeric'
  })
}

export default function BookDetailClient({ id }: { id: string }) {
  const { data, error, isLoading } = useBacklogLibrary()
  const [previewFormat, setPreviewFormat] = useState<'pdf' | 'epub' | 'kepub' | null>(null)

  const userBook = useMemo(() => {
    if (!data?.library) return null
    return flattenLibrary(data.library).find((ub) => ub.id === id) ?? null
  }, [data, id])

  const book = userBook?.book

  const knownShelves = data?.library?.shelves.map((s) => s.name) ?? []

  const breadcrumbItems: BreadcrumbItem[] = [
    { label: 'Books', href: '/backlog/books' },
    { label: 'Library', href: '/backlog/books/library' },
    { label: book?.title ?? 'Book' }
  ]

  const handleSaved = () => {
    void mutate('/backlog/books')
  }

  return (
    <main className="max-w-4xl mx-auto p-6">
      <Breadcrumb items={breadcrumbItems} />

      {isLoading && <p className="mt-6 text-muted">Loading book...</p>}
      {error && <p className="mt-6 text-danger">Failed to load book.</p>}
      {!isLoading && !error && !userBook && <p className="mt-6 text-muted">Book not found.</p>}

      {book && userBook && (
        <>
          {/* Header */}
          <div className="mt-6 flex flex-col gap-6 sm:flex-row sm:items-start">
            <div className="shrink-0">
              <BookCover coverUrl={book.coverUrl} title={book.title} size="lg" />
            </div>

            <div className="flex-1 min-w-0">
              <h1 className="text-3xl font-bold leading-tight">{book.title}</h1>
              {book.authors.length > 0 && (
                <p className="mt-1 text-lg text-muted">{book.authors.join(', ')}</p>
              )}

              {/* Rating + favourite + page count + status */}
              <div className="mt-3 flex flex-wrap items-center gap-3">
                {userBook.status === 'read' && (
                  <>
                    <BookRatingStars userBook={userBook} size="md" onSaved={handleSaved} />
                    <BookFavouriteButton userBook={userBook} onSaved={handleSaved} />
                  </>
                )}
                {book.pageCount > 0 && (
                  <span className="text-sm text-muted">{book.pageCount} pages</span>
                )}
                <span className="text-xs px-2 py-0.5 rounded-full bg-surface text-subtle capitalize">
                  {userBook.status.replace(/-/g, ' ')}
                </span>
              </div>

              {/* Ownership toggles */}
              <div className="mt-2">
                <BookOwnershipToggles userBook={userBook} onSaved={handleSaved} />
              </div>

              {book.isbn13 && <p className="mt-2 text-xs text-muted">ISBN: {book.isbn13}</p>}

              {/* Status + shelves popover */}
              <div className="mt-4">
                <BookShelfPopover
                  userBook={userBook}
                  knownShelves={knownShelves}
                  onSaved={handleSaved}
                />
              </div>
            </div>
          </div>

          {/* Description */}
          <section className="mt-8">
            <h2 className="text-lg font-semibold mb-2">Description</h2>
            {book.description ? (
              <div className="prose prose-sm max-w-none text-foreground">
                <ReactMarkdown remarkPlugins={[remarkGfm]}>{book.description}</ReactMarkdown>
              </div>
            ) : (
              <p className="text-sm text-muted">No description available.</p>
            )}
          </section>

          {/* Reading info */}
          <section className="mt-8">
            <h2 className="text-lg font-semibold mb-3">Your reading</h2>
            <div className="rounded-2xl border border-border bg-card shadow-card p-4 flex flex-col gap-4">
              {userBook.status === 'currently-reading' && (
                <div>
                  <p className="text-xs text-muted mb-1">Progress</p>
                  <BookProgressEditor userBook={userBook} onSaved={handleSaved} />
                </div>
              )}

              {userBook.finishedAt.length > 0 && (
                <div>
                  <p className="text-xs text-muted mb-1">
                    {userBook.finishedAt.length === 1 ? 'Finished' : 'Read dates'}
                  </p>
                  <div className="flex flex-col gap-0.5">
                    {userBook.finishedAt.map((date) => (
                      <span key={date} className="text-sm">
                        {formatDate(date)}
                      </span>
                    ))}
                  </div>
                </div>
              )}

              {/* Kobo sync — only shown when the book is owned digitally */}
              {userBook.tags.includes('own-digital') && (
                <div>
                  <p className="text-xs text-muted mb-1">Kobo sync</p>
                  <KoboSyncToggle
                    bookId={userBook.bookId}
                    enabled={userBook.tags.includes('kobo-sync')}
                    tags={userBook.tags}
                    onChanged={handleSaved}
                  />
                </div>
              )}

              {/* File preview buttons */}
              {(userBook.formats.includes('pdf') || userBook.formats.includes('epub')) && (
                <div>
                  <p className="text-xs text-muted mb-1">Preview</p>
                  <div className="flex gap-2 flex-wrap">
                    {userBook.formats.includes('pdf') && (
                      <Button
                        type="button"
                        variant="secondary"
                        size="sm"
                        className="text-xs"
                        onClick={() => setPreviewFormat('pdf')}
                      >
                        Preview PDF
                      </Button>
                    )}
                    {userBook.formats.includes('epub') ? (
                      <Button
                        type="button"
                        variant="secondary"
                        size="sm"
                        className="text-xs"
                        onClick={() => setPreviewFormat('epub')}
                      >
                        Preview EPUB
                      </Button>
                    ) : (
                      userBook.formats.includes('pdf') && (
                        <Button
                          type="button"
                          variant="secondary"
                          size="sm"
                          className="text-xs"
                          onClick={() => setPreviewFormat('kepub')}
                        >
                          Preview EPUB
                        </Button>
                      )
                    )}
                  </div>
                </div>
              )}

              {userBook.addedAt && (
                <p className="text-xs text-muted">Added {formatDate(userBook.addedAt)}</p>
              )}
            </div>
          </section>
        </>
      )}

      {previewFormat && userBook && (
        <BookPreviewDialog
          bookId={userBook.bookId}
          format={previewFormat}
          title={book?.title ?? 'Book Preview'}
          open={!!previewFormat}
          onOpenChange={(open) => !open && setPreviewFormat(null)}
        />
      )}
    </main>
  )
}
