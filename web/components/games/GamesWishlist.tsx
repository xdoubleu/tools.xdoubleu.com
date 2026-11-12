import type { WishlistGame } from '@/lib/gen/games/v1/games_pb'
import { Artwork } from '@/components/ui/artwork'
import { Card } from '@/components/ui/card'
import { headerFallback } from '@/lib/games/artwork'
import { EmptyState } from '@/components/ui/states'
import { SectionHeading } from '@/components/ui/section-heading'

const storeUrl = (id: number) => `https://store.steampowered.com/app/${id}`

// Steam wishlist cards; hidden while empty unless `showEmpty`.
export default function GamesWishlist({
  games,
  showEmpty = false
}: {
  games: WishlistGame[]
  showEmpty?: boolean
}) {
  if (games.length === 0) {
    return showEmpty ? <EmptyState placement="page">Nothing on your wishlist.</EmptyState> : null
  }

  return (
    <section className="mb-6">
      <SectionHeading className="mb-3">Wishlist ({games.length})</SectionHeading>
      <div className="grid grid-cols-1 sm:grid-cols-2 gap-3">
        {games.map((g) => (
          <Card key={g.id} className="relative flex gap-3">
            <Artwork
              src={g.imageUrl}
              fallbackSrc={headerFallback(g.imageUrl)}
              title={g.name}
              size="wide"
            />
            <h3 className="min-w-0 flex-1 break-words font-semibold">
              <a
                href={storeUrl(g.id)}
                target="_blank"
                rel="noopener noreferrer"
                className="after:absolute after:inset-0"
              >
                {g.name}
              </a>
            </h3>
          </Card>
        ))}
      </div>
    </section>
  )
}
