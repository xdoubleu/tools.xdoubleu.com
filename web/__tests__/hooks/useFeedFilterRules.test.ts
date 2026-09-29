import { renderHook } from '@testing-library/react'

const mutateMock = jest.fn()
jest.mock('swr', () => ({
  __esModule: true,
  default: jest.fn(),
  mutate: (...args: unknown[]) => mutateMock(...args)
}))

const clientMocks = {
  listFilterRules: jest.fn().mockResolvedValue({ rules: [] }),
  createFilterRule: jest.fn().mockResolvedValue({ rule: { id: 'r1', filteredCount: 0 } }),
  deleteFilterRule: jest.fn().mockResolvedValue({}),
  deleteFeed: jest.fn().mockResolvedValue({})
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
  useFilterRules,
  useCreateFilterRule,
  useDeleteFilterRule,
  useDeleteFeed
} from '@/hooks/useFeeds'
import { swrKeys } from '@/lib/swrKeys'

const mockUseSWR = jest.mocked(useSWR)

describe('filter rule hooks', () => {
  beforeEach(() => {
    jest.clearAllMocks()
    // @ts-expect-error -- partial SWRResponse is fine for these tests
    mockUseSWR.mockReturnValue({ data: undefined })
  })

  it('useFilterRules lists rules under their own key without focus revalidation', async () => {
    renderHook(() => useFilterRules())
    const [key, fetcher, options] = mockUseSWR.mock.calls[0]!
    expect(key).toBe(swrKeys.feedFilterRules)
    expect(options).toMatchObject({ revalidateOnFocus: false })
    await fetcher!()
    expect(clientMocks.listFilterRules).toHaveBeenCalledWith({})
  })

  it('useCreateFilterRule refreshes only rules and suggestions when nothing was filtered', async () => {
    const { result } = renderHook(() => useCreateFilterRule())
    const input = { feedId: '', kind: 2, value: 'sponsored' }
    await result.current(input)
    expect(clientMocks.createFilterRule).toHaveBeenCalledWith(input)
    expect(mutateMock).toHaveBeenCalledWith(swrKeys.feedFilterRules)
    expect(mutateMock).toHaveBeenCalledWith(swrKeys.feedFilterRuleSuggestions)
    expect(mutateMock).toHaveBeenCalledTimes(2)
  })

  it('useCreateFilterRule also refreshes items and stats when items were filtered', async () => {
    clientMocks.createFilterRule.mockResolvedValueOnce({ rule: { id: 'r1', filteredCount: 3 } })
    const { result } = renderHook(() => useCreateFilterRule())
    const resp = await result.current({ feedId: 'f1', kind: 1, value: 'News' })
    expect(resp.rule?.filteredCount).toBe(3)
    expect(mutateMock).toHaveBeenCalledWith(swrKeys.feedFilterRules)
    expect(mutateMock).toHaveBeenCalledWith(swrKeys.feedStats)
    expect(mutateMock).toHaveBeenCalledWith(swrKeys.feedsSummary)
    const itemsMatcher = mutateMock.mock.calls.find(([k]) => typeof k === 'function')![0]
    expect(itemsMatcher(swrKeys.feedItems(true))).toBe(true)
    expect(itemsMatcher(swrKeys.feedFilterRules)).toBe(false)
  })

  it('useCreateFilterRule treats a response without a rule as nothing filtered', async () => {
    clientMocks.createFilterRule.mockResolvedValueOnce({})
    const { result } = renderHook(() => useCreateFilterRule())
    await result.current({ feedId: '', kind: 2, value: 'x' })
    expect(mutateMock).toHaveBeenCalledTimes(2)
  })

  it('useDeleteFilterRule deletes and refreshes rules and suggestions', async () => {
    const { result } = renderHook(() => useDeleteFilterRule())
    await result.current('r1')
    expect(clientMocks.deleteFilterRule).toHaveBeenCalledWith({ ruleId: 'r1' })
    expect(mutateMock).toHaveBeenCalledWith(swrKeys.feedFilterRules)
    expect(mutateMock).toHaveBeenCalledWith(swrKeys.feedFilterRuleSuggestions)
  })

  it('useDeleteFeed refreshes rules, since a feed takes its rules with it', async () => {
    const { result } = renderHook(() => useDeleteFeed())
    await result.current('f1')
    expect(mutateMock).toHaveBeenCalledWith(swrKeys.feedFilterRules)
  })
})
