import { useRouter, useSearchParams } from '@/hooks/navigation'
import { mutate } from 'swr'
import { useLibrary } from '@/hooks/useBooks'
import BooksLibrary from '@/components/books/BooksLibrary'
import { swrKeys } from '@/lib/swrKeys'
import { LoadingState, ErrorState } from '@/components/ui/states'

export default function BooksSection() {
  const { data: libraryData, error: libError, isLoading: libLoading } = useLibrary()
  const router = useRouter()
  const searchParams = useSearchParams()
  // The query lives in ?q= so Back restores it after visiting a book.
  const query = searchParams.get('q') ?? ''
  const setQuery = (value: string) => {
    const params = new URLSearchParams(searchParams)
    if (value) {
      params.set('q', value)
    } else {
      params.delete('q')
    }
    router.replace(`/books/library${params.size ? `?${params}` : ''}`)
  }

  const library = libraryData?.library
  const knownShelves = library?.shelves.map((s) => s.name) ?? []

  const handleLibraryRefresh = () => {
    void mutate(swrKeys.books)
  }

  return (
    <section>
      {libLoading && <LoadingState label="books" placement="page" />}
      {libError && <ErrorState what="books" placement="page" />}
      {library && (
        <BooksLibrary
          library={library}
          knownShelves={knownShelves}
          searchQuery={query}
          onSearchChange={setQuery}
          onSaved={handleLibraryRefresh}
        />
      )}
    </section>
  )
}
