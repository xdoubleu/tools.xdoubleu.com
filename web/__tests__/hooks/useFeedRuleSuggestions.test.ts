import { renderHook } from '@testing-library/react'

jest.mock('swr', () => ({ __esModule: true, default: jest.fn() }))

const clientMocks = {
  getFilterRuleSuggestions: jest.fn().mockResolvedValue({ suggestions: [] })
}

jest.mock('@/lib/client', () => ({
  createServiceClient: jest.fn(() => clientMocks)
}))
jest.mock('@/lib/offline/outbox', () => ({
  enqueueWrite: jest.fn(async () => ({ status: () => 'sent' })),
  sendWrite: jest.fn(async (pending: Promise<unknown>) => {
    await pending
    return 'sent'
  })
}))
jest.mock('@/lib/feeds/offlineWrites', () => ({ dismissSuggestionWrite: { id: 'dismiss' } }))
jest.mock('@/lib/feeds/prefetch', () => ({ prefetchFeedBodies: jest.fn() }))
jest.mock('@/lib/gen/feeds/v1/feeds_pb', () => ({
  FeedService: {},
  FeedKind: { UNSPECIFIED: 0, RSS: 1, EMAIL: 2 }
}))

import useSWR from 'swr'
import { enqueueWrite, sendWrite } from '@/lib/offline/outbox'
import { dismissSuggestionWrite } from '@/lib/feeds/offlineWrites'
import {
  useDismissFilterRuleSuggestion,
  useFilterRuleSuggestions
} from '@/hooks/useFeedRuleSuggestions'
import { swrKeys } from '@/lib/swrKeys'

const mockUseSWR = jest.mocked(useSWR)

describe('filter rule suggestion hooks', () => {
  beforeEach(() => {
    jest.clearAllMocks()
    // @ts-expect-error -- partial SWRResponse is fine for these tests
    mockUseSWR.mockReturnValue({ data: undefined })
  })

  it('useFilterRuleSuggestions fetches under its own key without focus revalidation', async () => {
    renderHook(() => useFilterRuleSuggestions())
    const [key, fetcher, options] = mockUseSWR.mock.calls[0]!
    expect(key).toBe(swrKeys.feedFilterRuleSuggestions)
    expect(options).toMatchObject({ revalidateOnFocus: false })
    await fetcher!()
    expect(clientMocks.getFilterRuleSuggestions).toHaveBeenCalledWith({})
  })

  it('useDismissFilterRuleSuggestion sends the dismissal through the outbox', async () => {
    const { result } = renderHook(() => useDismissFilterRuleSuggestion())
    await result.current('f1', 'News')
    expect(enqueueWrite).toHaveBeenCalledWith(dismissSuggestionWrite, {
      feedId: 'f1',
      category: 'News'
    })
    expect(sendWrite).toHaveBeenCalled()
  })
})
