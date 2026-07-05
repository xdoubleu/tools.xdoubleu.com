'use client'

import { useMemo } from 'react'
import { mutate } from 'swr'
import { useLibrary } from '@/hooks/useBooks'
import { Breadcrumb, type BreadcrumbItem } from '@/components/ui/breadcrumb'
import BooksTable from '@/components/books/BooksTable'
import { SPECIAL_TAGS } from '@/lib/books/bookShelves'
import type { UserBook } from '@/lib/gen/books/v1/library_pb'
import { PageContainer } from '@/components/ui/page-container'
import { swrKeys } from '@/lib/swrKeys'

function flattenLibrary(
  library: NonNullable<ReturnType<typeof useLibrary>['data']>['library']
): UserBook[] {
  if (!library) return []
  return [
    ...library.reading,
    ...library.wishlist,
    ...library.finished,
    ...library.shelves.flatMap((s) => s.books)
  ]
}

interface AuthorBooksClientProps {
  name: string
}

export default function AuthorBooksClient({ name }: AuthorBooksClientProps) {
  const { data, error, isLoading } = useLibrary()

  const authorBooks = useMemo(() => {
    if (!data?.library) return []
    return flattenLibrary(data.library).filter((ub) => ub.book?.authors.includes(name))
  }, [data, name])

  const knownShelves = data?.library?.shelves.map((s) => s.name) ?? []

  const knownTags = useMemo(() => {
    if (!data?.library) return []
    const all = flattenLibrary(data.library)
    const seen = new Set<string>()
    for (const ub of all) {
      for (const t of ub.tags) {
        if (!SPECIAL_TAGS.has(t)) seen.add(t)
      }
    }
    return Array.from(seen).sort()
  }, [data])

  const breadcrumbItems: BreadcrumbItem[] = [
    { label: 'Backlog', href: '/backlog' },
    { label: 'Books', href: '/books' },
    { label: 'Library', href: '/books/library' },
    { label: name }
  ]

  const handleSaved = () => void mutate(swrKeys.books)

  return (
    <PageContainer className="space-y-4">
      <Breadcrumb items={breadcrumbItems} />
      <h1 className="text-2xl font-bold">{name}</h1>
      <p className="text-muted text-sm">
        {authorBooks.length} book{authorBooks.length !== 1 ? 's' : ''} in your library
      </p>

      {isLoading && <p className="text-muted text-sm">Loading...</p>}
      {error && <p className="text-danger text-sm">Failed to load library.</p>}

      {!isLoading && !error && (
        <BooksTable
          books={authorBooks}
          knownShelves={knownShelves}
          knownTags={knownTags}
          onSaved={handleSaved}
        />
      )}
    </PageContainer>
  )
}
