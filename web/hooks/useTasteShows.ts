import useSWR, { useSWRConfig } from 'swr'
import { swrKeys } from '@/lib/swrKeys'
import { createServiceClient } from '@/lib/client'
import { TasteService } from '@/lib/gen/taste/v1/taste_pb'
import type { ListFavouriteShowsResponse, SearchShowsResponse } from '@/lib/gen/taste/v1/taste_pb'

export function useShowFavourites() {
  const client = createServiceClient(TasteService)
  return useSWR<ListFavouriteShowsResponse, Error>(swrKeys.tasteShows, () =>
    client.listFavouriteShows({})
  )
}

/** iTunes search; no fetch for queries under two characters. */
export function useShowSearch(query: string) {
  const client = createServiceClient(TasteService)
  const trimmed = query.trim()
  return useSWR<SearchShowsResponse, Error>(
    trimmed.length >= 2 ? swrKeys.tasteShowSearch(trimmed) : null,
    () => client.searchShows({ query: trimmed }),
    { keepPreviousData: true }
  )
}

// Favourites also feed the taste log and stats.
const SHOW_KEY_PREFIXES = ['/taste/shows', '/taste/stats', '/taste/events']

const isShowsKey = (key: unknown) => {
  const path: unknown = Array.isArray(key) ? key[0] : key
  return typeof path === 'string' && SHOW_KEY_PREFIXES.some((p) => path.startsWith(p))
}

/** Favourite writes; each revalidates favourites, every search and the taste log. */
export function useShowActions() {
  const client = createServiceClient(TasteService)
  const { mutate } = useSWRConfig()
  const refresh = () => mutate(isShowsKey)

  return {
    add: async (itunesId: bigint) => {
      await client.addFavouriteShow({ itunesId })
      await refresh()
    },
    remove: async (id: string) => {
      await client.removeFavouriteShow({ id })
      await refresh()
    }
  }
}
