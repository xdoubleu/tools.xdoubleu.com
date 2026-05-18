'use client'

import { useState } from 'react'
import { useBacklogLibrary, useBacklogSteam } from '@/hooks/useBacklog'
import type { UserBook } from '@/lib/gen/backlog/v1/books_pb'
import type { Game } from '@/lib/gen/backlog/v1/games_pb'

function BookCard({ userBook }: { userBook: UserBook }) {
  const book = userBook.book
  if (!book) return null
  return (
    <div className="border rounded p-4 flex gap-4">
      {book.coverUrl && (
        <img
          src={book.coverUrl}
          alt={book.title}
          className="w-16 h-24 object-cover rounded"
        />
      )}
      <div>
        <h3 className="font-semibold">{book.title}</h3>
        <p className="text-sm text-gray-600">{book.authors.join(', ')}</p>
        <span className="text-xs px-2 py-0.5 rounded bg-gray-100 capitalize">
          {userBook.status}
        </span>
      </div>
    </div>
  )
}

function GameCard({ game }: { game: Game }) {
  return (
    <div className="border rounded p-4">
      <h3 className="font-semibold">{game.name}</h3>
      <p className="text-sm text-gray-600">
        Playtime: {Math.round(game.playtime / 60)} hrs
      </p>
      <p className="text-sm text-gray-600">
        Completion: {game.completionRate}
      </p>
    </div>
  )
}

type Tab = 'books' | 'steam'

export default function BacklogPage() {
  const [tab, setTab] = useState<Tab>('books')
  const { data: libraryData, error: libError, isLoading: libLoading } = useBacklogLibrary()
  const { data: steamData, error: steamError, isLoading: steamLoading } = useBacklogSteam()
  const library = libraryData?.library
  const steam = steamData?.steam

  return (
    <main className="max-w-4xl mx-auto p-6">
      <h1 className="text-3xl font-bold mb-6">Backlog</h1>

      <div className="flex gap-2 mb-6">
        <button
          onClick={() => setTab('books')}
          className={`px-4 py-2 rounded ${tab === 'books' ? 'bg-blue-600 text-white' : 'bg-gray-200'}`}
        >
          Books
        </button>
        <button
          onClick={() => setTab('steam')}
          className={`px-4 py-2 rounded ${tab === 'steam' ? 'bg-blue-600 text-white' : 'bg-gray-200'}`}
        >
          Steam
        </button>
      </div>

      {tab === 'books' && (
        <section>
          {libLoading && <p>Loading books...</p>}
          {libError && <p className="text-red-600">Failed to load books.</p>}
          {library && (
            <>
              {library.reading && library.reading.length > 0 && (
                <div className="mb-6">
                  <h2 className="text-xl font-semibold mb-3">Currently Reading</h2>
                  <div className="grid gap-3">
                    {library.reading.map((ub) => (
                      <BookCard key={ub.id} userBook={ub} />
                    ))}
                  </div>
                </div>
              )}
              {library.wishlist && library.wishlist.length > 0 && (
                <div className="mb-6">
                  <h2 className="text-xl font-semibold mb-3">Wishlist</h2>
                  <div className="grid gap-3">
                    {library.wishlist.map((ub) => (
                      <BookCard key={ub.id} userBook={ub} />
                    ))}
                  </div>
                </div>
              )}
              {library.finished && library.finished.length > 0 && (
                <div className="mb-6">
                  <h2 className="text-xl font-semibold mb-3">Finished</h2>
                  <div className="grid gap-3">
                    {library.finished.map((ub) => (
                      <BookCard key={ub.id} userBook={ub} />
                    ))}
                  </div>
                </div>
              )}
              {library.shelves.map((shelf) => (
                <div key={shelf.name} className="mb-6">
                  <h2 className="text-xl font-semibold mb-3">{shelf.name}</h2>
                  <div className="grid gap-3">
                    {shelf.books.map((ub) => (
                      <BookCard key={ub.id} userBook={ub} />
                    ))}
                  </div>
                </div>
              ))}
            </>
          )}
        </section>
      )}

      {tab === 'steam' && (
        <section>
          {steamLoading && <p>Loading Steam library...</p>}
          {steamError && <p className="text-red-600">Failed to load Steam data.</p>}
          {steam && (
            <>
              <p className="mb-4 text-gray-600">
                Total backlog: {steam.totalBacklog} games &mdash; Current rate:{' '}
                {steam.currentRate}
              </p>
              {steam.inProgress.length > 0 && (
                <div className="mb-6">
                  <h2 className="text-xl font-semibold mb-3">In Progress</h2>
                  <div className="grid sm:grid-cols-2 gap-3">
                    {steam.inProgress.map((g) => (
                      <GameCard key={g.id} game={g} />
                    ))}
                  </div>
                </div>
              )}
              {steam.notStarted.length > 0 && (
                <div className="mb-6">
                  <h2 className="text-xl font-semibold mb-3">Not Started</h2>
                  <div className="grid sm:grid-cols-2 gap-3">
                    {steam.notStarted.map((g) => (
                      <GameCard key={g.id} game={g} />
                    ))}
                  </div>
                </div>
              )}
              {steam.completed.length > 0 && (
                <div className="mb-6">
                  <h2 className="text-xl font-semibold mb-3">Completed</h2>
                  <div className="grid sm:grid-cols-2 gap-3">
                    {steam.completed.map((g) => (
                      <GameCard key={g.id} game={g} />
                    ))}
                  </div>
                </div>
              )}
            </>
          )}
        </section>
      )}
    </main>
  )
}
