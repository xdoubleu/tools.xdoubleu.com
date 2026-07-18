'use client'

import { useState } from 'react'
import { useRouter } from 'next/navigation'
import { mutate } from 'swr'
import ReactMarkdown from 'react-markdown'
import remarkGfm from 'remark-gfm'
import { useExternalBook } from '@/hooks/useBooks'
import BookCover from '@/components/reading/BookCover'
import BookModal from '@/components/reading/BookModal'
import { Breadcrumb, type BreadcrumbItem } from '@/components/ui/breadcrumb'
import { Button } from '@/components/ui/button'
import { PageContainer } from '@/components/ui/page-container'
import { swrKeys } from '@/lib/swrKeys'
import { providerLabel } from '@/lib/reading/bookShelves'

interface ExternalBookDetailClientProps {
  provider: string
  providerId: string
}

export default function ExternalBookDetailClient({
  provider,
  providerId
}: ExternalBookDetailClientProps) {
  const { data, error, isLoading } = useExternalBook(provider, providerId)
  const [showAddModal, setShowAddModal] = useState(false)
  const router = useRouter()

  const book = data?.result

  const breadcrumbItems: BreadcrumbItem[] = [
    { label: 'Reading', href: '/reading' },
    { label: 'Library', href: '/reading/library' },
    { label: book?.title ?? 'Book' }
  ]

  return (
    <PageContainer className="p-6">
      <Breadcrumb items={breadcrumbItems} />

      {isLoading && <p className="mt-6 text-muted">Loading book…</p>}
      {error && <p className="mt-6 text-danger">Failed to load book.</p>}
      {!isLoading && !error && !book && <p className="mt-6 text-muted">Book not found.</p>}

      {book && (
        <>
          <div className="mt-6 flex flex-col gap-6 sm:flex-row sm:items-start">
            <div className="shrink-0">
              <BookCover coverUrl={book.coverUrl} title={book.title} size="lg" />
            </div>

            <div className="flex-1 min-w-0">
              <h1 className="text-3xl font-bold leading-tight">{book.title}</h1>
              {book.authors.length > 0 && (
                <p className="mt-1 text-lg text-muted">{book.authors.join(', ')}</p>
              )}

              <p className="mt-3 text-xs px-2 py-0.5 rounded-full bg-surface text-subtle inline-block">
                {providerLabel(book.provider)}
              </p>

              {book.isbn13 && <p className="mt-2 text-xs text-muted">ISBN: {book.isbn13}</p>}

              <div className="mt-4">
                <Button type="button" onClick={() => setShowAddModal(true)}>
                  Add to library
                </Button>
              </div>
            </div>
          </div>

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
        </>
      )}

      {showAddModal && book && (
        <BookModal
          book={book}
          onClose={() => setShowAddModal(false)}
          onAdded={() => {
            setShowAddModal(false)
            void mutate(swrKeys.books)
            router.push('/reading/library')
          }}
        />
      )}
    </PageContainer>
  )
}
