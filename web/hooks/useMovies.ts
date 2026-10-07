import useSWR, { useSWRConfig } from 'swr'
import { useCallback } from 'react'
import { swrKeys } from '@/lib/swrKeys'
import { createServiceClient } from '@/lib/client'
import { MoviesService } from '@/lib/gen/movies/v1/movies_pb'
import type {
  GetTitleResponse,
  ListBacklogResponse,
  SearchTitlesResponse
} from '@/lib/gen/movies/v1/movies_pb'
import { DEFAULT_PAGE_SIZE } from '@/lib/pagination'
import type { BacklogFilter } from '@/lib/movies/format'

export function useMoviesBacklog({ status, mediaType, sort }: BacklogFilter) {
  const client = createServiceClient(MoviesService)
  return useSWR<ListBacklogResponse, Error>(swrKeys.moviesBacklog(status, mediaType, sort), () =>
    client.listBacklog({ status, mediaType, sort, limit: DEFAULT_PAGE_SIZE })
  )
}

export function useFetchMoviesBacklogPage({ status, mediaType, sort }: BacklogFilter) {
  // createServiceClient caches per service, so client is stable.
  const client = createServiceClient(MoviesService)
  return useCallback(
    (offset: number) =>
      client
        .listBacklog({ status, mediaType, sort, limit: DEFAULT_PAGE_SIZE, offset })
        .then((r) => ({ items: r.entries, hasMore: r.hasMore })),
    [client, status, mediaType, sort]
  )
}

/** TMDB search; no fetch for queries under two characters. */
export function useMovieSearch(query: string) {
  const client = createServiceClient(MoviesService)
  const trimmed = query.trim()
  return useSWR<SearchTitlesResponse, Error>(
    trimmed.length >= 2 ? swrKeys.moviesSearch(trimmed) : null,
    () => client.searchTitles({ query: trimmed }),
    { keepPreviousData: true }
  )
}

export function useMovieTitle(id: string) {
  const client = createServiceClient(MoviesService)
  return useSWR<GetTitleResponse, Error>(id ? swrKeys.moviesTitle(id) : null, () =>
    client.getTitle({ id })
  )
}

const isMoviesKey = (key: unknown) => {
  const path: unknown = Array.isArray(key) ? key[0] : key
  return typeof path === 'string' && path.startsWith('/movies/')
}

/** Backlog writes; each revalidates every movies list, search and title. */
export function useMoviesActions() {
  const client = createServiceClient(MoviesService)
  const { mutate } = useSWRConfig()
  const refresh = (except?: string) => mutate((key) => key !== except && isMoviesKey(key))

  // unknownDate dates new watches unknown instead of now; for a series marked
  // watched, that is every aired season it ticks.
  return {
    add: async (
      mediaType: string,
      tmdbId: bigint,
      status: 'want' | 'watched',
      unknownDate = false
    ) => {
      const res = await client.addTitle({ mediaType, tmdbId, status, unknownDate })
      await refresh()
      return res.entry
    },
    setStatus: async (id: string, status: string, unknownDate = false) => {
      await client.setStatus({ id, status, unknownDate })
      await refresh()
    },
    remove: async (id: string) => {
      await client.removeTitle({ id })
      // The removed title would only revalidate to NotFound.
      await refresh(swrKeys.moviesTitle(id))
    },
    setSeasonWatched: async (id: string, seasonNumber: number, watched: boolean) => {
      await client.setSeasonWatched({ id, seasonNumber, watched, unknownDate: false })
      await refresh()
    },
    // seasonNumber undefined targets the entry's own watches (movies); date
    // is YYYY-MM-DD or '' for unknown.
    addWatchDate: async (id: string, seasonNumber: number | undefined, date: string) => {
      await client.addWatchDate({ id, seasonNumber, date })
      await refresh()
    },
    editWatchDate: async (
      id: string,
      seasonNumber: number | undefined,
      index: number,
      date: string
    ) => {
      await client.editWatchDate({ id, seasonNumber, index, date })
      await refresh()
    },
    removeWatchDate: async (id: string, seasonNumber: number | undefined, index: number) => {
      await client.removeWatchDate({ id, seasonNumber, index })
      await refresh()
    }
  }
}
