import useSWR, { useSWRConfig } from 'swr'
import { useCallback } from 'react'
import { swrKeys } from '@/lib/swrKeys'
import { createServiceClient } from '@/lib/client'
import { MoviesService } from '@/lib/gen/movies/v1/movies_pb'
import type {
  GetSettingsResponse,
  GetStatsResponse,
  GetTitleResponse,
  ListAvailableProvidersResponse,
  ListBacklogResponse,
  SearchTitlesResponse
} from '@/lib/gen/movies/v1/movies_pb'
import { DEFAULT_PAGE_SIZE } from '@/lib/pagination'
import { backlogRequest, type BacklogFilter } from '@/lib/movies/format'
import { sharedField } from '@/lib/listScope'
import { useSharedList } from '@/hooks/useSharedList'

export function useMoviesBacklog(filter: BacklogFilter) {
  const client = createServiceClient(MoviesService)
  const shared = useSharedList()
  const { status, mediaType, sort, onMyServices } = filter
  return useSWR<ListBacklogResponse, Error>(
    swrKeys.moviesBacklog(status, mediaType, sort, onMyServices, shared),
    () =>
      client.listBacklog({
        ...backlogRequest(filter),
        ...sharedField(shared),
        limit: DEFAULT_PAGE_SIZE
      })
  )
}

export function useFetchMoviesBacklogPage({
  status,
  mediaType,
  sort,
  onMyServices
}: BacklogFilter) {
  // createServiceClient caches per service, so client is stable.
  const client = createServiceClient(MoviesService)
  const shared = useSharedList()
  return useCallback(
    (offset: number) =>
      client
        .listBacklog({
          ...backlogRequest({ status, mediaType, sort, onMyServices }),
          ...sharedField(shared),
          limit: DEFAULT_PAGE_SIZE,
          offset
        })
        .then((r) => ({ items: r.entries, hasMore: r.hasMore })),
    [client, status, mediaType, sort, onMyServices, shared]
  )
}

/** TMDB search; no fetch for queries under two characters. */
export function useMovieSearch(query: string) {
  const client = createServiceClient(MoviesService)
  const shared = useSharedList()
  const trimmed = query.trim()
  return useSWR<SearchTitlesResponse, Error>(
    trimmed.length >= 2 ? swrKeys.moviesSearch(trimmed, shared) : null,
    () => client.searchTitles({ query: trimmed, ...sharedField(shared) }),
    { keepPreviousData: true }
  )
}

/** The caller's streaming services, as TMDB provider IDs. */
export function useMovieSettings() {
  const client = createServiceClient(MoviesService)
  return useSWR<GetSettingsResponse, Error>(swrKeys.moviesSettings, () => client.getSettings({}))
}

/** Belgian providers to pick services from. */
export function useAvailableProviders() {
  const client = createServiceClient(MoviesService)
  return useSWR<ListAvailableProvidersResponse, Error>(swrKeys.moviesProviders, () =>
    client.listAvailableProviders({})
  )
}

export function useMovieStats() {
  const client = createServiceClient(MoviesService)
  const shared = useSharedList()
  return useSWR<GetStatsResponse, Error>(swrKeys.moviesStats(shared), () =>
    client.getStats(sharedField(shared))
  )
}

export function useMovieTitle(id: string) {
  const client = createServiceClient(MoviesService)
  const shared = useSharedList()
  return useSWR<GetTitleResponse, Error>(id ? swrKeys.moviesTitle(id, shared) : null, () =>
    client.getTitle({ id, ...sharedField(shared) })
  )
}

const isMoviesKey = (key: unknown) => {
  const path: unknown = Array.isArray(key) ? key[0] : key
  return typeof path === 'string' && path.startsWith('/movies/')
}

/**
 * Backlog writes on the list the URL selects; each revalidates every movies
 * list, search and title. add and the other-list actions can name a list.
 */
export function useMoviesActions() {
  const client = createServiceClient(MoviesService)
  const shared = useSharedList()
  const { mutate } = useSWRConfig()
  const refresh = (except?: string) => mutate((key) => key !== except && isMoviesKey(key))

  // unknownDate dates new watches unknown instead of now; for a series marked
  // watched, that is every aired season it ticks.
  return {
    // Replaces the picked services; backlog flags and filters follow them.
    setServices: async (providerIds: bigint[]) => {
      await client.setSettings({ providerIds })
      await refresh()
    },
    add: async (
      mediaType: string,
      tmdbId: bigint,
      status: 'want' | 'watched',
      unknownDate = false,
      toShared = shared
    ) => {
      const res = await client.addTitle({
        mediaType,
        tmdbId,
        status,
        unknownDate,
        ...sharedField(toShared)
      })
      await refresh()
      return res.entry
    },
    setStatus: async (id: string, status: string, unknownDate = false) => {
      await client.setStatus({ id, status, unknownDate, ...sharedField(shared) })
      await refresh()
    },
    // rating is 1-5, or 0 to clear.
    setRating: async (id: string, rating: number) => {
      await client.setRating({
        id,
        rating: rating > 0 ? rating : undefined,
        ...sharedField(shared)
      })
      await refresh()
    },
    remove: async (id: string) => {
      await client.removeTitle({ id, ...sharedField(shared) })
      // The removed title would only revalidate to NotFound.
      await refresh(swrKeys.moviesTitle(id, shared))
    },
    setSeasonWatched: async (id: string, seasonNumber: number, watched: boolean) => {
      await client.setSeasonWatched({
        id,
        seasonNumber,
        watched,
        unknownDate: false,
        ...sharedField(shared)
      })
      await refresh()
    },
    // seasonNumber undefined targets the entry's own watches (movies); date
    // is YYYY-MM-DD or '' for unknown.
    addWatchDate: async (id: string, seasonNumber: number | undefined, date: string) => {
      await client.addWatchDate({ id, seasonNumber, date, ...sharedField(shared) })
      await refresh()
    },
    editWatchDate: async (
      id: string,
      seasonNumber: number | undefined,
      index: number,
      date: string
    ) => {
      await client.editWatchDate({ id, seasonNumber, index, date, ...sharedField(shared) })
      await refresh()
    },
    removeWatchDate: async (id: string, seasonNumber: number | undefined, index: number) => {
      await client.removeWatchDate({ id, seasonNumber, index, ...sharedField(shared) })
      await refresh()
    }
  }
}
