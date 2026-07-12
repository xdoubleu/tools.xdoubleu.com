'use client'

import { useMemo, useState } from 'react'
import { useRouter, useSearchParams } from 'next/navigation'
import { mutate } from 'swr'
import ReactMarkdown from 'react-markdown'
import remarkGfm from 'remark-gfm'
import { useLibrary } from '@/hooks/useBooks'
import { useCurrentUser } from '@/hooks/useAuth'
import type { UserBook } from '@/lib/gen/books/v1/library_pb'
import BookCover from '@/components/books/BookCover'
import BookSourceSync from '@/components/books/BookSourceSync'
import { SPECIAL_TAGS } from '@/lib/books/bookShelves'
import BookProgressEditor from '@/components/books/BookProgressEditor'
import BookRatingStars from '@/components/books/BookRatingStars'
import BookReadDatesEditor from '@/components/books/BookReadDatesEditor'
import BookFavouriteButton from '@/components/books/BookFavouriteButton'
import BookOwnershipToggles from '@/components/books/BookOwnershipToggles'
import BookShelfTagFields from '@/components/books/BookShelfTagFields'
import KoboSyncToggle from '@/components/books/KoboSyncToggle'
import BookPreviewDialog from '@/components/books/BookPreviewDialog'
import RemoveBookDialog from '@/components/books/RemoveBookDialog'
import { Breadcrumb, type BreadcrumbItem } from '@/components/ui/breadcrumb'
import { Button } from '@/components/ui/button'
import { PageContainer } from '@/components/ui/page-container'
import { swrKeys } from '@/lib/swrKeys'
import { formatDate } from '@/lib/dates'

function flattenLibrary(
  library: NonNullable<ReturnType<typeof useLibrary>['data']>['library']
): UserBook[] {
  if (!library) return []
  const shelfBooks = library.shelves.flatMap((s) => s.books)
  return [...library.reading, ...library.wishlist, ...library.finished, ...shelfBooks]
}

export default function BookDetailClient({ id }: { id: string }) {
  const { data, error, isLoading } = useLibrary()
  const { data: currentUser } = useCurrentUser()
  const isAdmin = currentUser?.role === 'admin'
  const [previewFormat, setPreviewFormat] = useState<'pdf' | 'epub' | 'kepub' | null>(null)
  const [removeOpen, setRemoveOpen] = useState(false)
  const router = useRouter()
  const searchParams = useSearchParams()
  const query = searchParams.get('q')

  const userBook = useMemo(() => {
    if (!data?.library) return null
    return flattenLibrary(data.library).find((ub) => ub.id === id) ?? null
  }, [data, id])

  const book = userBook?.book

  const knownShelves = data?.library?.shelves.map((s) => s.name) ?? []

  const knownTags = useMemo(() => {
    if (!data?.library) return []
    const seen = new Set<string>()
    for (const ub of flattenLibrary(data.library)) {
      for (const t of ub.tags) {
        if (!SPECIAL_TAGS.has(t)) seen.add(t)
      }
    }
    return Array.from(seen).sort()
  }, [data])

  const breadcrumbItems: BreadcrumbItem[] = [
    { label: 'Books', href: '/books' },
    {
      label: 'Library',
      href: query ? `/books/library?q=${encodeURIComponent(query)}` : '/books/library'
    },
    { label: book?.title ?? 'Book' }
  ]

  const handleSaved = () => {
    void mutate(swrKeys.books)
  }

  return (
    <PageContainer className="p-6">
      <Breadcrumb items={breadcrumbItems} />

      {isLoading && <p className="mt-6 text-muted">Loading book…</p>}
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

              {/* Rating + favourite + page count */}
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
              </div>

              {book.isbn13 && <p className="mt-2 text-xs text-muted">ISBN: {book.isbn13}</p>}

              {/* Shelf, ownership + tags — inline editing, no popover */}
              <div className="mt-4 space-y-4">
                <BookShelfTagFields
                  userBook={userBook}
                  knownShelves={knownShelves}
                  knownTags={knownTags}
                  onSaved={handleSaved}
                />
                <BookOwnershipToggles userBook={userBook} onSaved={handleSaved} />
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

          {/* Admin: live metadata source sync */}
          {isAdmin && (
            <section className="mt-8">
              <h2 className="text-lg font-semibold mb-3">Metadata source</h2>
              <BookSourceSync bookId={userBook.bookId} />
            </section>
          )}

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

              {(userBook.status === 'read' || userBook.finishedAt.length > 0) && (
                <BookReadDatesEditor userBook={userBook} onSaved={handleSaved} />
              )}

              {/* Kobo sync — only shown when a syncable file exists (same
                  check the preview buttons below use), not the own-digital
                  tag, which can drift out of sync with the actual files */}
              {(userBook.formats.includes('epub') || userBook.formats.includes('pdf')) && (
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

              <div>
                <Button
                  type="button"
                  variant="destructive"
                  size="sm"
                  className="text-xs"
                  onClick={() => setRemoveOpen(true)}
                >
                  Remove from library
                </Button>
              </div>
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

      {userBook && (
        <RemoveBookDialog
          bookId={userBook.bookId}
          title={book?.title ?? 'this book'}
          open={removeOpen}
          onOpenChange={setRemoveOpen}
          onRemoved={() => router.push('/books/library')}
        />
      )}
    </PageContainer>
  )
}
