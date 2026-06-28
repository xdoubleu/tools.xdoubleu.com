'use client'

import { useState } from 'react'
import { mutate } from 'swr'
import { useBacklogLibrary } from '@/hooks/useBacklog'
import BookSearchBar from '@/components/backlog/BookSearchBar'
import BooksLibrary from '@/components/backlog/BooksLibrary'

export default function BooksSection() {
  const { data: libraryData, error: libError, isLoading: libLoading } = useBacklogLibrary()
  const [query, setQuery] = useState('')
  const [hasLibraryResults, setHasLibraryResults] = useState(true)

  const library = libraryData?.library
  const knownShelves = library?.shelves.map((s) => s.name) ?? []

  const handleLibraryRefresh = () => {
    void mutate('/backlog/books')
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

      {libLoading && <p>Loading books...</p>}
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
