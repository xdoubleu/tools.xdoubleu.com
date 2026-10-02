import { useCallback } from 'react'
import useSWR from 'swr'
import { swrKeys } from '@/lib/swrKeys'
import { createServiceClient } from '@/lib/client'
import { FeedService } from '@/lib/gen/feeds/v1/feeds_pb'
import type { GetFilterRuleSuggestionsResponse } from '@/lib/gen/feeds/v1/feeds_pb'
import { noAutoRevalidate } from '@/hooks/useFeeds'
import { enqueueWrite, sendWrite } from '@/lib/offline/outbox'
import { dismissSuggestionWrite } from '@/lib/feeds/offlineWrites'

// useFilterRuleSuggestions lists the feed categories the caller rarely reads
// that no rule covers yet.
export function useFilterRuleSuggestions() {
  const client = createServiceClient(FeedService)
  return useSWR<GetFilterRuleSuggestionsResponse, Error>(
    swrKeys.feedFilterRuleSuggestions,
    () => client.getFilterRuleSuggestions({}),
    noAutoRevalidate
  )
}

// useDismissFilterRuleSuggestion stops suggesting a feed's category for good.
export function useDismissFilterRuleSuggestion() {
  return useCallback(async (feedId: string, category: string) => {
    await sendWrite(enqueueWrite(dismissSuggestionWrite, { feedId, category }))
  }, [])
}
