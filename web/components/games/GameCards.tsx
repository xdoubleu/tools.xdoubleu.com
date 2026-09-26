import Image from 'next/image'
import type { Game } from '@/lib/gen/games/v1/games_pb'
import { LinkCard } from '@/components/ui/link-card'
import GameFavouriteButton from '@/components/games/GameFavouriteButton'

// Shared by the owner library and public profiles; the caller picks the link
// target and whether the favourite is an interactive toggle.
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
    <LinkCard
      href={href}
      linkClassName="flex gap-3 p-4"
      actions={showFavourite && <GameFavouriteButton game={game} className="text-lg" />}
    >
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
        <h3 className="break-words font-semibold">
          {game.name}
          {!showFavourite && game.favourite && (
            <span className="ml-2 text-star" aria-label="Favourite">
              ♥
            </span>
          )}
        </h3>
        <p className="text-sm text-muted">Playtime: {Math.round(game.playtime / 60)} hrs</p>
        <p className="text-sm text-muted">Completion: {game.completionRate}%</p>
      </div>
    </LinkCard>
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
