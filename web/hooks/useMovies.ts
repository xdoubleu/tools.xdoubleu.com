import useSWR, { useSWRConfig } from 'swr'
import { useCallback, useMemo } from 'react'
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
  const client = useMemo(() => createServiceClient(MoviesService), [])
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
  const refresh = () => mutate(isMoviesKey)

  return {
    add: async (mediaType: string, tmdbId: bigint, status: 'want' | 'watched') => {
      const res = await client.addTitle({ mediaType, tmdbId, status })
      await refresh()
      return res.entry
    },
    setStatus: async (id: string, status: string) => {
      await client.setStatus({ id, status })
      await refresh()
    },
    remove: async (id: string) => {
      await client.removeTitle({ id })
      await refresh()
    }
  }
}
