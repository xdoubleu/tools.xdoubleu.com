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
import { ShelfSidebar } from '@/components/ui/shelf-sidebar'
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
  const { data: wishlistData, error: wishlistError } = useWishlist()

  const { data: steamData, error: steamError, isLoading: steamLoading } = useSteam()

  const onSynced = useCallback(() => {
    void mutate(swrKeys.games)
  }, [])
  const { isRefreshing, lastRefresh, refresh } = useSteamRefresh(onSynced)

  const steam = steamData?.steam

  const filterGames = <T extends { name: string }>(games: T[]) => {
    const q = search.trim().toLowerCase()
    if (!q) return games
    return games.filter((g) => g.name.toLowerCase().includes(q))
  }

  const inProgress = steam ? filterGames(steam.inProgress) : []
  const notStarted = steam ? filterGames(steam.notStarted) : []
  const completed = steam ? filterGames(steam.completed) : []
  const favourites = [...inProgress, ...notStarted, ...completed].filter((g) => g.favourite)
  const wishlist = filterGames(wishlistData?.games ?? [])

  const labels: Record<Shelf, string> = {
    all: 'All',
    wishlist: 'Wishlist',
    'not-started': 'Not started',
    'in-progress': 'In progress',
    done: 'Done',
    favourites: 'Favourites'
  }
  // Counts ignore the search so they stay stable while typing.
  const owned = steam ? [...steam.inProgress, ...steam.notStarted, ...steam.completed] : []
  const counts: Record<Shelf, number> = {
    all: owned.length,
    wishlist: wishlistData?.games.length ?? 0,
    'not-started': steam?.notStarted.length ?? 0,
    'in-progress': steam?.inProgress.length ?? 0,
    done: steam?.completed.length ?? 0,
    favourites: owned.filter((g) => g.favourite).length
  }
  const groups = [
    { shelf: 'favourites', title: 'Favourites', games: favourites },
    { shelf: 'in-progress', title: 'In progress', games: inProgress },
    { shelf: 'not-started', title: 'Not started', games: notStarted },
    { shelf: 'done', title: 'Completed', games: completed }
  ]
  const onWishlist = shelf === 'wishlist'
  const visible = groups.filter((g) => shelf === 'all' || g.shelf === shelf)

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
            <Button variant="secondary" onClick={refresh} disabled={isRefreshing}>
              {isRefreshing ? 'Refreshing…' : 'Refresh'}
            </Button>
          </>
        }
      />

      <div className="flex flex-col md:flex-row gap-6">
        <ShelfSidebar
          shelves={SHELVES.map((s) => ({ id: s, label: labels[s], count: counts[s] }))}
          selectedShelfId={shelf}
          selectedTag={null}
          onSelectShelf={(id) => setShelf(SHELVES.find((s) => s === id) ?? 'all')}
          onSelectTag={() => {}}
        />
        <div className="flex-1 min-w-0">
          {(shelf === 'all' || onWishlist) && wishlistError && (
            <ErrorState what="Steam wishlist" placement="page" />
          )}
          {!onWishlist && steamLoading && <LoadingState label="Steam library" placement="page" />}
          {!onWishlist && steamError && <ErrorState what="Steam data" placement="page" />}
          {!onWishlist && steam && (
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
              {visible.every((g) => g.games.length === 0) &&
                (shelf !== 'all' || wishlist.length === 0) && (
                  <EmptyState placement="page">
                    {search.trim() ? 'No games match your search.' : 'No games on this shelf.'}
                  </EmptyState>
                )}
            </>
          )}
          {(shelf === 'all' || onWishlist) && !wishlistError && (
            <GamesWishlist games={wishlist} showEmpty={onWishlist} />
          )}
        </div>
      </div>
    </section>
  )
}
