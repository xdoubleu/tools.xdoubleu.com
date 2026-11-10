import { useWishlist } from '@/hooks/useGames'
import { Card } from '@/components/ui/card'
import { EmptyState, ErrorState } from '@/components/ui/states'
import { SectionHeading } from '@/components/ui/section-heading'

const storeUrl = (id: number) => `https://store.steampowered.com/app/${id}`

// Steam wishlist, synced with the library; hidden while empty unless `showEmpty`.
export default function GamesWishlist({
  search = '',
  showEmpty = false
}: {
  search?: string
  showEmpty?: boolean
}) {
  const { data, error } = useWishlist()

  if (error) return <ErrorState what="Steam wishlist" placement="page" />
  const q = search.trim().toLowerCase()
  const games = (data?.games ?? []).filter((g) => g.name.toLowerCase().includes(q))
  if (games.length === 0) {
    return showEmpty ? <EmptyState placement="page">Nothing on your wishlist.</EmptyState> : null
  }

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
