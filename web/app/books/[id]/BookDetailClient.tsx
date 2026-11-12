import { useMemo, useState } from 'react'
import { Link } from '@/components/ui/link'
import { useRouter, useSearchParams } from '@/hooks/navigation'
import { mutate } from 'swr'
import { useLibrary } from '@/hooks/useBooks'
import { useCurrentUser } from '@/hooks/useAuth'
import { Artwork } from '@/components/ui/artwork'
import BookDescription from '@/components/books/BookDescription'
import BookSourceSync from '@/components/books/BookSourceSync'
import { SPECIAL_TAGS, flattenLibrary } from '@/lib/books/bookShelves'
import { readerFormats } from '@/lib/books/readerSettings'
import { readSuggestionText } from '@/lib/books/readSuggestion'
import BookProgressEditor from '@/components/books/BookProgressEditor'
import BookRatingStars from '@/components/books/BookRatingStars'
import BookReadDatesEditor from '@/components/books/BookReadDatesEditor'
import BookFavouriteButton from '@/components/books/BookFavouriteButton'
import BookOwnershipToggles from '@/components/books/BookOwnershipToggles'
import BookShelfTagFields from '@/components/books/BookShelfTagFields'
import KoboSyncToggle from '@/components/books/KoboSyncToggle'
import OfflineBookBadge from '@/components/books/OfflineBookBadge'
import ArticleReaderDialog from '@/components/books/ArticleReaderDialog'
import RemoveBookDialog from '@/components/books/RemoveBookDialog'
import BookEditDialog from '@/components/books/BookEditDialog'
import { Breadcrumb, type BreadcrumbItem } from '@/components/ui/breadcrumb'
import { Button } from '@/components/ui/button'
import { Card } from '@/components/ui/card'
import { PageContainer } from '@/components/ui/page-container'
import { DetailHeader } from '@/components/ui/detail-header'
import { SectionHeading } from '@/components/ui/section-heading'
import { seriesHref, seriesLabel } from '@/lib/books/series'
import { EmptyState, ErrorState, LoadingState } from '@/components/ui/states'
import { swrKeys } from '@/lib/swrKeys'
import { formatDate } from '@/lib/dates'

