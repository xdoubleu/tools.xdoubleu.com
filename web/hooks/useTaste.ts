import useSWR from 'swr'
import { useCallback } from 'react'
import { swrKeys } from '@/lib/swrKeys'
import { createServiceClient } from '@/lib/client'
import { TasteService } from '@/lib/gen/taste/v1/taste_pb'
import type { GetStatsResponse, ListEventsResponse } from '@/lib/gen/taste/v1/taste_pb'
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
