import useSWR, { useSWRConfig } from 'swr'
import { useCallback } from 'react'
import { swrKeys } from '@/lib/swrKeys'
import { createServiceClient } from '@/lib/client'
import { TasteService } from '@/lib/gen/taste/v1/taste_pb'
import type {
  GetProfileResponse,
  GetStatsResponse,
  ListEventsResponse
} from '@/lib/gen/taste/v1/taste_pb'
import { DEFAULT_PAGE_SIZE } from '@/lib/pagination'

export function useTasteStats() {
  const client = createServiceClient(TasteService)
  return useSWR<GetStatsResponse, Error>(swrKeys.tasteStats, () => client.getStats({}))
}

/** The first page of the log; domain '' is every domain. */
export function useTasteEvents(domain: string) {
  const client = createServiceClient(TasteService)
  return useSWR<ListEventsResponse, Error>(swrKeys.tasteEvents(domain), () =>
    client.listEvents({ domain, limit: DEFAULT_PAGE_SIZE })
  )
}

export function useFetchTasteEventsPage(domain: string) {
  // createServiceClient caches per service, so client is stable.
  const client = createServiceClient(TasteService)
  return useCallback(
    (offset: number) =>
      client
        .listEvents({ domain, limit: DEFAULT_PAGE_SIZE, offset })
        .then((r) => ({ items: r.events, hasMore: r.hasMore })),
    [client, domain]
  )
}

/** How often a pending profile is re-read until the job has written it. */
export const PROFILE_POLL_MS = 10_000

/** The taste profile, polled while a regeneration is pending. */
export function useTasteProfile() {
  const client = createServiceClient(TasteService)
  return useSWR<GetProfileResponse, Error>(swrKeys.tasteProfile, () => client.getProfile({}), {
    refreshInterval: (data) => (data?.pending ? PROFILE_POLL_MS : 0)
  })
}

/** Queues a profile regeneration, then re-reads the profile. */
export function useRefreshTasteProfile() {
  const client = createServiceClient(TasteService)
  const { mutate } = useSWRConfig()
  return useCallback(async () => {
    await client.refreshProfile({})
    await mutate(swrKeys.tasteProfile)
  }, [client, mutate])
}