export default function BookDetailClient({ id }: { id: string }) {
  const { data, error, isLoading } = useLibrary()
  const { data: currentUser } = useCurrentUser()
  const isAdmin = currentUser?.role === 'admin'
  const [removeOpen, setRemoveOpen] = useState(false)
  const [editOpen, setEditOpen] = useState(false)
  const [readerOpen, setReaderOpen] = useState(false)
  const router = useRouter()
  const searchParams = useSearchParams()
  const query = searchParams.get('q')

  const userBook = useMemo(() => {
    if (!data?.library) return null
    return flattenLibrary(data.library).find((ub) => ub.id === id) ?? null
  }, [data, id])

  const book = userBook?.book
  const readerFormatList = readerFormats(userBook?.formats ?? [])
  const suggestion = readSuggestionText(userBook?.readSuggestion, userBook?.formats ?? [])

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
    { label: 'Books', href: '/dashboard/reading' },
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
    <PageContainer>
      {!book && <Breadcrumb items={breadcrumbItems} />}

      {isLoading && <LoadingState label="book" placement="page" />}
      {error && <ErrorState what="book" placement="page" />}
      {!isLoading && !error && !userBook && <EmptyState>Book not found.</EmptyState>}

      {book && userBook && (
        <>
          <DetailHeader
            breadcrumb={breadcrumbItems}
            title={book.title}
            subtitle={book.authors.join(', ') || undefined}
            artwork={
              <Artwork
                src={book.coverUrl}
                title={book.title}
                alt={book.title}
                initials={2}
                size="lg"
              />
            }
            meta={book.pageCount > 0 ? `${book.pageCount} pages` : undefined}
            favourite={
              userBook.status === 'read' && (
                <BookFavouriteButton userBook={userBook} onSaved={handleSaved} />
              )
            }
            actions={
              isAdmin && (
                <Button type="button" variant="secondary" onClick={() => setEditOpen(true)}>
                  Edit book
                </Button>
              )
            }
            menu={[
              {
                label: 'Remove from library',
                destructive: true,
                onSelect: () => setRemoveOpen(true)
              }
            ]}
          />
          <div className="space-y-3">
            {book.seriesName && (
              <p className="mt-1 text-sm">
                <Button asChild variant="link" className="max-w-full">
                  <Link href={seriesHref(book.seriesName)}>
                    <span className="truncate">{seriesLabel(book)}</span>
                  </Link>
                </Button>
              </p>
            )}

            {userBook.status === 'read' && (
              <BookRatingStars userBook={userBook} size="md" onSaved={handleSaved} />
            )}

            {book.isbn13 && <p className="mt-2 text-xs text-muted">ISBN: {book.isbn13}</p>}
            {book.sourceUrl && (
              <p className="mt-2 flex flex-wrap items-center gap-3 text-xs text-muted">
                <Button type="button" variant="link" onClick={() => setReaderOpen(true)}>
                  Read in app
                </Button>
                <Button asChild variant="link">
                  <a href={book.sourceUrl} target="_blank" rel="noopener noreferrer">
                    Open original ↗
                  </a>
                </Button>
              </p>
            )}

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

          <section className="mt-8">
            <SectionHeading className="mb-3">Description</SectionHeading>
            {book.description ? (
              <BookDescription description={book.description} />
            ) : (
              <EmptyState placement="inline">No description available.</EmptyState>
            )}
          </section>

          {/* Admin: manual edit + live metadata source sync */}
          {isAdmin && (
            <section className="mt-8">
              <SectionHeading className="mb-3">Metadata source</SectionHeading>
              <BookSourceSync bookId={userBook.bookId} />
            </section>
          )}

          <section className="mt-8">
            <SectionHeading className="mb-3">Your reading</SectionHeading>
            <Card className="flex flex-col gap-4">
              {userBook.status === 'currently-reading' && (
                <div>
                  <SectionHeading level="eyebrow" className="mb-1">
                    Progress
                  </SectionHeading>
                  <BookProgressEditor userBook={userBook} onSaved={handleSaved} />
                </div>
              )}

              {(userBook.status === 'read' || userBook.finishedAt.length > 0) && (
                <BookReadDatesEditor userBook={userBook} onSaved={handleSaved} />
              )}

              {suggestion && (
                <div>
                  <SectionHeading level="eyebrow" className="mb-1">
                    Device
                  </SectionHeading>
                  <p className="text-sm">
                    {suggestion.label}
                    <span className="text-muted"> · {suggestion.detail}</span>
                  </p>
                </div>
              )}

              {/* Keyed off actual files, not the own-digital tag, which can drift. */}
              {readerFormatList.length > 0 && (
                <div>
                  <SectionHeading level="eyebrow" className="mb-1">
                    Kobo sync
                  </SectionHeading>
                  <KoboSyncToggle
                    bookId={userBook.bookId}
                    enabled={userBook.tags.includes('kobo-sync')}
                    onChanged={handleSaved}
                  />
                </div>
              )}

              {readerFormatList.length > 0 && (
                <div>
                  <SectionHeading level="eyebrow" className="mb-1">
                    Read
                  </SectionHeading>
                  <div className="flex gap-2 flex-wrap items-center">
                    <OfflineBookBadge bookId={userBook.bookId} />
                    {readerFormatList.length === 1 ? (
                      <Button asChild variant="secondary" size="sm">
                        <Link href={`/books/${userBook.id}/read`}>Open reader</Link>
                      </Button>
                    ) : (
                      readerFormatList.map((format) => (
                        <Button key={format} asChild variant="secondary" size="sm">
                          <Link href={`/books/${userBook.id}/read?format=${format}`}>
                            Open {format.toUpperCase()}
                          </Link>
                        </Button>
                      ))
                    )}
                    {/* A PDF-only book also links straight to its reflowable KEPUB. */}
                    {!userBook.formats.includes('epub') && (
                      <Button asChild variant="secondary" size="sm">
                        <Link href={`/books/${userBook.id}/read?format=kepub`}>Open KEPUB</Link>
                      </Button>
                    )}
                  </div>
                </div>
              )}

              {userBook.addedAt && (
                <p className="text-xs text-muted">Added {formatDate(userBook.addedAt)}</p>
              )}
            </Card>
          </section>
        </>
      )}

      {userBook && book && (
        <ArticleReaderDialog
          bookId={userBook.bookId}
          title={book.title}
          sourceUrl={book.sourceUrl}
          open={readerOpen}
          onOpenChange={setReaderOpen}
        />
      )}

      {userBook && (
        <RemoveBookDialog
          userBook={userBook}
          title={book?.title ?? 'this book'}
          open={removeOpen}
          onOpenChange={setRemoveOpen}
          onRemoved={() => router.push('/books/library')}
        />
      )}

      {book && isAdmin && (
        <BookEditDialog
          book={book}
          open={editOpen}
          onOpenChange={setEditOpen}
          onSaved={handleSaved}
        />
      )}
    </PageContainer>
  )
}
