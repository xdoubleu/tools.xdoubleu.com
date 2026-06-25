'use client'

import { useState } from 'react'
import { mutate } from 'swr'
import { useBacklogLibrary } from '@/hooks/useBacklog'
import type { UserBook } from '@/lib/gen/backlog/v1/books_pb'
import type { BookActionKind } from '@/components/backlog/BookCard'
import BookSearchBar from '@/components/backlog/BookSearchBar'
import BookEntryModal from '@/components/backlog/BookEntryModal'
import BookShelfModal from '@/components/backlog/BookShelfModal'
import BookProgressModal from '@/components/backlog/BookProgressModal'
import BooksLibrary from '@/components/backlog/BooksLibrary'

interface ActiveEdit {
  kind: BookActionKind
  book: UserBook
}

export default function BooksSection() {
  const [activeEdit, setActiveEdit] = useState<ActiveEdit | null>(null)

  const { data: libraryData, error: libError, isLoading: libLoading } = useBacklogLibrary()

  const library = libraryData?.library

  const knownShelves = library?.shelves.map((s) => s.name) ?? []

  const handleLibraryRefresh = () => {
    void mutate('/backlog/books')
  }

  const handleAction = (kind: BookActionKind, book: UserBook) => {
    setActiveEdit({ kind, book })
  }

  const handleClose = () => setActiveEdit(null)

  return (
    <section>
      <div className="mb-4">
        <BookSearchBar onAdded={handleLibraryRefresh} />
      </div>

      {libLoading && <p>Loading books...</p>}
      {libError && <p className="text-danger">Failed to load books.</p>}
      {library && <BooksLibrary library={library} onAction={handleAction} />}

      {activeEdit?.kind === 'entry' && (
        <BookEntryModal
          key={activeEdit.book.id}
          userBook={activeEdit.book}
          onClose={handleClose}
          onSaved={handleLibraryRefresh}
        />
      )}
      {activeEdit?.kind === 'shelf' && (
        <BookShelfModal
          key={activeEdit.book.id}
          userBook={activeEdit.book}
          knownShelves={knownShelves}
          onClose={handleClose}
          onSaved={handleLibraryRefresh}
        />
      )}
      {activeEdit?.kind === 'progress' && (
        <BookProgressModal
          key={activeEdit.book.id}
          userBook={activeEdit.book}
          onClose={handleClose}
          onSaved={handleLibraryRefresh}
        />
      )}
    </section>
  )
}
