import useSWR, { useSWRConfig } from 'swr'
import { useCallback } from 'react'
import { swrKeys } from '@/lib/swrKeys'
import { createServiceClient } from '@/lib/client'
import { TasteService } from '@/lib/gen/taste/v1/taste_pb'
import type {
  GetProfileResponse,
  GetStatsResponse,
  ListEventsResponse,
  ListRecommendationsResponse
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

/** Re-read delay: poll while pending, else re-read once the cooldown ends. */
export function profileRefreshInterval(data: GetProfileResponse | undefined): number {
  if (data?.pending) return PROFILE_POLL_MS
  if (!data?.nextRefreshAt) return 0
  return Math.max(PROFILE_POLL_MS, Date.parse(data.nextRefreshAt) - Date.now())
}

/** The taste profile, re-read while pending and when the cooldown ends. */
export function useTasteProfile() {
  const client = createServiceClient(TasteService)
  return useSWR<GetProfileResponse, Error>(swrKeys.tasteProfile, () => client.getProfile({}), {
    refreshInterval: profileRefreshInterval
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

/** Picks for a domain and track; '' is every domain or both tracks. */
export function useTasteRecommendations(domain: string, track: string) {
  const client = createServiceClient(TasteService)
  return useSWR<ListRecommendationsResponse, Error>(
    swrKeys.tasteRecommendations(domain, track),
    () => client.listRecommendations({ domain, track })
  )
}
