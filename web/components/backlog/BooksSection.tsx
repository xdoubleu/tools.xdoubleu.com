'use client'

import { useState } from 'react'
import Image from 'next/image'
import { mutate } from 'swr'
import { useBacklogLibrary, useBooksProgress } from '@/hooks/useBacklog'
import type { UserBook } from '@/lib/gen/backlog/v1/books_pb'
import BookSearchBar from '@/components/backlog/BookSearchBar'
import BookEditModal from '@/components/backlog/BookEditModal'
import BooksProgressChart from '@/components/backlog/BooksProgressChart'
import SectionTabBar from '@/components/backlog/SectionTabBar'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { oneYearAgo, today } from '@/lib/backlog/dates'

type BooksTab = 'library' | 'progress'

function BookCard({ userBook, onEdit }: { userBook: UserBook; onEdit: (ub: UserBook) => void }) {
  const book = userBook.book
  if (!book) return null
  return (
    <div className="border border-border rounded-2xl p-4 flex gap-4">
      {book.coverUrl && (
        <Image
          src={book.coverUrl}
          alt={book.title}
          width={40}
          height={60}
          className="object-cover rounded-lg shrink-0"
        />
      )}
      <div className="flex-1 min-w-0">
        <h3 className="font-semibold">{book.title}</h3>
        <p className="text-sm text-muted">{book.authors.join(', ')}</p>
        <div className="flex items-center gap-2 mt-1 flex-wrap">
          <span className="text-xs px-2 py-0.5 rounded-full bg-surface text-subtle capitalize">
            {userBook.status}
          </span>
          {userBook.rating > 0 && <span className="text-xs text-muted">{userBook.rating}★</span>}
          {userBook.tags.includes('favourite') && <span className="text-xs text-amber-500">♥</span>}
        </div>
      </div>
      <Button
        variant="secondary"
        size="sm"
        onClick={() => onEdit(userBook)}
        className="shrink-0 self-start"
      >
        Edit
      </Button>
    </div>
  )
}

export default function BooksSection() {
  const [booksTab, setBooksTab] = useState<BooksTab>('library')
  const [editingBook, setEditingBook] = useState<UserBook | null>(null)

  const [progressStart, setProgressStart] = useState(oneYearAgo())
  const [progressEnd, setProgressEnd] = useState(today())

  const { data: libraryData, error: libError, isLoading: libLoading } = useBacklogLibrary()
  const { data: progressData } = useBooksProgress(progressStart, progressEnd)

  const library = libraryData?.library

  const handleLibraryRefresh = () => {
    mutate('/backlog/books')
  }

  return (
    <section>
      <div className="mb-4">
        <BookSearchBar onAdded={handleLibraryRefresh} />
      </div>

      <SectionTabBar
        tabs={[
          { id: 'library' as BooksTab, label: 'Library' },
          { id: 'progress' as BooksTab, label: 'Progress' }
        ]}
        active={booksTab}
        onChange={setBooksTab}
      />

      {booksTab === 'library' && (
        <>
          {libLoading && <p>Loading books...</p>}
          {libError && <p className="text-danger">Failed to load books.</p>}
          {library && (
            <>
              {library.reading.length > 0 && (
                <div className="mb-6">
                  <h2 className="text-lg font-semibold mb-3">
                    Currently Reading ({library.reading.length})
                  </h2>
                  <div className="grid gap-3">
                    {library.reading.map((ub) => (
                      <BookCard key={ub.id} userBook={ub} onEdit={setEditingBook} />
                    ))}
                  </div>
                </div>
              )}
              {library.wishlist.length > 0 && (
                <div className="mb-6">
                  <h2 className="text-lg font-semibold mb-3">
                    Wishlist ({library.wishlist.length})
                  </h2>
                  <div className="grid gap-3">
                    {library.wishlist.map((ub) => (
                      <BookCard key={ub.id} userBook={ub} onEdit={setEditingBook} />
                    ))}
                  </div>
                </div>
              )}
              {library.finished.length > 0 && (
                <div className="mb-6">
                  <h2 className="text-lg font-semibold mb-3">
                    Finished ({library.finished.length})
                  </h2>
                  <div className="grid gap-3">
                    {library.finished.map((ub) => (
                      <BookCard key={ub.id} userBook={ub} onEdit={setEditingBook} />
                    ))}
                  </div>
                </div>
              )}
              {library.shelves.map((shelf) => (
                <div key={shelf.name} className="mb-6">
                  <h2 className="text-lg font-semibold mb-3">
                    {shelf.name} ({shelf.books.length})
                  </h2>
                  <div className="grid gap-3">
                    {shelf.books.map((ub) => (
                      <BookCard key={ub.id} userBook={ub} onEdit={setEditingBook} />
                    ))}
                  </div>
                </div>
              ))}
            </>
          )}
        </>
      )}

      {booksTab === 'progress' && (
        <div>
          <div className="flex gap-4 mb-4 flex-wrap">
            <div>
              <label htmlFor="books-from" className="block text-xs text-muted mb-1">
                From
              </label>
              <Input
                id="books-from"
                type="date"
                value={progressStart}
                onChange={(e) => setProgressStart(e.target.value)}
                className="h-9 w-auto"
              />
            </div>
            <div>
              <label htmlFor="books-to" className="block text-xs text-muted mb-1">
                To
              </label>
              <Input
                id="books-to"
                type="date"
                value={progressEnd}
                onChange={(e) => setProgressEnd(e.target.value)}
                className="h-9 w-auto"
              />
            </div>
          </div>
          <BooksProgressChart data={progressData} />
        </div>
      )}

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
