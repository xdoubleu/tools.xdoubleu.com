'use client'

import { useState } from 'react'
import { mutate } from 'swr'
import { useLibrary } from '@/hooks/useBooks'
import BookSearchBar from '@/components/books/BookSearchBar'
import BooksLibrary from '@/components/books/BooksLibrary'

export default function BooksSection() {
  const { data: libraryData, error: libError, isLoading: libLoading } = useLibrary()
  const [query, setQuery] = useState('')
  const [hasLibraryResults, setHasLibraryResults] = useState(true)

  const library = libraryData?.library
  const knownShelves = library?.shelves.map((s) => s.name) ?? []

  const handleLibraryRefresh = () => {
    void mutate('/books')
  }

  return (
    <section>
      <div className="mb-4">
        <BookSearchBar
          query={query}
          onChange={setQuery}
          onAdded={handleLibraryRefresh}
          hasLibraryResults={hasLibraryResults}
        />
      </div>

      {libLoading && <p>Loading books…</p>}
      {libError && <p className="text-danger">Failed to load books.</p>}
      {library && (
        <BooksLibrary
          library={library}
          knownShelves={knownShelves}
          searchQuery={query}
          onSearchResultsChange={setHasLibraryResults}
          onSaved={handleLibraryRefresh}
        />
      )}
    </section>
  )
}
