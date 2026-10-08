import PodcastsClient from '@/components/podcasts/PodcastsClient'
import SWRFallback from '@/components/SWRFallback'
import { createServerClient } from '@/lib/server/client'
import { fetchOrNull } from '@/lib/server/fetchers'
import { swrKeys } from '@/lib/swrKeys'
import { PodcastsService } from '@/lib/gen/podcasts/v1/podcasts_pb'

export default async function PodcastsPage() {
  const client = await createServerClient(PodcastsService)
  const favourites = await fetchOrNull(() => client.listFavourites({}))

  return (
    <SWRFallback fallback={{}} keyed={favourites ? [[swrKeys.podcastsFavourites, favourites]] : []}>
      <PodcastsClient />
    </SWRFallback>
  )
}
