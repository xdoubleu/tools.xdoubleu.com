'use client'

import { useCallback, useState } from 'react'
import Link from 'next/link'
import Image from 'next/image'
import { mutate } from 'swr'
import { useSteam } from '@/hooks/useGames'
import { useSteamRefresh } from '@/lib/games/steamRefresh'
import type { Game } from '@/lib/gen/games/v1/games_pb'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { interactiveCardClass } from '@/components/ui/card'
import { cn } from '@/lib/cn'

function GameCard({ game }: { game: Game }) {
  return (
    <Link href={`/games/${game.id}`} className={cn(interactiveCardClass, 'flex gap-3 p-4')}>
      {game.imageUrl && (
        <Image
          src={game.imageUrl}
          alt={game.name}
          width={32}
          height={32}
          className="h-8 w-8 rounded-lg object-cover shrink-0"
        />
      )}
      <div className="min-w-0 flex-1">
        <h3 className="font-semibold">{game.name}</h3>
        <p className="text-sm text-muted">Playtime: {Math.round(game.playtime / 60)} hrs</p>
        <p className="text-sm text-muted">Completion: {game.completionRate}%</p>
      </div>
    </Link>
  )
}

function GameGroup({ title, games }: { title: string; games: Game[] }) {
  if (games.length === 0) return null
  return (
    <div className="mb-6">
      <h2 className="text-lg font-semibold mb-3">
        {title} ({games.length})
      </h2>
      <div className="grid sm:grid-cols-2 gap-3">
        {games.map((g) => (
          <GameCard key={g.id} game={g} />
        ))}
      </div>
    </div>
  )
}

export default function GamesLibrary() {
  const [search, setSearch] = useState('')

  const { data: steamData, error: steamError, isLoading: steamLoading } = useSteam()

  const onSynced = useCallback(() => {
    void mutate('/games')
  }, [])
  const { isRefreshing, lastRefresh, refresh } = useSteamRefresh(onSynced)

  const steam = steamData?.steam

  const filterGames = (games: Game[]) => {
    const q = search.trim().toLowerCase()
    if (!q) return games
    return games.filter((g) => g.name.toLowerCase().includes(q))
  }

  const inProgress = steam ? filterGames(steam.inProgress) : []
  const notStarted = steam ? filterGames(steam.notStarted) : []
  const completed = steam ? filterGames(steam.completed) : []

  return (
    <section>
      <div className="mb-4 flex flex-col gap-2 sm:flex-row sm:items-center">
        <Input
          type="search"
          value={search}
          onChange={(e) => setSearch(e.target.value)}
          placeholder="Search games..."
          className="flex-1"
        />
        <div className="flex items-center gap-2">
          <Button variant="secondary" onClick={refresh} disabled={isRefreshing}>
            {isRefreshing ? 'Refreshing...' : 'Refresh'}
          </Button>
          {lastRefresh && !isRefreshing && (
            <span className="text-xs text-muted">Last: {lastRefresh.toLocaleString('en-GB')}</span>
          )}
        </div>
      </div>

      {steamLoading && <p>Loading Steam library...</p>}
      {steamError && <p className="text-danger">Failed to load Steam data.</p>}
      {steam && (
        <>
          <p className="mb-4 text-muted text-sm">
            Total backlog: {steam.totalBacklog} games &mdash; Current rate: {steam.currentRate}
          </p>
          <GameGroup title="In Progress" games={inProgress} />
          <GameGroup title="Not Started" games={notStarted} />
          <GameGroup title="Completed" games={completed} />
          {search.trim() &&
            inProgress.length === 0 &&
            notStarted.length === 0 &&
            completed.length === 0 && (
              <p className="text-muted text-sm">No games match your search.</p>
            )}
        </>
      )}
    </section>
  )
}
