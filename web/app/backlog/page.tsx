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
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { interactiveCardClass } from '@/components/ui/card'
import { cn } from '@/lib/cn'

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

function GameCard({ game }: { game: Game }) {
  return (
    <Link href={`/backlog/steam/${game.id}`} className={cn(interactiveCardClass, 'block p-4')}>
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
        <Button
          key={t.id}
          variant={active === t.id ? 'default' : 'secondary'}
          size="sm"
          onClick={() => onChange(t.id)}
        >
          {t.label}
        </Button>
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
          {steamError && <p className="text-danger">Failed to load Steam data.</p>}
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
              <Input
                id="steam-from"
                type="date"
                value={progressStart}
                onChange={(e) => setProgressStart(e.target.value)}
                className="h-9 w-auto"
              />
            </div>
            <div>
              <label htmlFor="steam-to" className="block text-xs text-muted mb-1">
                To
              </label>
              <Input
                id="steam-to"
                type="date"
                value={progressEnd}
                onChange={(e) => setProgressEnd(e.target.value)}
                className="h-9 w-auto"
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
                  <Line
                    type="monotone"
                    dataKey="value"
                    stroke="rgb(var(--color-accent))"
                    strokeWidth={2}
                    dot={false}
                  />
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
        <Button variant={tab === 'books' ? 'default' : 'secondary'} onClick={() => setTab('books')}>
          Books
        </Button>
        <Button variant={tab === 'steam' ? 'default' : 'secondary'} onClick={() => setTab('steam')}>
          Steam
        </Button>
      </div>

      {tab === 'books' && <BooksSection />}
      {tab === 'steam' && <SteamSection />}
    </main>
  )
}
