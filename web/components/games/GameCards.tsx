import Link from 'next/link'
import Image from 'next/image'
import type { Game } from '@/lib/gen/games/v1/games_pb'
import { interactiveCardClass } from '@/components/ui/card'
import { CardLinkStatus } from '@/components/ui/CardLinkStatus'
import GameFavouriteButton from '@/components/games/GameFavouriteButton'
import { cn } from '@/lib/cn'

// Shared by the owner's games library and the public profile pages; the
// caller decides where a card links to (owner detail vs public detail), and
// whether the favourite indicator is an interactive toggle (owner) or a
// static marker (public profile — showFavourite defaults to false).
export function GameCard({
  game,
  href,
  showFavourite = false
}: {
  game: Game
  href: string
  showFavourite?: boolean
}) {
  return (
    <Link href={href} className={cn(interactiveCardClass, 'relative flex gap-3 p-4')}>
      <CardLinkStatus />
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
        <h3 className="font-semibold">
          {game.name}
          {!showFavourite && game.favourite && (
            <span className="ml-2 text-amber-500" aria-label="Favourite">
              ♥
            </span>
          )}
        </h3>
        <p className="text-sm text-muted">Playtime: {Math.round(game.playtime / 60)} hrs</p>
        <p className="text-sm text-muted">Completion: {game.completionRate}%</p>
      </div>
      {showFavourite && (
        <GameFavouriteButton game={game} className="absolute right-3 top-3 z-10 text-lg" />
      )}
    </Link>
  )
}

export function GameGroup({
  title,
  games,
  hrefFor,
  showFavourite = false
}: {
  title: string
  games: Game[]
  hrefFor: (game: Game) => string
  showFavourite?: boolean
}) {
  if (games.length === 0) return null
  return (
    <div className="mb-6">
      <h2 className="text-lg font-semibold mb-3">
        {title} ({games.length})
      </h2>
      <div className="grid sm:grid-cols-2 gap-3">
        {games.map((g) => (
          <GameCard key={g.id} game={g} href={hrefFor(g)} showFavourite={showFavourite} />
        ))}
      </div>
    </div>
  )
}
