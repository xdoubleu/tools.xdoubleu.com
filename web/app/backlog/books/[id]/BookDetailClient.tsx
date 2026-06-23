'use client'

import { useMemo, useState } from 'react'
import { mutate } from 'swr'
import { useBacklogLibrary } from '@/hooks/useBacklog'
import type { UserBook } from '@/lib/gen/backlog/v1/books_pb'
import BookCover from '@/components/backlog/BookCover'
import BookProgressBar from '@/components/backlog/BookProgressBar'
import BookEditModal from '@/components/backlog/BookEditModal'
import { OwnershipBadges } from '@/components/backlog/BookCard'
import { Breadcrumb, type BreadcrumbItem } from '@/components/ui/breadcrumb'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'

function StarRating({ rating }: { rating: number }) {
  if (rating <= 0) return null
  const stars = Array.from({ length: 5 }, (_, i) => i + 1)
  return (
    <div className="flex items-center gap-0.5" aria-label={`${rating} out of 5 stars`}>
      {stars.map((star) => (
        <span key={star} className={star <= rating ? 'text-amber-400' : 'text-border'}>
          ★
        </span>
      ))}
    </div>
  )
}

function formatDate(iso: string): string {
  if (!iso) return ''
  return new Date(iso).toLocaleDateString(undefined, {
    year: 'numeric',
    month: 'short',
    day: 'numeric'
  })
}

function flattenLibrary(
  library: NonNullable<ReturnType<typeof useBacklogLibrary>['data']>['library']
): UserBook[] {
  if (!library) return []
  const shelfBooks = library.shelves.flatMap((s) => s.books)
  return [...library.reading, ...library.wishlist, ...library.finished, ...shelfBooks]
}

export default function BookDetailClient({ id }: { id: string }) {
  const { data, error, isLoading } = useBacklogLibrary()
  const [editingBook, setEditingBook] = useState<UserBook | null>(null)

  const userBook = useMemo(() => {
    if (!data?.library) return null
    return flattenLibrary(data.library).find((ub) => ub.id === id) ?? null
  }, [data, id])

  const book = userBook?.book

  const breadcrumbItems: BreadcrumbItem[] = [
    { label: 'Books', href: '/backlog/books' },
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

              <div className="mt-3 flex flex-wrap items-center gap-3">
                <StarRating rating={userBook.rating} />
                {book.pageCount > 0 && (
                  <span className="text-sm text-muted">{book.pageCount} pages</span>
                )}
                <span className="text-xs px-2 py-0.5 rounded-full bg-surface text-subtle capitalize">
                  {userBook.status.replace(/-/g, ' ')}
                </span>
                {userBook.tags.includes('favourite') && (
                  <Badge variant="secondary">Favourite</Badge>
                )}
              </div>

              <div className="mt-2">
                <OwnershipBadges userBook={userBook} />
              </div>

              {book.isbn13 && <p className="mt-2 text-xs text-muted">ISBN: {book.isbn13}</p>}

              <div className="mt-4">
                <Button variant="secondary" size="sm" onClick={() => setEditingBook(userBook)}>
                  Edit
                </Button>
              </div>
            </div>
          </div>

          {/* Description */}
          <section className="mt-8">
            <h2 className="text-lg font-semibold mb-2">Description</h2>
            {book.description ? (
              <p className="text-sm leading-relaxed text-foreground whitespace-pre-line">
                {book.description}
              </p>
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
                  <BookProgressBar userBook={userBook} />
                </div>
              )}

              {userBook.notes && (
                <div>
                  <p className="text-xs text-muted mb-1">Notes</p>
                  <p className="text-sm whitespace-pre-line">{userBook.notes}</p>
                </div>
              )}

              {userBook.tags.filter(
                (t) => t !== 'favourite' && t !== 'own-physical' && t !== 'own-digital'
              ).length > 0 && (
                <div>
                  <p className="text-xs text-muted mb-1">Tags</p>
                  <div className="flex flex-wrap gap-1">
                    {userBook.tags
                      .filter(
                        (t) => t !== 'favourite' && t !== 'own-physical' && t !== 'own-digital'
                      )
                      .map((tag) => (
                        <Badge key={tag} variant="secondary">
                          {tag}
                        </Badge>
                      ))}
                  </div>
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

              {userBook.addedAt && (
                <p className="text-xs text-muted">Added {formatDate(userBook.addedAt)}</p>
              )}
            </div>
          </section>
        </>
      )}

      {editingBook && (
        <BookEditModal
          userBook={editingBook}
          onClose={() => setEditingBook(null)}
          onSaved={handleSaved}
        />
      )}
    </main>
  )
}
