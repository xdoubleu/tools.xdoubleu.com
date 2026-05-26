'use client'

import { useState } from 'react'
import { useRouter } from 'next/navigation'
import Link from 'next/link'
import Image from 'next/image'
import {
  LineChart,
  Line,
  XAxis,
  YAxis,
  CartesianGrid,
  Tooltip,
  ResponsiveContainer
} from 'recharts'
import {
  useBacklogLibrary,
  useBacklogSteam,
  useBooksProgress,
  useSteamProgress
} from '@/hooks/useBacklog'
import { mutate } from 'swr'
import type { UserBook } from '@/lib/gen/backlog/v1/books_pb'
import type { Game } from '@/lib/gen/backlog/v1/games_pb'
import BookSearchBar from '@/components/backlog/BookSearchBar'
import BookEditModal from '@/components/backlog/BookEditModal'
import BooksProgressChart from '@/components/backlog/BooksProgressChart'
import SteamDistributionChart from '@/components/backlog/SteamDistributionChart'

function oneYearAgo(): string {
  const d = new Date()
  d.setFullYear(d.getFullYear() - 1)
  return d.toISOString().slice(0, 10)
}

function today(): string {
  return new Date().toISOString().slice(0, 10)
}

function BookCard({ userBook, onEdit }: { userBook: UserBook; onEdit: (ub: UserBook) => void }) {
  const book = userBook.book
  if (!book) return null
  return (
    <div className="border border-border rounded p-4 flex gap-4">
      {book.coverUrl && (
        <Image
          src={book.coverUrl}
          alt={book.title}
          width={40}
          height={60}
          className="object-cover rounded shrink-0"
        />
      )}
      <div className="flex-1 min-w-0">
        <h3 className="font-semibold">{book.title}</h3>
        <p className="text-sm text-muted">{book.authors.join(', ')}</p>
        <div className="flex items-center gap-2 mt-1 flex-wrap">
          <span className="text-xs px-2 py-0.5 rounded bg-surface text-subtle capitalize">
            {userBook.status}
          </span>
          {userBook.rating > 0 && <span className="text-xs text-muted">{userBook.rating}★</span>}
          {userBook.tags.includes('favourite') && <span className="text-xs text-amber-500">♥</span>}
        </div>
      </div>
      <button
        onClick={() => onEdit(userBook)}
        className="text-xs px-2 py-1 rounded bg-surface text-subtle hover:bg-border shrink-0 self-start"
      >
        Edit
      </button>
    </div>
  )
}

function GameCard({ game }: { game: Game }) {
  return (
    <Link
      href={`/backlog/steam/${game.id}`}
      className="block border border-border rounded p-4 hover:bg-surface transition-colors"
    >
      <h3 className="font-semibold">{game.name}</h3>
      <p className="text-sm text-muted">Playtime: {Math.round(game.playtime / 60)} hrs</p>
      <p className="text-sm text-muted">Completion: {game.completionRate}</p>
    </Link>
  )
}

type BooksTab = 'search' | 'library' | 'progress'
type SteamTab = 'backlog' | 'progress' | 'distribution'

function TabBar<T extends string>({
  tabs,
  active,
  onChange
}: {
  tabs: { id: T; label: string }[]
  active: T
  onChange: (t: T) => void
}) {
  return (
    <div className="flex gap-2 mb-4">
      {tabs.map((t) => (
        <button
          key={t.id}
          onClick={() => onChange(t.id)}
          className={`px-3 py-1.5 rounded text-sm ${active === t.id ? 'bg-blue-600 text-white' : 'bg-surface text-subtle hover:bg-border'}`}
        >
          {t.label}
        </button>
      ))}
    </div>
  )
}

