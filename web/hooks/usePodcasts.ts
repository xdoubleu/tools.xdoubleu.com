import useSWR, { useSWRConfig } from 'swr'
import { swrKeys } from '@/lib/swrKeys'
import { createServiceClient } from '@/lib/client'
import { PodcastsService } from '@/lib/gen/podcasts/v1/podcasts_pb'
import type { ListFavouritesResponse, SearchShowsResponse } from '@/lib/gen/podcasts/v1/podcasts_pb'

export function usePodcastFavourites() {
  const client = createServiceClient(PodcastsService)
  return useSWR<ListFavouritesResponse, Error>(swrKeys.podcastsFavourites, () =>
    client.listFavourites({})
  )
}

/** iTunes search; no fetch for queries under two characters. */
export function usePodcastSearch(query: string) {
  const client = createServiceClient(PodcastsService)
  const trimmed = query.trim()
  return useSWR<SearchShowsResponse, Error>(
    trimmed.length >= 2 ? swrKeys.podcastsSearch(trimmed) : null,
    () => client.searchShows({ query: trimmed }),
    { keepPreviousData: true }
  )
}

const isPodcastsKey = (key: unknown) => {
  const path: unknown = Array.isArray(key) ? key[0] : key
  return typeof path === 'string' && path.startsWith('/podcasts/')
}

/** Favourite writes; each revalidates the list and every search. */
export function usePodcastActions() {
  const client = createServiceClient(PodcastsService)
  const { mutate } = useSWRConfig()
  const refresh = () => mutate(isPodcastsKey)

  return {
    add: async (itunesId: bigint) => {
      await client.addFavourite({ itunesId })
      await refresh()
    },
    remove: async (id: string) => {
      await client.removeFavourite({ id })
      await refresh()
    }
  }
}
