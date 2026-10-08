import PodcastsClient from '@/components/podcasts/PodcastsClient'
import SWRFallback from '@/components/SWRFallback'
import { createServerClient } from '@/lib/server/client'
import { fetchOrNull } from '@/lib/server/fetchers'
import { swrKeys } from '@/lib/swrKeys'
import { DEFAULT_PAGE_SIZE } from '@/lib/pagination'
import { PodcastsService } from '@/lib/gen/podcasts/v1/podcasts_pb'

export default async function PodcastsPage() {
  const client = await createServerClient(PodcastsService)
  const [favourites, episodes] = await Promise.all([
    fetchOrNull(() => client.listFavourites({})),
    fetchOrNull(() => client.listEpisodes({ limit: DEFAULT_PAGE_SIZE }))
  ])

  return (
    <SWRFallback
      fallback={{
        ...(favourites && { [swrKeys.podcastsFavourites]: favourites }),
        ...(episodes && { [swrKeys.podcastsEpisodes]: episodes })
      }}
    >
      <PodcastsClient />
    </SWRFallback>
  )
}
