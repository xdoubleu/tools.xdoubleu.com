'use client'

import Link from 'next/link'
import { useSharedLibrary, useSharedSteam } from '@/hooks/useProfile'
import type { GetSharedLibraryResponse } from '@/lib/gen/books/v1/public_pb'
import type { GetSharedSteamResponse } from '@/lib/gen/games/v1/public_pb'
import type { Game } from '@/lib/gen/games/v1/games_pb'
import BookCover from '@/components/books/BookCover'
import GamesStatCard from '@/components/games/GamesStatCard'
import { GameCard } from '@/components/games/GameCards'
import { Button } from '@/components/ui/button'
import { Card } from '@/components/ui/card'
import { ytdProgress } from '@/lib/books/ytdProgress'
import { statusLabel } from '@/lib/books/bookShelves'

export default function ProfileLanding({
  token,
  initialLibrary,
  initialSteam
}: {
  token: string
  initialLibrary?: GetSharedLibraryResponse
  initialSteam?: GetSharedSteamResponse
}) {
  const {
    data: libraryData,
    error: libraryError,
    isLoading: libraryLoading
  } = useSharedLibrary(token, initialLibrary)
  const {
    data: steamData,
    error: steamError,
    isLoading: steamLoading
  } = useSharedSteam(token, initialSteam)

  const library = libraryData?.library
  const steam = steamData?.steam

  if (libraryLoading || steamLoading) {
    if (!library && !steam) return <p className="text-muted">Loading profile…</p>
  }
  if (!library && !steam && (libraryError || steamError)) {
    return <p className="text-danger">This profile link is invalid or has been disabled.</p>
  }

  const allBooks = library
    ? [
        ...library.reading,
        ...library.wishlist,
        ...library.finished,
        ...library.shelves.flatMap((s) => s.books)
      ]
    : []
  const favouriteBooks = allBooks.filter((b) => b.tags.includes('favourite'))
  const ytd = ytdProgress(library?.finished ?? [])

  const allGames: Game[] = steam
    ? [...steam.inProgress, ...steam.notStarted, ...steam.completed]
    : []
  const favouriteGames = allGames.filter((g) => g.favourite)

  return (
    <div className="flex flex-col gap-10">
      {library && (
        <section>
          <div className="mb-3 flex items-center justify-between gap-3">
            <h2 className="text-xl font-semibold">Books</h2>
            <Button asChild variant="secondary" size="sm">
              <Link href={`/profile/${token}/books`}>View books</Link>
            </Button>
          </div>
          <div className="grid grid-cols-2 gap-3 sm:grid-cols-3 lg:grid-cols-5">
            <GamesStatCard label="Total books" value={allBooks.length} />
            <GamesStatCard
              label={statusLabel('currently-reading')}
              value={library.reading.length}
            />
            <GamesStatCard label={statusLabel('read')} value={library.finished.length} />
            <GamesStatCard label="Read this year" value={ytd.total} />
            <GamesStatCard label={statusLabel('to-read')} value={library.wishlist.length} />
          </div>
          {favouriteBooks.length > 0 && (
            <div className="mt-4">
              <h3 className="mb-2 text-sm font-semibold text-muted">Favourite books</h3>
              <div className="grid grid-cols-1 gap-3 sm:grid-cols-2 lg:grid-cols-3">
                {favouriteBooks.map((ub) => (
                  <Card key={ub.id} className="flex gap-3 p-4">
                    <BookCover coverUrl={ub.book?.coverUrl ?? ''} title={ub.book?.title ?? ''} />
                    <div className="min-w-0 flex-1">
                      <h4 className="font-semibold truncate">{ub.book?.title}</h4>
                      <p className="text-sm text-muted truncate">{ub.book?.authors.join(', ')}</p>
                    </div>
                  </Card>
                ))}
              </div>
            </div>
          )}
        </section>
      )}

      {steam && (
        <section>
          <div className="mb-3 flex items-center justify-between gap-3">
            <h2 className="text-xl font-semibold">Games</h2>
            <Button asChild variant="secondary" size="sm">
              <Link href={`/profile/${token}/games`}>View games</Link>
            </Button>
          </div>
          <div className="grid grid-cols-2 gap-3 sm:grid-cols-4">
            <GamesStatCard label="Total backlog" value={steam.totalBacklog} />
            <GamesStatCard label="Current rate" value={`${steam.currentRate}%`} />
            <GamesStatCard label="In progress" value={steam.inProgress.length} />
            <GamesStatCard label="Completed" value={steam.completed.length} />
          </div>
          {favouriteGames.length > 0 && (
            <div className="mt-4">
              <h3 className="mb-2 text-sm font-semibold text-muted">Favourite games</h3>
              <div className="grid grid-cols-1 gap-3 sm:grid-cols-2 lg:grid-cols-3">
                {favouriteGames.map((g) => (
                  <GameCard key={g.id} game={g} href={`/profile/${token}/games/${g.id}`} />
                ))}
              </div>
            </div>
          )}
        </section>
      )}
    </div>
  )
}
