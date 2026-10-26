import { useState } from 'react'
import { useSharedSteam } from '@/hooks/useDashboardShare'
import type { Game } from '@/lib/gen/games/v1/games_pb'
import { GameGroup } from '@/components/games/GameCards'
import { Input } from '@/components/ui/input'
import { EmptyState, ErrorState, LoadingState } from '@/components/ui/states'

// Read-only shared games library: the same grouped game grid as the owner's
// /games/library, with game cards linking to the public game pages.
export default function GamesDashboardPublicLibrary({ token }: { token: string }) {
  const [search, setSearch] = useState('')

  const { data, error, isLoading } = useSharedSteam(token)

  const steam = data?.steam
  const gameHref = (game: Game) => `/dashboard/games/${token}/${game.id}`

  if (isLoading && !steam) return <LoadingState label="games" />
  if (error && !steam) return <ErrorState what="games" />
  if (!steam) return null

  const filterGames = (games: Game[]) => {
    const q = search.trim().toLowerCase()
    if (!q) return games
    return games.filter((g) => g.name.toLowerCase().includes(q))
  }

  const inProgress = filterGames(steam.inProgress)
  const notStarted = filterGames(steam.notStarted)
  const completed = filterGames(steam.completed)
  const favourites = [...inProgress, ...notStarted, ...completed].filter((g) => g.favourite)

  return (
    <div>
      <Input
        type="search"
        value={search}
        onChange={(e) => setSearch(e.target.value)}
        aria-label="Search games"
        placeholder="Search games…"
        className="mb-4 max-w-md"
      />
      <p className="mb-4 text-muted text-sm">
        Total backlog: {steam.totalBacklog} games &mdash; Current rate: {steam.currentRate}
      </p>
      <GameGroup title="Favourites" games={favourites} hrefFor={gameHref} />
      <GameGroup title="In Progress" games={inProgress} hrefFor={gameHref} />
      <GameGroup title="Not Started" games={notStarted} hrefFor={gameHref} />
      <GameGroup title="Completed" games={completed} hrefFor={gameHref} />
      {search.trim() &&
        inProgress.length === 0 &&
        notStarted.length === 0 &&
        completed.length === 0 && <EmptyState>No games match your search.</EmptyState>}
    </div>
  )
}
