'use client'

import { useState } from 'react'
import { mutate } from 'swr'
import { useBacklogLibrary } from '@/hooks/useBacklog'
import type { UserBook } from '@/lib/gen/backlog/v1/books_pb'
import BookSearchBar from '@/components/backlog/BookSearchBar'
import BookEditModal from '@/components/backlog/BookEditModal'
import BooksLibrary from '@/components/backlog/BooksLibrary'

export default function BooksSection() {
  const [editingBook, setEditingBook] = useState<UserBook | null>(null)

  const { data: libraryData, error: libError, isLoading: libLoading } = useBacklogLibrary()

  const library = libraryData?.library

  const handleLibraryRefresh = () => {
    void mutate('/backlog/books')
  }

  return (
    <section>
      <div className="mb-4">
        <BookSearchBar onAdded={handleLibraryRefresh} />
      </div>

      {libLoading && <p>Loading books...</p>}
      {libError && <p className="text-danger">Failed to load books.</p>}
      {library && <BooksLibrary library={library} onEdit={setEditingBook} />}

      {editingBook && (
        <BookEditModal
          userBook={editingBook}
          onClose={() => setEditingBook(null)}
          onSaved={handleLibraryRefresh}
        />
      )}
    </section>
  )
}
