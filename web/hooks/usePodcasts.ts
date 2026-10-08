import useSWR, { useSWRConfig } from 'swr'
import { useCallback } from 'react'
import { swrKeys } from '@/lib/swrKeys'
import { createServiceClient } from '@/lib/client'
import { PodcastsService } from '@/lib/gen/podcasts/v1/podcasts_pb'
import type {
  ListEpisodesResponse,
  ListFavouritesResponse,
  SearchShowsResponse
} from '@/lib/gen/podcasts/v1/podcasts_pb'
import { DEFAULT_PAGE_SIZE } from '@/lib/pagination'

export function usePodcastFavourites() {
  const client = createServiceClient(PodcastsService)
  return useSWR<ListFavouritesResponse, Error>(swrKeys.podcastsFavourites, () =>
    client.listFavourites({})
  )
}

/** The newest episodes across every favourite. */
export function usePodcastEpisodes() {
  const client = createServiceClient(PodcastsService)
  return useSWR<ListEpisodesResponse, Error>(swrKeys.podcastsEpisodes, () =>
    client.listEpisodes({ limit: DEFAULT_PAGE_SIZE })
  )
}

export function useFetchPodcastEpisodesPage() {
  // createServiceClient caches per service, so client is stable.
  const client = createServiceClient(PodcastsService)
  return useCallback(
    (offset: number) =>
      client
        .listEpisodes({ limit: DEFAULT_PAGE_SIZE, offset })
        .then((r) => ({ items: r.episodes, hasMore: r.hasMore })),
    // Stryker disable next-line ArrayDeclaration: equivalent; the client is cached per service, so it never changes.
    [client]
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

/** Favourite writes; each revalidates favourites, episodes and every search. */
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
