import { useCallback, useState } from 'react'
import { mutate } from 'swr'
import { useRouter, useSearchParams } from '@/hooks/navigation'
import { useSteam, useWishlist } from '@/hooks/useGames'
import { useSteamRefresh } from '@/lib/games/steamRefresh'
import type { Game } from '@/lib/gen/games/v1/games_pb'
import { Button } from '@/components/ui/button'
import { EmptyState, ErrorState, LoadingState } from '@/components/ui/states'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Toolbar } from '@/components/ui/toolbar'
import GamesWishlist from '@/components/games/GamesWishlist'
import { SegmentedTabs } from '@/components/ui/segmented-tabs'
import { GameGroup } from '@/components/games/GameCards'
import { swrKeys } from '@/lib/swrKeys'

const ownerGameHref = (game: Game) => `/games/${game.id}`

const SHELVES = ['all', 'wishlist', 'not-started', 'in-progress', 'done', 'favourites'] as const
type Shelf = (typeof SHELVES)[number]

const isShelf = (v: string | null): v is Shelf => SHELVES.some((s) => s === v)

export default function GamesLibrary() {
  const [search, setSearch] = useState('')
  const router = useRouter()
  const searchParams = useSearchParams()
  // The shelf lives in ?shelf= so reload and Back keep it.
  const param = searchParams.get('shelf')
  const shelf: Shelf = isShelf(param) ? param : 'all'
  const setShelf = (value: Shelf) => {
    const params = new URLSearchParams(searchParams)
    if (value === 'all') params.delete('shelf')
    else params.set('shelf', value)
    router.replace(`/games/library${params.size ? `?${params}` : ''}`)
  }
  const { data: wishlistData } = useWishlist()

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

  const labels: Record<Shelf, string> = {
    all: 'All',
    wishlist: 'Wishlist',
    'not-started': 'Not started',
    'in-progress': 'In progress',
    done: 'Done',
    favourites: 'Favourites'
  }
  const counts: Record<Shelf, number> = {
    all: inProgress.length + notStarted.length + completed.length,
    wishlist: wishlistData?.games.length ?? 0,
    'not-started': notStarted.length,
    'in-progress': inProgress.length,
    done: completed.length,
    favourites: favourites.length
  }
  const groups = [
    { shelf: 'favourites', title: 'Favourites', games: favourites },
    { shelf: 'in-progress', title: 'In Progress', games: inProgress },
    { shelf: 'not-started', title: 'Not Started', games: notStarted },
    { shelf: 'done', title: 'Completed', games: completed }
  ]
  const visible = groups.filter((g) => shelf === 'all' || g.shelf === shelf)

  return (
    <section>
      <SegmentedTabs
        aria-label="Shelves"
        className="mb-4"
        value={shelf}
        onChange={setShelf}
        options={SHELVES.map((s) => ({ value: s, label: `${labels[s]} (${counts[s]})` }))}
      />
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
            <Button variant="secondary" onClick={refresh} disabled={isRefreshing}>
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
          {visible.map((g) => (
            <GameGroup
              key={g.shelf}
              title={g.title}
              games={g.games}
              hrefFor={ownerGameHref}
              showFavourite
            />
          ))}
          {shelf !== 'wishlist' && visible.every((g) => g.games.length === 0) && (
            <EmptyState placement="page">
              {search.trim() ? 'No games match your search.' : 'No games on this shelf.'}
            </EmptyState>
          )}
        </>
      )}
      {(shelf === 'all' || shelf === 'wishlist') && (
        <GamesWishlist search={search} showEmpty={shelf === 'wishlist'} />
      )}
    </section>
  )
}
