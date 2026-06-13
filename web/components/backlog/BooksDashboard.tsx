'use client'

import { useState } from 'react'
import Link from 'next/link'
import Image from 'next/image'
import { mutate } from 'swr'
import { useBacklogLibrary, useBooksProgress } from '@/hooks/useBacklog'
import type { UserBook } from '@/lib/gen/backlog/v1/books_pb'
import BookSearchBar from '@/components/backlog/BookSearchBar'
import BookEditModal from '@/components/backlog/BookEditModal'
import BooksProgressChart from '@/components/backlog/BooksProgressChart'
import BookProgressBar from '@/components/backlog/BookProgressBar'
import { Button } from '@/components/ui/button'
import { Card, interactiveCardClass } from '@/components/ui/card'
import { Input } from '@/components/ui/input'
import { cn } from '@/lib/cn'
import { oneYearAgo, today } from '@/lib/backlog/dates'

function StatCard({ label, value }: { label: string; value: string | number }) {
  return (
    <Card className="p-3">
      <p className="text-xs text-muted">{label}</p>
      <p className="text-xl font-bold mt-0.5">{value}</p>
    </Card>
  )
}

function ReadingBookCard({
  userBook,
  onEdit
}: {
  userBook: UserBook
  onEdit: (ub: UserBook) => void
}) {
  const book = userBook.book
  if (!book) return null
  return (
    <button
      type="button"
      onClick={() => onEdit(userBook)}
      className={cn(interactiveCardClass, 'flex gap-3 p-4 text-left')}
    >
      {book.coverUrl && (
        <Image
          src={book.coverUrl}
          alt={book.title}
          width={40}
          height={60}
          className="rounded-lg object-cover shrink-0"
        />
      )}
      <div className="min-w-0 flex-1">
        <h3 className="font-semibold truncate">{book.title}</h3>
        <p className="text-sm text-muted truncate">{book.authors.join(', ')}</p>
        <div className="mt-2">
          <BookProgressBar userBook={userBook} />
        </div>
      </div>
    </button>
  )
}

export default function BooksDashboard() {
  const [editingBook, setEditingBook] = useState<UserBook | null>(null)
  const [progressStart, setProgressStart] = useState(oneYearAgo())
  const [progressEnd, setProgressEnd] = useState(today())

  const { data: libraryData, error: libError, isLoading: libLoading } = useBacklogLibrary()
  const { data: progressData } = useBooksProgress(progressStart, progressEnd)

  const library = libraryData?.library
  const reading = library?.reading ?? []

  const handleRefresh = () => {
    void mutate('/backlog/books')
  }

  return (
    <section className="flex flex-col gap-3 lg:h-full lg:min-h-0">
      <div className="flex flex-wrap items-center justify-end gap-2">
        <div className="mr-auto w-full max-w-md">
          <BookSearchBar onAdded={handleRefresh} />
        </div>
        <Button asChild variant="secondary">
          <Link href="/backlog/books/library">Browse full library</Link>
        </Button>
      </div>

      {libLoading && <p>Loading dashboard...</p>}
      {libError && <p className="text-danger">Failed to load books.</p>}

      {library && (
        <div className="grid grid-cols-2 gap-3 sm:grid-cols-4">
          <StatCard
            label="Total books"
            value={reading.length + library.wishlist.length + library.finished.length}
          />
          <StatCard label="In progress" value={reading.length} />
          <StatCard label="Finished" value={library.finished.length} />
          <StatCard label="Wishlist" value={library.wishlist.length} />
        </div>
      )}

      <div className="grid gap-3 lg:min-h-0 lg:flex-1 lg:grid-cols-2">
        <div className="flex min-h-0 flex-col">
          <h2 className="mb-2 text-base font-semibold">Currently reading</h2>
          {!libLoading && reading.length === 0 && (
            <p className="text-muted text-sm">No books in progress.</p>
          )}
          {reading.length > 0 && (
            <div className="grid min-h-0 gap-3 overflow-y-auto pr-1 sm:grid-cols-2 lg:flex-1 lg:grid-cols-1">
              {reading.map((ub) => (
                <ReadingBookCard key={ub.id} userBook={ub} onEdit={setEditingBook} />
              ))}
            </div>
          )}
        </div>

        <div className="flex min-h-0 flex-col">
          <div className="mb-2 flex flex-wrap items-end justify-between gap-3">
            <h2 className="text-base font-semibold">Books finished over time</h2>
            <div className="flex gap-3">
              <div>
                <label htmlFor="books-dash-from" className="mb-1 block text-xs text-muted">
                  From
                </label>
                <Input
                  id="books-dash-from"
                  type="date"
                  value={progressStart}
                  onChange={(e) => setProgressStart(e.target.value)}
                  className="h-9 w-auto"
                />
              </div>
              <div>
                <label htmlFor="books-dash-to" className="mb-1 block text-xs text-muted">
                  To
                </label>
                <Input
                  id="books-dash-to"
                  type="date"
                  value={progressEnd}
                  onChange={(e) => setProgressEnd(e.target.value)}
                  className="h-9 w-auto"
                />
              </div>
            </div>
          </div>
          <BooksProgressChart data={progressData} />
        </div>
      </div>

      {editingBook && (
        <BookEditModal
          userBook={editingBook}
          onClose={() => setEditingBook(null)}
          onSaved={handleRefresh}
        />
      )}
    </section>
  )
}
