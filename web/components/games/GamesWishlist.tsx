import { useWishlist } from '@/hooks/useGames'
import { Card } from '@/components/ui/card'
import { ErrorState } from '@/components/ui/states'
import { SectionHeading } from '@/components/ui/section-heading'

const storeUrl = (id: number) => `https://store.steampowered.com/app/${id}`

// Steam wishlist, synced with the library; hidden while empty.
export default function GamesWishlist() {
  const { data, error } = useWishlist()

  if (error) return <ErrorState what="Steam wishlist" placement="page" />
  const games = data?.games ?? []
  if (games.length === 0) return null

  return (
    <section className="mb-6">
      <SectionHeading className="mb-3">Wishlist ({games.length})</SectionHeading>
      <div className="grid grid-cols-1 sm:grid-cols-2 gap-3">
        {games.map((g) => (
          <Card key={g.id} className="relative flex gap-3">
            {g.imageUrl && (
              <img
                src={g.imageUrl}
                alt=""
                width={64}
                height={30}
                className="h-8 w-16 rounded-lg object-cover shrink-0"
              />
            )}
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
