import { renderHook } from '@testing-library/react'

jest.mock('swr', () => ({ __esModule: true, default: jest.fn() }))
const mockClient = {
  getStats: jest.fn().mockResolvedValue({}),
  listEvents: jest.fn().mockResolvedValue({ events: [{ itemId: 'b1' }], hasMore: true })
}
jest.mock('@/lib/client', () => ({ createServiceClient: jest.fn(() => mockClient) }))
jest.mock('@/lib/gen/taste/v1/taste_pb', () => ({ TasteService: {} }))

import useSWR from 'swr'
import { useFetchTasteEventsPage, useTasteEvents, useTasteStats } from '@/hooks/useTaste'

const mockUseSWR = jest.mocked(useSWR)

beforeEach(() => {
  jest.clearAllMocks()
  // @ts-expect-error -- mock returns partial SWRResponse for test purposes
  mockUseSWR.mockImplementation((key: unknown, fetcher: unknown) => {
    if (key !== null && typeof fetcher === 'function') void fetcher()
    return { data: undefined, isLoading: false, error: undefined }
  })
})

describe('useTasteStats', () => {
  it('fetches the stats', () => {
    renderHook(() => useTasteStats())
    expect(mockUseSWR).toHaveBeenCalledWith('/taste/stats', expect.any(Function))
    expect(mockClient.getStats).toHaveBeenCalledWith({})
  })
})

describe('useTasteEvents', () => {
  it('fetches the first page of a domain', () => {
    renderHook(() => useTasteEvents('games'))
    expect(mockUseSWR).toHaveBeenCalledWith(['/taste/events', 'games'], expect.any(Function))
    expect(mockClient.listEvents).toHaveBeenCalledWith({ domain: 'games', limit: 50 })
  })
})

describe('useFetchTasteEventsPage', () => {
  it('fetches a later page as items', async () => {
    const { result } = renderHook(() => useFetchTasteEventsPage('books'))
    await expect(result.current(50)).resolves.toEqual({
      items: [{ itemId: 'b1' }],
      hasMore: true
    })
    expect(mockClient.listEvents).toHaveBeenCalledWith({ domain: 'books', limit: 50, offset: 50 })
  })
})
