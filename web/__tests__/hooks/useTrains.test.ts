import { renderHook } from '@testing-library/react'

jest.mock('swr', () => ({ __esModule: true, default: jest.fn() }))
const mockClient = {
  getFeedInfo: jest.fn().mockResolvedValue({ feedVersion: '2026-08-31' }),
  searchStations: jest.fn().mockResolvedValue({ stations: [{ stopId: 'SA', name: 'Alpha' }] }),
  searchJourneys: jest.fn().mockResolvedValue({ journeys: [] })
}
jest.mock('@/lib/client', () => ({
  createServiceClient: jest.fn(() => mockClient)
}))
jest.mock('@/lib/gen/trains/v1/trains_pb', () => ({
  TrainService: {}
}))

import useSWR from 'swr'
import { useTrainsFeedInfo, useStationSearch, useJourneySearch } from '@/hooks/useTrains'

const mockUseSWR = jest.mocked(useSWR)

beforeEach(() => {
  // @ts-expect-error -- mock returns partial SWRResponse for test purposes
  mockUseSWR.mockReturnValue({ data: undefined, isLoading: false, error: undefined })
  mockUseSWR.mockClear()
})

describe('useTrainsFeedInfo', () => {
  it('uses the feed-info key', () => {
    renderHook(() => useTrainsFeedInfo())
    expect(mockUseSWR).toHaveBeenCalledWith('/trains/feed-info', expect.any(Function))
  })
})

describe('useStationSearch', () => {
  it('keys by the trimmed query', () => {
    renderHook(() => useStationSearch('  Brussels  '))
    expect(mockUseSWR).toHaveBeenCalledWith(
      ['/trains/stations', 'Brussels'],
      expect.any(Function),
      { keepPreviousData: true }
    )
  })

  it('returns an empty station list when there is no data yet', () => {
    const { result } = renderHook(() => useStationSearch(''))
    expect(result.current.stations).toEqual([])
  })

  it('returns the fetched stations', () => {
    // @ts-expect-error -- mock returns partial SWRResponse for test purposes
    mockUseSWR.mockReturnValue({
      data: { stations: [{ stopId: 'SA', name: 'Alpha' }] },
      isLoading: false
    })
    const { result } = renderHook(() => useStationSearch('a'))
    expect(result.current.stations).toEqual([{ stopId: 'SA', name: 'Alpha' }])
  })
})

describe('useJourneySearch', () => {
  it('passes null as key until both stations are chosen', () => {
    renderHook(() => useJourneySearch('', '', '2026-01-01T00:00:00Z', false))
    expect(mockUseSWR).toHaveBeenCalledWith(null, expect.any(Function))
  })

  it('keys by the search criteria once both stations are chosen', () => {
    renderHook(() => useJourneySearch('SA', 'SB', '2026-01-01T00:00:00Z', true))
    expect(mockUseSWR).toHaveBeenCalledWith(
      ['/trains/journeys', 'SA', 'SB', '2026-01-01T00:00:00Z', true],
      expect.any(Function)
    )
  })
})
