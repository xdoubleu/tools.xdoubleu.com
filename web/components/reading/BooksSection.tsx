'use client'

import { useState } from 'react'
import { useRouter, useSearchParams } from 'next/navigation'
import { mutate } from 'swr'
import { useLibrary } from '@/hooks/useBooks'
import AddToLibraryDialog from '@/components/reading/AddToLibraryDialog'
import BookSearchBar from '@/components/reading/BookSearchBar'
import BooksLibrary from '@/components/reading/BooksLibrary'
import { Button } from '@/components/ui/button'
import { swrKeys } from '@/lib/swrKeys'

export default function BooksSection() {
  const { data: libraryData, error: libError, isLoading: libLoading } = useLibrary()
  const [addOpen, setAddOpen] = useState(false)
  const router = useRouter()
  const searchParams = useSearchParams()
  // The query lives in the URL (?q=) rather than component state so that
  // navigating to a book and hitting Back restores it — component state
  // resets on remount, the URL doesn't.
  const query = searchParams.get('q') ?? ''
  const setQuery = (value: string) => {
    const params = new URLSearchParams(searchParams)
    if (value) {
      params.set('q', value)
    } else {
      params.delete('q')
    }
    router.replace(`/reading/library${params.size ? `?${params}` : ''}`, { scroll: false })
  }

  const library = libraryData?.library
  const knownShelves = library?.shelves.map((s) => s.name) ?? []

  const handleLibraryRefresh = () => {
    void mutate(swrKeys.books)
  }

  return (
    <section>
      <div className="mb-4 flex items-start gap-2">
        <div className="min-w-0 flex-1">
          <BookSearchBar query={query} onChange={setQuery} onAdded={handleLibraryRefresh} />
        </div>
        <Button onClick={() => setAddOpen(true)}>Add to library</Button>
      </div>
      <AddToLibraryDialog open={addOpen} onOpenChange={setAddOpen} onAdded={handleLibraryRefresh} />

      {libLoading && <p className="text-muted">Loading books…</p>}
      {libError && <p className="text-danger">Failed to load books.</p>}
      {library && (
        <BooksLibrary
          library={library}
          knownShelves={knownShelves}
          searchQuery={query}
          onSaved={handleLibraryRefresh}
        />
      )}
    </section>
  )
}
