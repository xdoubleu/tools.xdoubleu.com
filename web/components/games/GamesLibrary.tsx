'use client'

import { useCallback, useState } from 'react'
import { mutate } from 'swr'
import { useSteam } from '@/hooks/useGames'
import { useSteamRefresh } from '@/lib/games/steamRefresh'
import type { Game, GetSteamResponse } from '@/lib/gen/games/v1/games_pb'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { GameGroup } from '@/components/games/GameCards'
import { swrKeys } from '@/lib/swrKeys'

const ownerGameHref = (game: Game) => `/games/${game.id}`

export default function GamesLibrary({ initialSteam }: { initialSteam?: GetSteamResponse }) {
  const [search, setSearch] = useState('')

  const { data: steamData, error: steamError, isLoading: steamLoading } = useSteam(initialSteam)

  const onSynced = useCallback(() => {
    void mutate(swrKeys.games)
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
  const favourites = [...inProgress, ...notStarted, ...completed].filter((g) => g.favourite)

  return (
    <section>
      <div className="mb-4 flex flex-col gap-2 sm:flex-row sm:items-center">
        <Input
          type="search"
          value={search}
          onChange={(e) => setSearch(e.target.value)}
          placeholder="Search games…"
          className="flex-1"
        />
        <div className="flex items-center gap-2">
          <Button variant="secondary" onClick={refresh} disabled={isRefreshing}>
            {isRefreshing ? 'Refreshing…' : 'Refresh'}
          </Button>
          {lastRefresh && !isRefreshing && (
            <span className="text-xs text-muted">Last: {lastRefresh.toLocaleString('en-GB')}</span>
          )}
        </div>
      </div>

      {steamLoading && <p className="text-muted">Loading Steam library…</p>}
      {steamError && <p className="text-danger">Failed to load Steam data.</p>}
      {steam && (
        <>
          <p className="mb-4 text-muted text-sm">
            Total backlog: {steam.totalBacklog} games &mdash; Current rate: {steam.currentRate}
          </p>
          <GameGroup title="Favourites" games={favourites} hrefFor={ownerGameHref} showFavourite />
          <GameGroup title="In Progress" games={inProgress} hrefFor={ownerGameHref} showFavourite />
          <GameGroup title="Not Started" games={notStarted} hrefFor={ownerGameHref} showFavourite />
          <GameGroup title="Completed" games={completed} hrefFor={ownerGameHref} showFavourite />
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