function BooksSection() {
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
      <TabBar
        tabs={[
          { id: 'search' as BooksTab, label: 'Search' },
          { id: 'library' as BooksTab, label: 'Library' },
          { id: 'progress' as BooksTab, label: 'Progress' }
        ]}
        active={booksTab}
        onChange={setBooksTab}
      />

      {booksTab === 'search' && <BookSearchBar onAdded={handleLibraryRefresh} />}

      {booksTab === 'library' && (
        <>
          {libLoading && <p>Loading books...</p>}
          {libError && <p className="text-red-600">Failed to load books.</p>}
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
              <input
                id="books-from"
                type="date"
                value={progressStart}
                onChange={(e) => setProgressStart(e.target.value)}
                className="px-3 py-1.5 border border-input-border bg-input text-input-text rounded text-sm"
              />
            </div>
            <div>
              <label htmlFor="books-to" className="block text-xs text-muted mb-1">
                To
              </label>
              <input
                id="books-to"
                type="date"
                value={progressEnd}
                onChange={(e) => setProgressEnd(e.target.value)}
                className="px-3 py-1.5 border border-input-border bg-input text-input-text rounded text-sm"
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

function SteamSection() {
  const router = useRouter()
  const [steamTab, setSteamTab] = useState<SteamTab>('backlog')

  const [progressStart, setProgressStart] = useState(oneYearAgo())
  const [progressEnd, setProgressEnd] = useState(today())

  const { data: steamData, error: steamError, isLoading: steamLoading } = useBacklogSteam()
  const { data: progressData, isLoading: progressLoading } = useSteamProgress(
    progressStart,
    progressEnd
  )

  const steam = steamData?.steam
  const progressSteam = progressData?.steam

  const progressChartData =
    progressSteam?.labels?.map((label, idx) => ({
      label,
      value: parseFloat(progressSteam.values?.[idx] ?? '0')
    })) ?? []

  return (
    <section>
      <TabBar
        tabs={[
          { id: 'backlog' as SteamTab, label: 'Backlog' },
          { id: 'progress' as SteamTab, label: 'Progress' },
          { id: 'distribution' as SteamTab, label: 'Distribution' }
        ]}
        active={steamTab}
        onChange={setSteamTab}
      />

      {steamTab === 'backlog' && (
        <>
          {steamLoading && <p>Loading Steam library...</p>}
          {steamError && <p className="text-red-600">Failed to load Steam data.</p>}
          {steam && (
            <>
              <p className="mb-4 text-muted text-sm">
                Total backlog: {steam.totalBacklog} games &mdash; Current rate: {steam.currentRate}
              </p>
              {steam.inProgress.length > 0 && (
                <div className="mb-6">
                  <h2 className="text-lg font-semibold mb-3">
                    In Progress ({steam.inProgress.length})
                  </h2>
                  <div className="grid sm:grid-cols-2 gap-3">
                    {steam.inProgress.map((g) => (
                      <GameCard key={g.id} game={g} />
                    ))}
                  </div>
                </div>
              )}
              {steam.notStarted.length > 0 && (
                <div className="mb-6">
                  <h2 className="text-lg font-semibold mb-3">
                    Not Started ({steam.notStarted.length})
                  </h2>
                  <div className="grid sm:grid-cols-2 gap-3">
                    {steam.notStarted.map((g) => (
                      <GameCard key={g.id} game={g} />
                    ))}
                  </div>
                </div>
              )}
              {steam.completed.length > 0 && (
                <div className="mb-6">
                  <h2 className="text-lg font-semibold mb-3">
                    Completed ({steam.completed.length})
                  </h2>
                  <div className="grid sm:grid-cols-2 gap-3">
                    {steam.completed.map((g) => (
                      <GameCard key={g.id} game={g} />
                    ))}
                  </div>
                </div>
              )}
            </>
          )}
        </>
      )}

      {steamTab === 'progress' && (
        <div>
          <div className="flex gap-4 mb-4 flex-wrap">
            <div>
              <label htmlFor="steam-from" className="block text-xs text-muted mb-1">
                From
              </label>
              <input
                id="steam-from"
                type="date"
                value={progressStart}
                onChange={(e) => setProgressStart(e.target.value)}
                className="px-3 py-1.5 border border-input-border bg-input text-input-text rounded text-sm"
              />
            </div>
            <div>
              <label htmlFor="steam-to" className="block text-xs text-muted mb-1">
                To
              </label>
              <input
                id="steam-to"
                type="date"
                value={progressEnd}
                onChange={(e) => setProgressEnd(e.target.value)}
                className="px-3 py-1.5 border border-input-border bg-input text-input-text rounded text-sm"
              />
            </div>
          </div>
          {progressLoading && <p className="text-muted">Loading progress...</p>}
          {!progressLoading && progressChartData.length === 0 && (
            <p className="text-muted">No progress data for this range.</p>
          )}
          {progressChartData.length > 0 && (
            <div className="w-full h-64">
              <ResponsiveContainer width="100%" height="100%">
                <LineChart data={progressChartData}>
                  <CartesianGrid strokeDasharray="3 3" />
                  <XAxis dataKey="label" tick={{ fontSize: 11 }} />
                  <YAxis />
                  <Tooltip />
                  <Line type="monotone" dataKey="value" stroke="#3b82f6" dot={false} />
                </LineChart>
              </ResponsiveContainer>
            </div>
          )}
        </div>
      )}

      {steamTab === 'distribution' && (
        <>
          {steamLoading && <p>Loading distribution...</p>}
          {steam && (
            <SteamDistributionChart
              distribution={steam.distribution}
              onBucketClick={(bucket) => router.push(`/backlog/steam/distribution/${bucket}`)}
            />
          )}
          <p className="text-xs text-muted mt-2">Click a bar to see games in that range.</p>
        </>
      )}
    </section>
  )
}

type MainTabType = 'books' | 'steam'

export default function BacklogPage() {
  const [tab, setTab] = useState<MainTabType>('books')

  return (
    <main className="max-w-4xl mx-auto p-6">
      <h1 className="text-3xl font-bold mb-6">Backlog</h1>

      <div className="flex gap-2 mb-6">
        <button
          onClick={() => setTab('books')}
          className={`px-4 py-2 rounded ${tab === 'books' ? 'bg-blue-600 text-white' : 'bg-surface text-subtle'}`}
        >
          Books
        </button>
        <button
          onClick={() => setTab('steam')}
          className={`px-4 py-2 rounded ${tab === 'steam' ? 'bg-blue-600 text-white' : 'bg-surface text-subtle'}`}
        >
          Steam
        </button>
      </div>

      {tab === 'books' && <BooksSection />}
      {tab === 'steam' && <SteamSection />}
    </main>
  )
}
