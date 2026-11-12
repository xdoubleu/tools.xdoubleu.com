import { renderHook } from '@testing-library/react'

vi.mock('swr', () => ({ __esModule: true, default: vi.fn() }))
vi.mock('@/lib/client', () => ({
  createServiceClient: vi.fn(() => ({ getToday: vi.fn().mockResolvedValue({ shopping: {} }) }))
}))
vi.mock('@/lib/gen/dashboard/v1/today_pb', () => ({ TodayService: {} }))

import useSWR from 'swr'
import { useToday } from '@/hooks/useToday'
import { swrKeys } from '@/lib/swrKeys'

const mockUseSWR = vi.mocked(useSWR)

describe('useToday', () => {
  it('fetches the summary under the today key', async () => {
    // @ts-expect-error -- partial SWR response for the test
    mockUseSWR.mockReturnValue({})
    renderHook(() => useToday())

    expect(mockUseSWR).toHaveBeenCalledWith(swrKeys.today, expect.any(Function))
    const fetcher = mockUseSWR.mock.calls[0][1]
    if (typeof fetcher !== 'function') throw new Error('no fetcher')
    await expect(fetcher(swrKeys.today)).resolves.toEqual({ shopping: {} })
  })
})
