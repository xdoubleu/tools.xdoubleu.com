import { useCallback, useState } from 'react'
import { mutate } from 'swr'
import { useSteam } from '@/hooks/useGames'
import { useSteamRefresh } from '@/lib/games/steamRefresh'
import type { Game } from '@/lib/gen/games/v1/games_pb'
import { Button } from '@/components/ui/button'
import { EmptyState, ErrorState, LoadingState } from '@/components/ui/states'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Toolbar } from '@/components/ui/toolbar'
import { GameGroup } from '@/components/games/GameCards'
import { swrKeys } from '@/lib/swrKeys'

const ownerGameHref = (game: Game) => `/games/${game.id}`

export default function GamesLibrary() {
  const [search, setSearch] = useState('')

  const { data: steamData, error: steamError, isLoading: steamLoading } = useSteam()

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
      <Toolbar
        aria-label="Filter games"
        search={
          <>
            <Label htmlFor="games-library-search" className="sr-only">
              Search games
            </Label>
            <Input
              id="games-library-search"
              type="search"
              value={search}
              onChange={(e) => setSearch(e.target.value)}
              placeholder="Search games…"
            />
          </>
        }
        trailing={
          <>
            {lastRefresh && !isRefreshing && (
              <span className="text-xs">Last: {lastRefresh.toLocaleString('en-GB')}</span>
            )}
            <Button variant="secondary" size="sm" onClick={refresh} disabled={isRefreshing}>
              {isRefreshing ? 'Refreshing…' : 'Refresh'}
            </Button>
          </>
        }
      />

      {steamLoading && <LoadingState label="Steam library" placement="page" />}
      {steamError && <ErrorState what="Steam data" placement="page" />}
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
              <EmptyState placement="page">No games match your search.</EmptyState>
            )}
        </>
      )}
    </section>
  )
}
