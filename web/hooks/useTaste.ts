import useSWR, { useSWRConfig } from 'swr'
import { useCallback, useEffect, useMemo, useRef } from 'react'
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
import { SAVED_VIEW } from '@/lib/taste/format'
import { sharedField } from '@/lib/listScope'
import { useSharedList } from '@/hooks/useSharedList'

/** The stats, log, profile and picks of the list the URL selects: solo or the family's. */
export function useTasteStats() {
  const client = createServiceClient(TasteService)
  const shared = useSharedList()
  return useSWR<GetStatsResponse, Error>(swrKeys.tasteStats(shared), () =>
    client.getStats(sharedField(shared))
  )
}

/** The first page of the log; domain '' is every domain. */
export function useTasteEvents(domain: string) {
  const client = createServiceClient(TasteService)
  const shared = useSharedList()
  return useSWR<ListEventsResponse, Error>(swrKeys.tasteEvents(domain, shared), () =>
    client.listEvents({ domain, limit: DEFAULT_PAGE_SIZE, ...sharedField(shared) })
  )
}

export function useFetchTasteEventsPage(domain: string) {
  // createServiceClient caches per service, so client is stable.
  const client = createServiceClient(TasteService)
  const shared = useSharedList()
  return useCallback(
    (offset: number) =>
      client
        .listEvents({ domain, limit: DEFAULT_PAGE_SIZE, offset, ...sharedField(shared) })
        .then((r) => ({ items: r.events, hasMore: r.hasMore })),
    [client, domain, shared]
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
  const shared = useSharedList()
  return useSWR<GetProfileResponse, Error>(
    swrKeys.tasteProfile(shared),
    () => client.getProfile(sharedField(shared)),
    { refreshInterval: profileRefreshInterval }
  )
}

/** Queues a profile regeneration, then re-reads the profile. */
export function useRefreshTasteProfile() {
  const client = createServiceClient(TasteService)
  const { mutate } = useSWRConfig()
  const shared = useSharedList()
  return useCallback(async () => {
    await client.refreshProfile(sharedField(shared))
    await Promise.all([mutate(swrKeys.tasteProfile(shared)), mutate(isRecommendationsKey)])
  }, [client, mutate, shared])
}

/** Matches every recommendations key, whatever its domain and track. */
export function isRecommendationsKey(key: unknown): boolean {
  return Array.isArray(key) && key[0] === swrKeys.tasteRecommendationsPrefix
}

/** Re-reads the picks once a pending profile run finishes; it writes them too. */
export function useRevalidatePicksWhenDone(pending: boolean) {
  const { mutate } = useSWRConfig()
  const wasPending = useRef(pending)
  useEffect(() => {
    if (wasPending.current && !pending) void mutate(isRecommendationsKey)
    wasPending.current = pending
  }, [pending, mutate])
}

/**
 * Picks for a domain and track, or every saved one for SAVED_VIEW; '' is
 * every domain or both tracks.
 */
export function useTasteRecommendations(domain: string, view: string) {
  const client = createServiceClient(TasteService)
  const shared = useSharedList()
  const savedOnly = view === SAVED_VIEW
  return useSWR<ListRecommendationsResponse, Error>(
    swrKeys.tasteRecommendations(domain, view, shared),
    () =>
      client.listRecommendations({
        domain,
        track: savedOnly ? '' : view,
        savedOnly,
        ...sharedField(shared)
      })
  )
}

/** Turns the new-picks email on or off, then re-reads the picks that carry the setting. */
export function useSetEmailNewPicks() {
  const client = createServiceClient(TasteService)
  const { mutate } = useSWRConfig()
  return useCallback(
    async (enabled: boolean) => {
      await client.setEmailNewPicks({ enabled })
      await mutate(isRecommendationsKey)
    },
    [client, mutate]
  )
}

/** Feedback, save and accept on a pick; each re-reads the picks and stats. */
export function usePickActions() {
  const client = createServiceClient(TasteService)
  const { mutate } = useSWRConfig()
  const shared = useSharedList()
  return useMemo(() => {
    const after = async <T>(call: Promise<T>) => {
      await call
      await Promise.all([mutate(isRecommendationsKey), mutate(swrKeys.tasteStats(shared))])
    }
    const scope = sharedField(shared)
    return {
      setFeedback: (id: string, feedback: string) =>
        after(client.setFeedback({ id, feedback, ...scope })),
      setSaved: (id: string, saved: boolean) => after(client.setSaved({ id, saved, ...scope })),
      accept: (id: string) => after(client.acceptRecommendation({ id, ...scope }))
    }
  }, [client, mutate, shared])
}
