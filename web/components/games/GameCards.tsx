import type { Game } from '@/lib/gen/games/v1/games_pb'
import { Artwork } from '@/components/ui/artwork'
import { CollectionGrid } from '@/components/ui/collection-grid'
import { ItemCard } from '@/components/ui/item-card'
import { ProgressBar } from '@/components/ui/progress-bar'
import GameFavouriteButton from '@/components/games/GameFavouriteButton'
import { SectionHeading } from '@/components/ui/section-heading'
import { formatPercent } from '@/lib/format'

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
    <ItemCard
      href={href}
      title={game.name}
      artwork={<Artwork src={game.imageUrl} title={game.name} size="square" />}
      meta={
        <>
          {Math.round(game.playtime / 60)} hrs &middot; {formatPercent(game.completionRate)}{' '}
          complete
          {!showFavourite && game.favourite && (
            <span className="ml-2 text-star" aria-label="Favourite">
              ♥
            </span>
          )}
        </>
      }
      status={
        <ProgressBar
          value={Number(game.completionRate)}
          aria-label={`${game.name} completion`}
          valueText={formatPercent(game.completionRate)}
        />
      }
      actions={showFavourite && <GameFavouriteButton game={game} className="text-lg" />}
    />
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
      <SectionHeading className="mb-3">{title}</SectionHeading>
      <CollectionGrid>
        {games.map((g) => (
          <GameCard key={g.id} game={g} href={hrefFor(g)} showFavourite={showFavourite} />
        ))}
      </CollectionGrid>
    </div>
  )
}
