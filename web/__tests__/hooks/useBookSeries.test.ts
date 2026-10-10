import { renderHook } from '@testing-library/react'

jest.mock('swr', () => ({ __esModule: true, default: jest.fn() }))
const mockGetSeries = jest.fn()
jest.mock('@/lib/client', () => ({
  createServiceClient: () => ({ getSeries: mockGetSeries })
}))
jest.mock('@/lib/gen/books/v1/library_pb', () => ({ LibraryService: {} }))

import useSWR from 'swr'
import { useBookSeries } from '@/hooks/useBookSeries'
import { swrKeys } from '@/lib/swrKeys'

const mockUseSWR = jest.mocked(useSWR)

describe('useBookSeries', () => {
  beforeEach(() => mockUseSWR.mockReset())

  it('fetches the named series', async () => {
    renderHook(() => useBookSeries('Discworld'))
    expect(mockUseSWR.mock.calls[0]![0]).toEqual(swrKeys.bookSeries('Discworld'))
    const fetcher = mockUseSWR.mock.calls[0]![1]!
    await fetcher()
    expect(mockGetSeries).toHaveBeenCalledWith({ name: 'Discworld' })
  })

  it('skips the fetch without a name', () => {
    renderHook(() => useBookSeries(''))
    expect(mockUseSWR.mock.calls[0][0]).toBeNull()
  })
})
