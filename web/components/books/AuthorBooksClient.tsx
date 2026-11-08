import { useMemo } from 'react'
import { mutate } from 'swr'
import { useLibrary } from '@/hooks/useBooks'
import { type BreadcrumbItem } from '@/components/ui/breadcrumb'
import { PageHeader } from '@/components/ui/page-header'
import { LoadingState, ErrorState } from '@/components/ui/states'
import BooksTable from '@/components/books/BooksTable'
import { LibraryTableControls } from '@/components/books/BooksTableToolbar'
import { useLibraryTableState } from '@/components/books/useLibraryTableState'
import { Toolbar } from '@/components/ui/toolbar'
import { SPECIAL_TAGS, flattenLibrary } from '@/lib/books/bookShelves'
import { PageContainer } from '@/components/ui/page-container'
import { swrKeys } from '@/lib/swrKeys'

interface AuthorBooksClientProps {
  name: string
}

export default function AuthorBooksClient({ name }: AuthorBooksClientProps) {
  const { data, error, isLoading } = useLibrary()
  const table = useLibraryTableState()

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
    { label: 'Books', href: '/dashboard/reading' },
    { label: 'Library', href: '/books/library' },
    { label: name }
  ]

  const handleSaved = () => void mutate(swrKeys.books)

  return (
    <PageContainer className="space-y-4">
      <PageHeader
        breadcrumb={breadcrumbItems}
        title={name}
        description={`${authorBooks.length} book${authorBooks.length !== 1 ? 's' : ''} in your library`}
      />

      {isLoading && <LoadingState placement="page" />}
      {error && <ErrorState what="library" placement="page" />}

      {!isLoading && !error && (
        <>
          <Toolbar aria-label="Filter books">
            <LibraryTableControls state={table} />
          </Toolbar>
          <BooksTable
            books={authorBooks}
            knownShelves={knownShelves}
            knownTags={knownTags}
            onSaved={handleSaved}
            state={table}
          />
        </>
      )}
    </PageContainer>
  )
}
