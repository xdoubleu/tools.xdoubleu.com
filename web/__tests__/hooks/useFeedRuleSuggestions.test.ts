import { renderHook } from '@testing-library/react'

const mutateMock = jest.fn()
jest.mock('swr', () => ({
  __esModule: true,
  default: jest.fn(),
  mutate: (...args: unknown[]) => mutateMock(...args)
}))

const clientMocks = {
  getFilterRuleSuggestions: jest.fn().mockResolvedValue({ suggestions: [] }),
  dismissFilterRuleSuggestion: jest.fn().mockResolvedValue({})
}

jest.mock('@/lib/client', () => ({
  createServiceClient: jest.fn(() => clientMocks)
}))
jest.mock('@/lib/gen/feeds/v1/feeds_pb', () => ({
  FeedService: {},
  FeedKind: { UNSPECIFIED: 0, RSS: 1, EMAIL: 2 }
}))

import useSWR from 'swr'
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

  it('useDismissFilterRuleSuggestion dismisses, then refreshes the suggestions', async () => {
    const { result } = renderHook(() => useDismissFilterRuleSuggestion())
    await result.current('f1', 'News')
    expect(clientMocks.dismissFilterRuleSuggestion).toHaveBeenCalledWith({
      feedId: 'f1',
      category: 'News'
    })
    expect(mutateMock).toHaveBeenCalledWith(swrKeys.feedFilterRuleSuggestions)
  })

  it('useDismissFilterRuleSuggestion refreshes nothing when dismissing fails', async () => {
    clientMocks.dismissFilterRuleSuggestion.mockRejectedValueOnce(new Error('boom'))
    const { result } = renderHook(() => useDismissFilterRuleSuggestion())
    await expect(result.current('f1', 'News')).rejects.toThrow('boom')
    expect(mutateMock).not.toHaveBeenCalled()
  })
})
