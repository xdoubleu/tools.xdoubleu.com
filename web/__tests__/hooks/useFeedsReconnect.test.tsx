import { act, renderHook, waitFor } from '@testing-library/react'
import { SWRConfig } from 'swr'
import { persistMiddleware } from '@/lib/offline/persist'
import { getSnapshot, markLive } from '@/lib/offline/status'
import { loadEntry } from '@/lib/offline/store'
import {
  useFeedItem,
  useFeedItems,
  useFeedStats,
  useFeeds,
  useFeedsSummary,
  useFilterRules
} from '@/hooks/useFeeds'
import { useFilteredFeedItems } from '@/hooks/useFeedFilteredItems'
import { useFilterRuleSuggestions } from '@/hooks/useFeedRuleSuggestions'

// Real SWR and persist middleware; only the network and IndexedDB are faked.
const client = vi.hoisted(() => ({
  listFeeds: vi.fn(),
  listFeedItems: vi.fn(),
  getFeedItem: vi.fn(),
  listFilterRules: vi.fn(),
  getFeedStats: vi.fn(),
  getFilterRuleSuggestions: vi.fn()
}))
vi.mock('@/lib/client', () => ({ createServiceClient: () => client, transport: {} }))
vi.mock('@/lib/offline/store', () => ({ loadEntry: vi.fn(), saveEntry: vi.fn() }))
vi.mock('@/lib/offline/outbox', () => ({
  applyPending: async (_key: unknown, data: unknown) => data,
  enqueueWrite: vi.fn(),
  sendWrite: vi.fn()
}))
vi.mock('@/lib/feeds/prefetch', () => ({ prefetchFeedBodies: vi.fn() }))

function wrapper({ children }: { children: React.ReactNode }) {
  return (
    <SWRConfig
      value={{
        provider: () => new Map(),
        use: [persistMiddleware],
        shouldRetryOnError: false,
        dedupingInterval: 0
      }}
    >
      {children}
    </SWRConfig>
  )
}

describe('feeds hooks on reconnect', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    markLive()
    vi.mocked(loadEntry).mockResolvedValue({ data: { items: [] }, savedAt: 42 })
  })

  it.each([
    ['useFeeds', () => useFeeds(), client.listFeeds],
    ['useFeedItems', () => useFeedItems(true), client.listFeedItems],
    ['useFeedItem', () => useFeedItem('item-1'), client.getFeedItem],
    ['useFilterRules', () => useFilterRules(), client.listFilterRules],
    ['useFeedStats', () => useFeedStats(), client.getFeedStats],
    ['useFeedsSummary', () => useFeedsSummary(), client.listFeedItems],
    ['useFilteredFeedItems', () => useFilteredFeedItems(), client.listFeedItems],
    ['useFilterRuleSuggestions', () => useFilterRuleSuggestions(), client.getFilterRuleSuggestions]
  ])('%s replaces its saved copy with live data', async (_, hook, fetch) => {
    // useFeedsSummary also reads stats; keep them live so only the list is saved.
    client.getFeedStats.mockResolvedValue({ stats: [] })
    fetch.mockRejectedValueOnce(new TypeError('Failed to fetch')).mockResolvedValue({ items: [] })
    renderHook(hook, { wrapper })
    await waitFor(() => expect(getSnapshot().servedFromCacheAt).toBe(42))

    act(() => void window.dispatchEvent(new Event('online')))

    // The offline banner clears once a live response arrives.
    await waitFor(() => expect(getSnapshot().servedFromCacheAt).toBeNull())
  })
})
